package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/stmansour/simq/util"
	"github.com/yosuke-furukawa/json5/encoding/json5"
)

// SimulatorCommand to be sent to simulator
type SimulatorCommand struct {
	Command string
}

// SimdConfig is the configuration for the simulator
type SimdConfig struct {
	MachineID          string
	CPUs               int
	Memory             string
	CPUArchitecture    string
	Availability       string
	DispatcherURL      string
	FQDispatcherURL    string
	SimdURL            string
	MaxSimulations     int    // maximum number of simulations this machine can run
	SimdSimulationsDir string // directory where simulations are stored

}

// SimulatorStatus response from simulator
type SimulatorStatus struct {
	ProgramStarted         string
	RunDuration            string
	ConfigFile             string
	SimulationDateRange    string
	LoopCount              int
	GenerationsRequested   int
	CompletedLoops         int
	CompletedGenerations   int
	ElapsedTimeLastGen     string
	EstimatedTimeRemaining string
	EstimatedCompletion    string
}

var app struct {
	cfg         SimdConfig         // configuration of this machine
	listenPort  int                // port to listen on 8251 by default
	sims        []Simulation       // currently running simulations
	simsMu      sync.Mutex         // mutex for updating sims
	HexASCIIDbg bool               // if true print reply buffers in hex and ASCII
	HTTPHdrsDbg bool               // if true print HTTP headers
	version     bool               // program version string
	simdHomeDir string             // home directory - typically /usr/local/simq/simd
	mutex       sync.Mutex         // mutex for creating the tar.gz file
	DtStart     time.Time          // start time of the program
	Paused      bool               // when true, do not book any more simulations
	cancel      context.CancelFunc // used to shutdown smoothly
	ctx         context.Context    // used to shutdown smoothly
}

func readCommandLineArgs() {
	flag.BoolVar(&app.HexASCIIDbg, "D", false, "Turn on debug mode")
	flag.BoolVar(&app.version, "v", false, "Print the program version string")
	flag.BoolVar(&app.Paused, "p", false, "When present, start simd in paused mode (don't book any simulations)")
	flag.Parse()
}

func main() {
	var err error
	app.listenPort = 8251
	app.Paused = false // this is the default, but I want to be very explicit about this
	app.HTTPHdrsDbg = app.HexASCIIDbg

	//-------------------------------------
	// HANDLE COMMAND LINE ARGUMENTS
	//-------------------------------------
	readCommandLineArgs()
	if app.version {
		s := util.Version()
		fmt.Printf("Version: %s\n", s)
		os.Exit(0)
	}

	//-----------------------------------------
	// OUTPUT MESSAGES TO A LOGFILE
	//-----------------------------------------
	app.simdHomeDir, err = util.GetExecutableDir()
	if err != nil {
		log.Fatalf("Failed to get executable directory: %v", err)
	}

	fname := filepath.Join(app.simdHomeDir, "simd.log")
	logFile, err := os.OpenFile(fname, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	log.Printf("----------------------------------------------------------------\n")
	log.Printf("simd version: %s\n", util.Version())
	app.DtStart = time.Now()
	log.Printf("Initiated: %s\n", app.DtStart.Format(time.RFC3339))

	app.sims = make([]Simulation, 0) // initialize it empty

	//-------------------------------------
	// READ CONFIG
	//-------------------------------------
	fname = filepath.Join(app.simdHomeDir, "simdconf.json5")
	if err = loadConfig(fname, &app.cfg); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	//-------------------------------------
	// GET MY IP ADDRESS
	//-------------------------------------
	naddrs, err := util.GetNetworkInfo()
	if err != nil {
		log.Fatalf("Failed to get network info: %v", err)
	}
	for i := 0; i < len(naddrs); i++ {
		if strings.Contains(naddrs[i].IPAddress, "127.0.0.1") {
			continue
		}
		app.cfg.SimdURL = naddrs[i].IPAddress
	}
	log.Printf("\n-----------------------------------------------------------------\n")
	log.Printf("simd version: %s\n", util.Version())
	log.Printf("simd network address: %s\n", app.cfg.SimdURL)

	//-------------------------------------
	// SETUP THE HTTP LISTENER
	//-------------------------------------
	go func() {
		http.HandleFunc("/PauseBooking", PauseBookingHandler)
		http.HandleFunc("/ResumeBooking", ResumeBookingHandler)
		http.HandleFunc("/Shutdown", ShutdownHandler)
		http.HandleFunc("/Status", StatusHandler)
		http.HandleFunc("/CheckUpdates", CheckUpdatesHandler)

		log.Printf("Starting SIMD HTTP listener on port %d\n", app.listenPort)
		if err := http.ListenAndServe(fmt.Sprintf(":%d", app.listenPort), nil); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	//-------------------------------------
	// SETUP THE DISPATCHER URL
	//-------------------------------------
	parsedURL, err := url.Parse(app.cfg.DispatcherURL)
	if err != nil {
		log.Printf("ERROR: failed to parse dispatcher base URL: %v", err)
	}
	app.cfg.DispatcherURL = parsedURL.String()
	parsedURL.Path = path.Join(parsedURL.Path, "command")
	app.cfg.FQDispatcherURL = parsedURL.String()
	log.Printf("FQDispatcherURL: %s\n", app.cfg.FQDispatcherURL)

	//-----------------------------------------------------
	// ENSURE THAT THE SIMULATIONS DIRECTORY EXISTS
	//-----------------------------------------------------
	dirPath := filepath.Join(app.cfg.SimdSimulationsDir, "simulations")
	if err = os.MkdirAll(dirPath, os.ModePerm); err != nil {
		log.Fatalf("Failed to create %s directory: %v", dirPath, err)
	}

	if app.Paused {
		log.Printf("Starting in paused mode. Will not book or recover simulations until pause is removed.\n")
	} else {
		log.Printf("Started in normal mode. Will book simulations as they become available.\n")
	}

	//-------------------------------------
	// SETUP THE SHUTDOWN CHANNEL...
	//-------------------------------------
	app.ctx, app.cancel = context.WithCancel(context.Background())

	// Signal handling for graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		app.cancel()
	}()

	//-----------------------------------------------------
	// SEE IF WE NEED TO RESTORE ANY INTERRUPTED JOBS...
	//-----------------------------------------------------
	err = RebuildSimulatorList()
	if err != nil {
		log.Fatalf("Failed to rebuild simulator list: %v", err)
	}
	log.Printf("Finished RebuildSimulatorList\n")

	//-------------------------------------
	// AFTER REBUILD CHECKS, BOOK AND RUN
	//-------------------------------------
	if isAvailable() {
		err := bookAndRunSimulation("Book", 0)
		if err != nil {
			log.Printf("Failed to book and run simulation: %v", err)
		}
	}

	//---------------------------------------------------------------
	// NOTHING TO DO NOW. PERIODICALLY CHECK FOR WORK AND HANDLE IT
	//---------------------------------------------------------------
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if isAvailable() {
				// fmt.Printf("simd >>>> isAvailable() reports: true\n") // debug
				err := bookAndRunSimulation("Book", 0)
				if err != nil {
					log.Printf("Failed to book and run simulation: %v", err)
				}
			}
		case <-app.ctx.Done():
			log.Println("Shutting down gracefully...")
			// Perform any necessary cleanup here
			return
		}
	}
}

// Load configuration from JSON5 file
// ------------------------------------------------------------------------------
func loadConfig(file string, config *SimdConfig) error {
	content, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	err = json5.Unmarshal(content, config)
	if err != nil {
		return err
	}
	config.MachineID, err = GetMachineUUID()
	if err != nil {
		return err
	}
	return nil
}

// Check if simd is available to run simulations
// ------------------------------------------------------------------------------
func isAvailable() bool {
	//-------------------------------------
	// Can we run any more simulations?
	// trivial implementation for now...
	//-------------------------------------
	return len(app.sims) < app.cfg.MaxSimulations && !app.Paused
}

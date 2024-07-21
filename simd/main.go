package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
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
	cfg         SimdConfig   // configuration of this machine
	sims        []Simulation // currently running simulations
	simsMu      sync.Mutex   // mutex for updating sims
	HexASCIIDbg bool         // if true print reply buffers in hex and ASCII
	HTTPHdrsDbg bool         // if true print HTTP headers
	version     bool         // program version string
	simdHomeDir string       // home directory - typically /usr/local/simq/simd
	mutex       sync.Mutex   // mutex for creating the tar.gz file
}

func readCommandLineArgs() {
	flag.BoolVar(&app.HexASCIIDbg, "D", false, "Turn on debug mode")
	flag.BoolVar(&app.version, "v", false, "Print the program version string")

	fmt.Println("Command-line arguments:", os.Args)

	flag.Parse()
}

func main() {
	var err error

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
	log.Printf("Initiated: %s\n", time.Now().Format(time.RFC3339))

	app.sims = make([]Simulation, 0) // initialize it empty

	//-------------------------------------
	// READ CONFIG
	//-------------------------------------
	if err = loadConfig("simdconf.json5", &app.cfg); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	//-------------------------------------
	// HANDLE COMMAND LINE ARGUMENTS
	//-------------------------------------
	readCommandLineArgs()
	app.HTTPHdrsDbg = app.HexASCIIDbg
	if app.version {
		fmt.Println("simd version:", util.Version())
		os.Exit(0)
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
	for range ticker.C {
		if isAvailable() {
			// fmt.Printf("simd >>>> isAvailable() reports: true\n") // debug
			err := bookAndRunSimulation("Book", 0)
			if err != nil {
				log.Printf("Failed to book and run simulation: %v", err)
			}
		}
	}
}

// Load configuration from JSON5 file
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
func isAvailable() bool {
	//-------------------------------------
	// Can we run any more simulations?
	// trivial implementation for now...
	//-------------------------------------
	return len(app.sims) < app.cfg.MaxSimulations
}

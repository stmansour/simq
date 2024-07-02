package main

import (
	"log"
	"os"
	"strings"
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
	MachineID       string
	CPUs            int
	Memory          string
	CPUArchitecture string
	Availability    string
	DispatcherURL   string
	SimdURL         string
	MaxSimulations  int // maximum number of simulations this machine can run
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
	HexASCIIDbg bool         // if true print reply buffers in hex and ASCII
	HTTPHdrsDbg bool         // if true print HTTP headers
}

func main() {
	// Load configuration
	err := loadConfig("simdconf.json5", &app.cfg)
	if err != nil {
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
	log.Printf("SimdIP Address: %s\n", app.cfg.SimdURL)

	//-------------------------------------
	// Initial check and run is immediate
	//-------------------------------------
	if isAvailable() {
		err := bookAndRunSimulation()
		if err != nil {
			log.Printf("Failed to book and run simulation: %v", err)
		}
	}

	// Create a ticker that triggers every minute
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		// Check if available to run simulations
		if isAvailable() {
			err := bookAndRunSimulation()
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

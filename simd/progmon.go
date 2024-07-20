package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// Simulation defines a running simulation managed by simd
type Simulation struct {
	Cmd            *exec.Cmd
	SID            int64
	Directory      string
	MachineID      string
	SimPort        int
	BaseURL        string
	FQSimStatusURL string
	ConfigFile     string
	LastStatus     SimulatorStatus
}

// Start the simulator with given SID and config file.
// Inputs:
//
//	sid - the simulation ID
//	FQConfigFileName - the fully qualified name of the config file
//
// -----------------------------------------------------------------------------
func startSimulator(sid int64, FQConfigFileName string) error {
	log.Printf("startSimulator: %d\n", sid)
	//-------------------------------------------------------------
	// Start the simulator
	// Simulator needs to run in ./simulator/<sid>/
	//-------------------------------------------------------------
	Directory := filepath.Join(app.cfg.SimdSimulationsDir, "simulations", fmt.Sprintf("%d", sid))
	logFile := filepath.Join(Directory, "sim.log")
	cmd := exec.Command("/usr/local/plato/bin/simwrapper",
		"-c", FQConfigFileName,
		"-SID", fmt.Sprintf("%d", sid),
		"-DISPATCHER", app.cfg.DispatcherURL) // note: we pass the base url to simulator, not the fully qualified url

	//----------------------------------------------
	// Set the working directory for the command
	//----------------------------------------------
	cmd.Dir = Directory

	//----------------------------------------------
	// Redirect stdout and stderr to the log file
	//----------------------------------------------
	outputFile, err := os.Create(logFile)
	if err != nil {
		return fmt.Errorf("startSimulator: SID=%d, failed to create log file: %v", sid, err)
	}
	cmd.Stdout = outputFile
	cmd.Stderr = outputFile

	//----------------------------------------------
	// Detach the process. We don't want it to stop
	// if this process exits or dies
	//----------------------------------------------
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	//----------------------------------------------
	// Start the process
	//----------------------------------------------
	if err := cmd.Start(); err != nil {
		outputFile.Close()
		return fmt.Errorf("startSimulator: SID=%d, failed to start simulator: %v", sid, err)
	}

	log.Printf("startSimulator: SID=%d, simulator started\n", sid)

	//----------------------------------------------
	// Detach the process. We don't want it to stop
	// if this process exits or dies
	//----------------------------------------------
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("startSimulator: SID=%d, failed to detach simulator process: %v", sid, err)
	}

	//----------------------------------------------------
	// Creating process no longer needs this file handle
	//----------------------------------------------------
	outputFile.Close()

	log.Printf("startSimulator: SID=%d, simulator process released\n", sid)

	//---------------------------------------------------------------
	// we have a new simulation in process. Add it to the list...
	//---------------------------------------------------------------
	sm := Simulation{
		SID:       sid,
		Directory: Directory,
		Cmd:       cmd,
	}
	app.simsMu.Lock() // Lock the mutex before modifying app.sims
	app.sims = append(app.sims, sm)
	app.simsMu.Unlock() // Unlock the mutex after modification

	//---------------------------------------------------------------
	// Monitor the simulator process
	//---------------------------------------------------------------
	go monitorSimulator(&sm)

	return nil
}

// Monitor the simulator process
func monitorSimulator(sim *Simulation) {
	log.Printf("monitorSimulator: monitoring simulator for SID %d @ %s\n", sim.SID, sim.BaseURL)

	//-----------------------------------------------------------------
	// In some cases, the simulator may already be running. But
	// the first time it is booked, the simulator will need to
	// be started. So, give it a few seconds to start. If it's already
	// running then 3 seconds from now is not going to hurt anything.
	//-----------------------------------------------------------------
	time.Sleep(3 * time.Second)
	maxRetries := 3
	if len(sim.FQSimStatusURL) == 0 {
		for retryCount := 0; retryCount < maxRetries; retryCount++ {
			// fmt.Printf("DEBUG: retryCount = %d\n", retryCount)
			if !sim.FindRunningSimulator() {
				time.Sleep(3 * time.Second) // we'll wait for 1 minute, up to 5 times
				continue                    // try again
			}
			break
		}
		if len(sim.FQSimStatusURL) == 0 {
			//------------------------------------------------------------------
			// IT IS POSSIBLE THAT WE HAD A VERY FAST SIMULATION...
			// CHECK TO SEE IF THE SIMULATION RESULT FILES ARE PRESENT...
			//------------------------------------------------------------------
			filenames, err := getFilenamesInDir(sim.Directory)
			if err != nil {
				//-----------------------------------------------------------
				// Exhausted retries.  No files to be found.  This computer
				// seems to be having difficulties.  Tell dispatcher to put
				// it in the Error state.
				//-----------------------------------------------------------
				if err = ErrorEndThisSimulation(sim); err != nil {
					log.Printf("monitorSimulator: failed ErrorEndThisSimulation for SID %d, err = %s\n", sim.SID, err.Error())
					return
				}
				return
			}
			//---------------------------------------------
			// Check for finrep.csv...
			//---------------------------------------------
			found := false
			foundResultsTar := false
			for i := 0; i < len(filenames); i++ {
				if filenames[i] == "finrep.csv" {
					found = true
				}
				if filenames[i] == "results.tar.gz" {
					foundResultsTar = true
				}
			}
			if !found {
				//--------------------------------------------------------
				// We've exhausted the retries and no files can be found
				//--------------------------------------------------------
				if err = ErrorEndThisSimulation(sim); err != nil {
					log.Printf("monitorSimulator: failed ErrorEndThisSimulation for SID %d, err = %s\n", sim.SID, err.Error())
				}
				return
			}

			//----------------------------------------------------------------------
			// We found all the files.  We can do a normal end for this simulation
			//----------------------------------------------------------------------
			if !foundResultsTar {
				if err = sim.archiveSimulationResults(); err != nil {
					log.Printf("monitorSimulator: failed archiveSimulationResults for SID %d, err = %s\n", sim.SID, err.Error())
					log.Printf("monitorSimulator: removing simulation with SID %d\n", sim.SID)
					if err = ErrorEndThisSimulation(sim); err != nil {
						log.Printf("monitorSimulator: failed ErrorEndThisSimulation for SID %d, err = %s\n", sim.SID, err.Error())
					}
					return
				}
			}
			if err = sim.sendEndSimulationRequest(); err != nil {
				log.Printf("monitorSimulator: failed sendEndSimulationRequest for SID %d, err = %s\n", sim.SID, err.Error())
				log.Printf("monitorSimulator: removing simulation with SID %d\n", sim.SID)
				if err = ErrorEndThisSimulation(sim); err != nil {
					log.Printf("monitorSimulator: failed ErrorEndThisSimulation for SID %d, err = %s\n", sim.SID, err.Error())
				}
				return
			}
			log.Printf("monitorSimulator: successfully ended SID %d\n", sim.SID)
			return
		}
		log.Printf("monitorSimulator: found running simulator for SID %d @ %s\n", sim.SID, sim.BaseURL)
	} else {
		log.Printf("monitorSimulator: simulator @ %s\n", sim.BaseURL)
	}

	//-------------------------------------------------------------
	// Create a ticker that triggers every 5 minutes
	//-------------------------------------------------------------
	//ticker := time.NewTicker(5 * time.Minute)
	ticker := time.NewTicker(30 * time.Second) // DEBUG: 30 seconds  (for testing. Make it longer later)
	defer ticker.Stop()

	//-------------------------------------------------------------
	// Periodically ping the simulator to check its status
	//-------------------------------------------------------------
	for range ticker.C {
		// log.Printf("simd >>>> ticker loop >>>> Simulator @ %s is still running\n", sim.BaseURL)
		if !sim.isSimulatorRunning() {
			log.Printf("SID: %d, simulator @ %s is no longer running", sim.SID, sim.BaseURL)
			break
		}
	}
	//-------------------------------------------------------------
	// Simulator has finished. Verify status with dispatcher. If
	// all is well, then transmit files to the dispatcher
	//-------------------------------------------------------------
	log.Printf("Simulator @ %s is no longer running.\n", sim.BaseURL)
	if err := sim.archiveSimulationResults(); err != nil {
		log.Printf("monitorSimulator: SID: %d, failed to archive simulation results: %v", sim.SID, err)
		return
	}
	log.Printf("monitorSimulator: SID: %d, Archived simulation results to %s\n", sim.SID, sim.Directory)

	//-----------------------------------------
	// Send the results to the dispatcher
	//-----------------------------------------
	if err := sim.sendEndSimulationRequest(); err != nil {
		log.Printf("monitorSimulator: SID: %d, failed to send EndSimulation request: %v", sim.SID, err)
		return
	}
}

// Check if the simulator process is still running
func (sim *Simulation) isSimulatorRunning() bool {
	resp, err := http.Get(sim.FQSimStatusURL)
	if err != nil {
		log.Printf("isSimulatorRunning: SID=%d failed to get simulator status: %v", sim.SID, err)
		return false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("isSimulatorRunning: SID=%d error reading response body: %v", sim.SID, err)
		return false
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("isSimulatorRunning: SID=%d server returned error status: %s", sim.SID, resp.Status)
	}
	var status SimulatorStatus
	err = json.Unmarshal(body, &status)
	if err != nil {
		log.Printf("isSimulatorRunning: SID=%d error unmarshaling response body: %v", sim.SID, err)
		return false
	}
	return true
}

// archiveSimulationResults adds all the files we care in the simulation directory
// to a tar.gz file
// -----------------------------------------------------------------------------------
func (sim *Simulation) archiveSimulationResults() error {
	log.Printf("archiveSimulationResults: SID = %d, directory = %s\n", sim.SID, sim.Directory)

	//------------------------------------
	// Save the current working directory
	//------------------------------------
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("archiveSimulationResults: SID=%d, failed to get current directory: %w", sim.SID, err)
	}

	//------------------------------------
	// Change to the simulation directory
	//------------------------------------
	err = os.Chdir(sim.Directory)
	if err != nil {
		return fmt.Errorf("archiveSimulationResults: SID=%d, failed to change to simulation directory: %w", sim.SID, err)
	}

	//------------------------------------------------------------------
	// Ensure we change back to the original directory when we're done
	//------------------------------------------------------------------
	defer os.Chdir(originalDir)

	//---------------------------------------------------------------------------------
	// Create the output file in the current directory (which is now sim.Directory)
	//---------------------------------------------------------------------------------
	outFile, err := os.Create("results.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create archive file: %w", err)
	}
	defer outFile.Close()

	gzWriter := gzip.NewWriter(outFile)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	//---------------------------------------------------------------
	// Search for the files that matter and add them to the archive
	//---------------------------------------------------------------
	patterns := []string{"*.json5", "*.csv", "*.log"} // Define file patterns to archive
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("archiveSimulationResults: SID=%d, failed to find files matching pattern %s: %w", sim.SID, pattern, err)
		}

		for _, filePath := range matches {
			err = addFileToTar(tarWriter, filePath)
			if err != nil {
				return fmt.Errorf("archiveSimulationResults: SID=%d, failed to add file %s to archive: %w", sim.SID, filePath, err)
			}
		}
	}

	return nil
}

// addFileToTar adds a file to a tar.gz archive
// ----------------------------------------------------------------------------
func addFileToTar(tarWriter *tar.Writer, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return err
	}
	header.Name = filePath // This will be the name without any directory prefix

	err = tarWriter.WriteHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(tarWriter, file)
	return err
}

// ErrorEndThisSimulation is called when this computer has exhausted all recovery
// methods but cannot get a simulation to work.
// ----------------------------------------------------------------------------
func ErrorEndThisSimulation(sim *Simulation) error {
	// TODO
	log.Printf("*** Please implement error state\n")

	//--------------------------------------
	// REMOVE THIS SIMULATION FROM THE LIST
	//--------------------------------------
	RemoveSimFromList(sim)
	log.Printf("SetToErrorState:  SID %d has been removed from app.sims\n", sim.SID)
	return nil
}

package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Simulation defines a running simulation managed by simd
type Simulation struct {
	Cmd        *exec.Cmd
	SID        int64
	Directory  string
	MachineID  string
	URL        string
	ConfigFile string
	LastStatus SimulatorStatus
}

// Start the simulator with given SID and config file.
// Inputs:
//
//	sid - the simulation ID
//	FQConfigFileName - the fully qualified name of the config file
//
// -----------------------------------------------------------------------------
func startSimulator(sid int64, FQConfigFileName string) error {
	log.Printf("Starting simulation %d\n", sid)
	//-------------------------------------------------------------
	// Start the simulator
	// Simulator needs to run in ./simulator/<sid>/
	//-------------------------------------------------------------
	Directory := filepath.Join(app.cfg.SimdSimulationsDir, "simulations", fmt.Sprintf("%d", sid))
	logFile := filepath.Join(Directory, "sim.log")
	cmd := exec.Command("/usr/local/plato/bin/simulator",
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
		return fmt.Errorf("failed to create log file: %v", err)
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
		return fmt.Errorf("failed to start simulator: %v", err)
	}

	log.Printf("startSimulator: simulator started\n")

	//----------------------------------------------
	// Detach the process. We don't want it to stop
	// if this process exits or dies
	//----------------------------------------------
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("failed to detach simulator process: %v", err)
	}
	outputFile.Close()

	log.Printf("startSimulator: simulator process released\n")

	//---------------------------------------------------------------
	// we have a new simulation in process. Add it to the list...
	//---------------------------------------------------------------
	sm := Simulation{
		SID:       sid,
		Directory: Directory,
		Cmd:       cmd,
	}
	app.sims = append(app.sims, sm)

	//---------------------------------------------------------------
	// Monitor the simulator process
	//---------------------------------------------------------------
	go monitorSimulator(&sm)

	return nil
}

// Monitor the simulator process
func monitorSimulator(sim *Simulation) {
	log.Printf("simd >>>>  %d\n", sim.SID)
	//-------------------------------------------------------------
	// First thing to do is get the first line of its log file.
	// But let's wait 3 seconds to give it time to startup
	//-------------------------------------------------------------
	time.Sleep(3 * time.Second)
	firstLine, err := getFirstLineOfLog(sim.Directory)
	if err != nil {
		log.Printf("Failed to get first line of log file: %v", err)
		return
	}
	//-------------------------------------------------------------------------------
	// the first line looks like this: "Simtalk address: http://192.168.1.180:8090"
	// We need to extract the URL from that string
	//-------------------------------------------------------------------------------
	startIndex := strings.Index(firstLine, "http://")
	if startIndex == -1 {
		log.Println("No HTTP address found")
		return
	}
	sim.URL = firstLine[startIndex:]

	log.Printf("simd >>>> Simulator @ %s\n", sim.URL)

	//-------------------------------------------------------------
	// Create a ticker that triggers every 5 minutes
	//-------------------------------------------------------------
	//ticker := time.NewTicker(5 * time.Minute)
	ticker := time.NewTicker(1 * time.Minute) // delete this after debugging
	defer ticker.Stop()

	log.Printf("simd >>>> ticker loop >>>> timer set for 1 minute intervals\n")

	//-------------------------------------------------------------
	// Periodically ping the simulator to check its status
	//-------------------------------------------------------------
	for range ticker.C {
		// log.Printf("simd >>>> ticker loop >>>> Simulator @ %s is still running\n", sim.URL)
		if !sim.isSimulatorRunning() {
			log.Printf("Simulator @ %s is no longer running", sim.URL)
			break
		}
	}
	//-------------------------------------------------------------
	// Simulator has finished. Verify status with dispatcher. If
	// all is well, then transmit files to the dispatcher
	//-------------------------------------------------------------
	log.Printf("Simulator @ %s is no longer running.\n", sim.URL)
	if err = sim.archiveSimulationResults(); err != nil {
		log.Printf("Failed to archive simulation results: %v", err)
		return
	}
	log.Printf("Archived simulation results to %s\n", sim.Directory)

	//-----------------------------------------
	// Send the results to the dispatcher
	//-----------------------------------------
	if err = sim.sendEndSimulationRequest(); err != nil {
		log.Printf("Failed to send end simulation request: %v", err)
		return
	}
}

func getFirstLineOfLog(Directory string) (string, error) {
	filePath := filepath.Join(Directory, "sim.log")

	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		return scanner.Text(), nil
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading log file: %w", err)
	}

	return "", fmt.Errorf("log file is empty")
}

// Check if the simulator process is still running
func (sim *Simulation) isSimulatorRunning() bool {
	url := fmt.Sprintf("%s/status", sim.URL)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("failed to get simulator status: %v", err)
		return false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("error reading response body: %v", err)
		return false
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("server returned error status: %s", resp.Status)
	}
	var status SimulatorStatus
	err = json.Unmarshal(body, &status)
	if err != nil {
		log.Printf("error unmarshaling response body: %v", err)
		return false
	}
	return true
}

// archiveSimulationResults adds all the files we care in the simulation directory
// to a tar.gz file
// -----------------------------------------------------------------------------------
func (sim *Simulation) archiveSimulationResults() error {
	//------------------------------------
	// Save the current working directory
	//------------------------------------
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	//------------------------------------
	// Change to the simulation directory
	//------------------------------------
	err = os.Chdir(sim.Directory)
	if err != nil {
		return fmt.Errorf("failed to change to simulation directory: %w", err)
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
			return fmt.Errorf("failed to find files matching pattern %s: %w", pattern, err)
		}

		for _, filePath := range matches {
			err = addFileToTar(tarWriter, filePath)
			if err != nil {
				return fmt.Errorf("failed to add file %s to archive: %w", filePath, err)
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

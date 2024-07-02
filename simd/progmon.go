package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Simulation defines a running simulation managed by simd
type Simulation struct {
	Cmd        *exec.Cmd
	SID        int64
	MachineID  string
	URL        string
	ConfigFile string
	PID        int
	LastStatus SimulatorStatus
	stdout     io.ReadCloser
}

func createFQCWD() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Resolve "." in the path if needed
	if cwd == "." {
		cwd, err = filepath.Abs(".") // Resolve "." to the absolute path of the cwd
		if err != nil {
			return "", err
		}
	}
	return cwd, nil
}
func createFQDirName(cwd string, dirLevels []string) string {
	fullPath := filepath.Join(cwd, filepath.Join(dirLevels...))
	return fullPath
}

func createFQFilename(cwd string, fname string) string {
	fullPath := filepath.Join(cwd, fname)
	return fullPath
}

// Start the simulator with given SID and config file
func startSimulator(sid int64, configFile string) error {
	//-------------------------------------------------------------
	// Start the simulator
	// Simulator needs to run in ./simulator/<sid>/
	//-------------------------------------------------------------
	cwd, err := createFQCWD()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %v", err)
	}
	simDir := createFQDirName(cwd, []string{"simulations", fmt.Sprintf("%d", sid)})
	cf := createFQFilename(simDir, configFile)

	cmd := exec.Command("/usr/local/plato/bin/simulator", "-c", cf, "-SID", fmt.Sprintf("%d", sid), "-DISPATCHER", app.cfg.DispatcherURL)
	cmd.Dir = simDir // Set the working directory for the command

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start simulator: %v", err)
	}

	// we have a new simulation in process. Add it to the list...
	sm := Simulation{
		SID:    sid,
		PID:    cmd.Process.Pid,
		Cmd:    cmd,
		stdout: stdout,
	}
	app.sims = append(app.sims, sm)

	go monitorSimulator(&sm)
	return nil
}

// Monitor the simulator process
func monitorSimulator(sim *Simulation) {
	triggers := []string{
		"ERROR",
		"WARNING",
		"PANIC",
		"INVALID",
		"FORMATER",
	}
	// Read the stdout for port information and other status
	scanner := bufio.NewScanner(sim.stdout)
	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			log.Printf("Simulator output: %s", line)

			for _, trigger := range triggers {
				if strings.Contains(line, trigger) {
					log.Printf("")
					return
				}
			}

			// look for network address
			if strings.Contains(line, "net address:") {
				// parse out the network address
				sa := strings.Split(line, " ")
				if len(sa) >= 3 && !strings.Contains(sa[2], "127.0.0.1") {
					sim.URL = sa[2]
				}
			}
		}
		//
	}()

	// Create a ticker that triggers every 5 minutes
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	// Periodically ping the simulator to check its status
	for range ticker.C {
		if !sim.isSimulatorRunning() {
			log.Printf("Simulator with PID %d is no longer running", sim.PID)
			reportFailure()
			return
		}
		status, err := getSimulatorStatus()
		if err != nil {
			log.Printf("Failed to get simulator status: %v", err)
			reportFailure()
			return
		}
		log.Printf("Simulator status: %+v", status)
	}
}

// Check if the simulator process is still running
func (sim *Simulation) isSimulatorRunning() bool {
	resp, err := http.Get(fmt.Sprintf("%s/status", sim.URL))
	if err != nil {
		log.Printf("failed to get simulator status: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("error reading response body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("server returned error status: %s", resp.Status)
	}
	var status SimulatorStatus
	err = json.Unmarshal(body, &status)
	if err != nil {
		log.Printf("error unmarshaling response body: %v", err)
	}
	fmt.Printf("Simulator estimates completion: %s\n", status.EstimatedCompletion)
	return true

}

// Get the status of the simulator
func getSimulatorStatus() (*SimulatorStatus, error) {
	resp, err := http.Get("http://localhost:port/status")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var status SimulatorStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}
	return &status, nil
}

// Report failure to the dispatcher
func reportFailure() {
	// Implement reporting failure to dispatcher
}

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stmansour/simq/data"
)

// Redo Simulation represents the data for redoing a simulation.
// This command is initiated when a simd contacts the dispatcher to redo a simulation.
//
// Step 1. Pull the config file from /genome/simres
//         This means we need a search for SID.  Copy the config file to qdconfigs,
//         then remove the SID directory from /genome/simres
// Step 2. Re-create the directory for this SID and copy the config file to it
// Step 3. Respond back to the simd process with the config file
// Step 4. Update the database to put this Simulation in the "Booked" state
//

// handleRedo handles the Book command
// ---------------------------------------------------------------------------
func handleRedo(w http.ResponseWriter, r *http.Request, d *HInfo) {
	var queueItem data.QueueItem
	var err error
	//---------------------------------------------------
	// Decode the Redo
	//     Cmd: "Redo"
	//     Username: "whoever"
	//     Data:  {"SID": 1234}
	//     MachineID: "A7B8C9"
	//---------------------------------------------------
	var rebookRequest SimulationRebookRequest
	log.Printf("handling REDO command\n")
	if err := json.Unmarshal(d.cmd.Data, &rebookRequest); err != nil {
		SvcErrorReturn(w, fmt.Errorf("handleRedo: invalid rebook request data"))
		return
	}
	log.Printf("handleRedo: SID %d, MachineID %s\n", rebookRequest.SID, rebookRequest.MachineID)
	queueItem, err = app.qm.GetItemByID(rebookRequest.SID)
	if err != nil {
		SvcErrorReturn(w, fmt.Errorf("handleRedo: could not getItemByID SID %d:  %s", rebookRequest.SID, err.Error()))
		return
	}
	if queueItem.MachineID != rebookRequest.MachineID {
		log.Printf("*** WARNING *** Granted MachineID %s rebooking for SID %d originally assigned to MachineID %s", rebookRequest.MachineID, rebookRequest.SID, queueItem.MachineID)
	}

	//-----------------------------------------------------------------------------
	// THE CONFIG FOR THIS JOB IS IN /GENOME/SIMRES.  We need to find it first.
	//-----------------------------------------------------------------------------
	log.Printf("handleRedo: processing %s command\n", d.cmd.Command)
	simresDir, err := findSimulationDirectory(queueItem.SID)
	if err != nil {
		SvcErrorReturn(w, fmt.Errorf("handleRedo: error finding simulation results directory for SID %d: %s", queueItem.SID, err.Error()))
	}
	configDir := filepath.Join(app.QdConfigsDir, fmt.Sprintf("%d", queueItem.SID))
	if err != nil {
		SvcErrorReturn(w, fmt.Errorf("handleRedo: error finding config file: %s", err.Error()))
		return
	}

	//-----------------------------------------------------------------------------
	// Determine the config filename
	//-----------------------------------------------------------------------------
	configFilename, err := findConfigFile(simresDir)
	if err != nil {
		SvcErrorReturn(w, fmt.Errorf("handleRedo: error finding config file: %s", err.Error()))
		return
	}

	//-----------------------------------------------------------------------------
	// Copy the config file to qdconfigs
	//-----------------------------------------------------------------------------
	if err := copyFile(configFilename, filepath.Join(configDir, filepath.Base(configFilename))); err != nil {
		SvcErrorReturn(w, fmt.Errorf("handleRedo: error copying config file: %s", err.Error()))
		return
	}

	//-----------------------------------------------------------------------------
	// Mark the queue entry for this simulation as "Booked"...
	//-----------------------------------------------------------------------------
	queueItem.State = data.StateQueued
	queueItem.MachineID = ""
	queueItem.DtCompleted.Valid = false
	queueItem.DtCompleted.Time = time.Time{}
	queueItem.DtEstimate.Valid = false
	queueItem.DtEstimate.Time = time.Time{}
	if err := app.qm.UpdateItem(queueItem); err != nil {
		SvcErrorReturn(w, fmt.Errorf("handleRedo: failed to update queue item"))
		return
	}

	//-----------------------------------------------------------------------------
	// Now remove the simulation results directory
	//-----------------------------------------------------------------------------
	if err := os.RemoveAll(simresDir); err != nil {
		SvcErrorReturn(w, fmt.Errorf("handleRedo: failed to remove simulation results directory: %s", err.Error()))
		return
	}

	//-----------------------------------------------------------------------------
	// Send the simple response
	//-----------------------------------------------------------------------------
	w.WriteHeader(http.StatusOK)
	msg := SvcStatus201{
		Status:  "success",
		Message: "Re-queued",
		ID:      queueItem.SID,
	}
	SvcWriteResponse(w, &msg)

	log.Printf("*** handleRedo:  SUCCESSFUL ***\n")
}

// findSimulationDirectory finds the SID in the simulation results repo
// -----------------------------------------------------------------------------
func findSimulationDirectory(sid int64) (string, error) {
	baseDir := app.SimResultsDir
	sidStr := fmt.Sprintf("%d", sid)
	var resultPath string

	baseDirComponents := len(strings.Split(baseDir, string(os.PathSeparator))) // Count the number of components in baseDir
	expectedDepth := baseDirComponents + 4                                     // Add 4 for YYYY/MM/DD/SID
	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			components := strings.Split(path, string(os.PathSeparator)) // Split the path into components

			//-----------------------------------------------------------------------------
			// Check if we're at the correct depth and the last component matches the SID
			//-----------------------------------------------------------------------------
			if len(components) == expectedDepth && components[len(components)-1] == sidStr {
				resultPath = path
				return filepath.SkipAll // terminate the filepath.Walk
			}
		}
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("error walking the path %s: %v", baseDir, err)
	}
	if resultPath == "" {
		return "", fmt.Errorf("simulation directory for SID %d not found", sid)
	}
	return resultPath, nil
}

func copyFile(src, dst string) error {
	// Ensure the destination directory exists
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Open the source file
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	// Create the destination file
	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	// Copy the contents
	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	// Ensure all data is written to disk
	err = destFile.Sync()
	if err != nil {
		return fmt.Errorf("failed to sync destination file: %w", err)
	}

	return nil
}

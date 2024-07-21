package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/stmansour/simq/data"
)

// SvcStatus200 is a simple status message return
type SvcStatus200 struct {
	Status  string
	Message string
}

// SvcStatus201 is a simple status message for use when a new resource is created
type SvcStatus201 struct {
	Status  string
	Message string
	ID      int64
}

// SvcWrite is a general write routine for service calls... it is a bottleneck
// where we can place debug statements as needed.
func SvcWrite(w http.ResponseWriter, b []byte) {
	w.Write(b)
}

// SvcErrorReturn formats an error return to the grid widget and sends it
func SvcErrorReturn(w http.ResponseWriter, err error) {
	log.Printf("%v\n", err)
	var e SvcStatus200
	e.Status = "error"
	e.Message = err.Error()
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.Marshal(e)
	SvcWrite(w, b)
}

// SvcWriteResponse finishes the transaction with the W2UI client
func SvcWriteResponse(w http.ResponseWriter, g interface{}) {
	w.Header().Set("Content-Type", "application/json") // we're marshaling the data as json
	b, err := json.Marshal(g)
	if err != nil {
		SvcErrorReturn(w, fmt.Errorf("error marshaling json data: %s", err.Error()))
		return
	}
	SvcWrite(w, b)
}

func findConfigFile(configDir string) (string, error) {
	files, err := os.ReadDir(configDir)
	if err != nil {
		return "", err
	}

	for _, file := range files {
		if file.Type().IsRegular() && filepath.Ext(file.Name()) == ".json5" {
			return filepath.Join(configDir, file.Name()), nil
		}
	}

	return "", fmt.Errorf("no config file found in the directory")
}

func threadSafeFileIOEndSim(dirPath, filename string, r *http.Request) error {
	//--------------------------------------------------
	// CREATE THE FULL PATH
	//--------------------------------------------------
	app.mutex.Lock() // Lock the mutex before modifying app.sims
	defer app.mutex.Unlock()

	if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
		app.mutex.Unlock() // Unlock the mutex after modification
		return fmt.Errorf("handleEndSimulation: error from os.MkdirAll(%s): %s", dirPath, err.Error())
	}

	//----------------------------------------------------
	// EXTRACT THE FILE CONTENT
	//----------------------------------------------------
	file, _, err := r.FormFile("file")
	if err != nil {
		return fmt.Errorf("handleEndSimulation: failed to get file from form")
	}
	defer file.Close()
	fileContent, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("handleEndSimulation: failed to read file content")
	}

	//----------------------------------------------
	// CREATE THE TAR.GZ FILE
	//----------------------------------------------
	tarz, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("handleEndSimulation: failed to create file %s: %v", filename, err)
	}
	defer tarz.Close()
	if len(fileContent) == 0 {
		return fmt.Errorf("handleEndSimulation: no file content. 0-length file")
	}

	//----------------------------------------------
	// WRITE THE TAR.GZ FILE
	//----------------------------------------------
	if _, err := tarz.Write(fileContent); err != nil {
		return fmt.Errorf("handleEndSimulation: failed to write to file %s: %v", filename, err)
	}

	//----------------------------------------------
	// EXTRACT THE FILES FROM THE TAR.GZ FILE
	//----------------------------------------------
	originalDir, err := os.Getwd() // Save the current directory
	if err != nil {
		return fmt.Errorf("handleEndSimulation: failed to get current directory: %v", err)
	}
	err = os.Chdir(dirPath) // Change to the target directory
	if err != nil {
		return fmt.Errorf("handleEndSimulation: failed to change directory to %s: %v", dirPath, err)
	}

	//----------------------------------------------------------------------
	// tar has actually failed here.  We'll implement retry logic...
	//----------------------------------------------------------------------
	const maxRetries = 3
	const retryDelay = 2 * time.Second
	for i := 0; i < maxRetries; i++ {
		err = executeTarCommand()
		if err == nil {
			break
		}
		log.Printf("handleEndSimulation: failed to execute tar command (attempt %d/%d): %v", i+1, maxRetries, err)
		time.Sleep(retryDelay)
	}
	if err != nil {
		return fmt.Errorf("handleEndSimulation: failed to execute tar command after %d attempts: %v", maxRetries, err)
	}

	err = os.Chdir(originalDir) // Return back to the original directory
	if err != nil {
		return fmt.Errorf("handleEndSimulation: failed to change back to the original directory: %v", err)
	}

	//---------------------------------------------------------------------------------------
	// After we have extracted the data files, we no longer need the tar.gz file. Delete it.
	//---------------------------------------------------------------------------------------
	if err := os.Remove(filename); err != nil {
		return fmt.Errorf("handleEndSimulation: failed to remove config file")
	}

	return nil
}

func executeTarCommand() error {
	tcmd := exec.Command("tar", "xzf", "results.tar.gz")
	tcmd.Stdout = os.Stdout
	tcmd.Stderr = os.Stderr
	return tcmd.Run()
}

func threadSafeRemoveAll(configDir string) error {
	app.mutex.Lock() // Lock the mutex before modifying app.sims
	defer app.mutex.Unlock()
	if err := os.RemoveAll(configDir); err != nil {
		return fmt.Errorf("error in os.RemoveAll: %v", err)
	}
	return nil
}

func threadSafeNewSim(fileContent []byte, queueItem *data.QueueItem, req *CreateQueueEntryRequest) (int64, error) {
	app.mutex.Lock() // Lock the mutex before modifying app.sims
	defer app.mutex.Unlock()
	var err error

	//----------------------------------------------
	// Create the directory if it doesn't exist
	//----------------------------------------------
	err = os.MkdirAll(app.QdConfigsDir, os.ModePerm)
	if err != nil {
		return 0, fmt.Errorf("handleNewSimulation: failed to create directory: %v", err)
	}

	//----------------------------------------------
	// Create a new file in the qdconfigs directory
	//----------------------------------------------
	tempFile, err := os.CreateTemp(app.QdConfigsDir, "config-*.json5")
	if err != nil {
		return 0, fmt.Errorf("handleNewSimulation: failed to create temp file in directory %s: %v", app.QdConfigsDir, err)
	}
	defer tempFile.Close()
	if len(fileContent) == 0 {
		return 0, fmt.Errorf("handleNewSimulation: no file content. 0-length file")
	}

	//----------------------------------------------
	// Write the file content to a TEMPORARY file
	//----------------------------------------------
	if _, err := tempFile.Write(fileContent); err != nil {
		return 0, fmt.Errorf("handleNewSimulation: failed to write file content: %v", err)
	}

	sid, err := app.qm.InsertItem(*queueItem)
	if err != nil {
		return 0, fmt.Errorf("handleNewSimulation: failed to insert new item to database: %v", err)
	}

	//----------------------------------------------
	// Make the new directory
	//----------------------------------------------
	fpath := filepath.Join(app.QdConfigsDir, fmt.Sprintf("%d", sid))
	err = os.MkdirAll(fpath, os.ModePerm)
	if err != nil {
		return 0, fmt.Errorf("handleNewSimulation: failed to make directory %s: %v", fpath, err)
	}

	//---------------------------------------------------------------------
	// Rename the file to include the queue item ID and original filename
	//---------------------------------------------------------------------
	newFilePath := filepath.Join(app.QdConfigsDir, fmt.Sprintf("%d", sid), req.OriginalFilename)
	if err := os.Rename(tempFile.Name(), newFilePath); err != nil {
		return 0, fmt.Errorf("handleNewSimulation: failed to rename %s to %s: %v", tempFile.Name(), newFilePath, err)
	}

	return sid, nil
}

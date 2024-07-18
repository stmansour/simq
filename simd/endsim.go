package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

// EndSimulationRequest represents the data for ending a simulation
type EndSimulationRequest struct {
	Command  string
	Username string
	SID      int64  // simulation ID that has ended
	Filename string // the tar.gz file that contains the results
}

func (sim *Simulation) sendEndSimulationRequest() error {
	//---------------------
	// INITIALIZE...
	//---------------------
	username := "simd"
	sid := sim.SID
	filePath := sim.Directory
	filename := "results.tar.gz"

	//------------------------------------
	// Open the file...
	//------------------------------------
	file, err := os.Open(filePath + "/" + filename)
	if err != nil {
		return fmt.Errorf("sendEndSimulationRequest: failed to open file: %w", err)
	}
	defer file.Close()

	//------------------------------------
	// Create a new multipart writer
	//------------------------------------
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	//------------------------------------
	// Create the JSON part
	//------------------------------------
	cmd := EndSimulationRequest{
		Command:  "EndSimulation",
		Username: username,
		SID:      sid,
		Filename: filename,
	}
	jsonData, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("sendEndSimulationRequest: failed to marshal JSON data: %w", err)
	}

	err = writer.WriteField("data", string(jsonData))
	if err != nil {
		return fmt.Errorf("sendEndSimulationRequest: failed to write JSON data: %w", err)
	}

	//------------------------------------
	// Create the file part
	//------------------------------------
	filePart, err := writer.CreateFormFile("file", filepath.Base(filename))
	if err != nil {
		return fmt.Errorf("sendEndSimulationRequest: failed to create file part: %w", err)
	}
	_, err = io.Copy(filePart, file)
	if err != nil {
		return fmt.Errorf("sendEndSimulationRequest: failed to copy file data: %w", err)
	}

	//------------------------------------
	// Close the multipart writer
	//------------------------------------
	err = writer.Close()
	if err != nil {
		return fmt.Errorf("sendEndSimulationRequest: failed to close multipart writer: %w", err)
	}

	//------------------------------------
	// Send the request
	//------------------------------------
	req, err := http.NewRequest("POST", app.cfg.FQDispatcherURL, body)
	if err != nil {
		return fmt.Errorf("sendEndSimulationRequest: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sendEndSimulationRequest: failed to send request: %w", err)
	}
	defer resp.Body.Close()

	//------------------------------------
	// Check for successful response
	//------------------------------------
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sendEndSimulationRequest: unexpected status code: %d", resp.StatusCode)
	}

	//------------------------------------
	// REMOVE THE SIMULATION DIRECTORY
	//------------------------------------
	err = os.RemoveAll(filePath)
	if err != nil {
		return fmt.Errorf("sendEndSimulationRequest: failed to remove directory: %w", err)
	}

	//--------------------------------------
	// REMOVE THE SIMULATION FROM APP.SIMS
	//--------------------------------------
	app.simsMu.Lock() // Lock the mutex before modifying app.sims
	defer app.simsMu.Unlock()
	for i, s := range app.sims {
		if s.SID == sim.SID {
			app.sims = append(app.sims[:i], app.sims[i+1:]...) // Remove sim from app.sims
			break
		}
	}

	return nil
}

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/stmansour/simq/util"
)

// SimulationBookingRequest represents the data for booking a simulation
type SimulationBookingRequest struct {
	Command         string
	Username        string
	MachineID       string
	CPUs            int
	Memory          string
	CPUArchitecture string
	Availability    string
}

// BookedResponse represents the response from the book command
type BookedResponse struct {
	Status         string
	SID            int64
	ConfigFilename string
}

// SvcStatus200 is a simple status message
type SvcStatus200 struct {
	Status  string
	Message string
}

// bookAndRunSimulation books a simulation and runs it
func bookAndRunSimulation(bkcmd string, sid int64) error {
	var err error
	var dataBytes []byte
	var machineID string

	cmd := util.Command{
		Command:  bkcmd,
		Username: "simd",
	}
	machineID, err = util.GetMachineUUID()
	if err != nil {
		return fmt.Errorf("bookAndRunSimulation: failed to get machine ID: %v", err)
	}

	if app.Paused {
		return fmt.Errorf("bookAndRunSimulation: booking is paused")
	}

	if bkcmd == "Book" {
		cmdDataStruct := struct {
			MachineID       string
			CPUs            int
			Memory          string
			CPUArchitecture string
			Availability    string
		}{
			MachineID:       machineID,
			CPUs:            10,
			Memory:          "64GB",
			CPUArchitecture: "ARM64",
			Availability:    "always",
		}
		dataBytes, err = json.Marshal(cmdDataStruct)
		if err != nil {
			return fmt.Errorf("bookAndRunSimulation: failed to marshal book request: %v", err)
		}
	} else if bkcmd == "Rebook" {
		cmdDataStruct := struct {
			MachineID string
			SID       int64
		}{
			MachineID: machineID,
			SID:       sid,
		}
		dataBytes, err = json.Marshal(cmdDataStruct)
		if err != nil {
			return fmt.Errorf("bookAndRunSimulation: failed to marshal book request: %v", err)
		}
	}
	cmd.Data = json.RawMessage(dataBytes)
	bookData, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("bookAndRunSimulation: failed to marshal book request: %v", err)
	}

	if app.HTTPHdrsDbg {
		PrintHexAndASCII(bookData, len(bookData))
	}

	//----------------------------------------
	// Create the URL to the dispatcher
	//----------------------------------------
	req, err := http.NewRequest("POST", app.cfg.FQDispatcherURL, bytes.NewBuffer(bookData))
	if err != nil {
		return fmt.Errorf("bookAndRunSimulation: failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	//----------------------------------------
	// DEBUG:Print request headers
	//----------------------------------------
	if app.HTTPHdrsDbg {
		log.Println("Request Headers:")
		for k, v := range req.Header {
			log.Printf("%s: %s\n", k, v)
		}
	}

	//----------------------------------------
	// Send the request
	//----------------------------------------
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("bookAndRunSimulation: failed to send book request: %v", err)
	}
	defer resp.Body.Close()

	//----------------------------------------
	// Read the response
	//----------------------------------------
	var bookResp struct {
		Status         string
		Message        string
		SID            int64
		ConfigFilename string
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error reading response body:", err)
		return err
	}

	//----------------------------------------
	// DEBUG:
	//----------------------------------------
	if app.HTTPHdrsDbg {
		PrintHexAndASCII(bodyBytes, len(bodyBytes))
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bookAndRunSimulation: unexpected status code: %v, body: %s", resp.StatusCode, string(bodyBytes))
	}

	//-----------------------------------------------------------
	// Determine whether the response is multipart or just JSON
	//-----------------------------------------------------------
	contentType := resp.Header.Get("Content-Type")
	if app.HTTPHdrsDbg {
		log.Println("Response Content-Type:", contentType) // Debugging
	}

	var FQConfigFileName string
	if strings.HasPrefix(contentType, "multipart/") {
		//=========================================================
		// MULTIPART
		//=========================================================
		boundary := strings.Split(contentType, "boundary=")[1]
		if app.HTTPHdrsDbg {
			log.Println("Multipart boundary:", boundary) // Debugging
		}
		multipartReader := multipart.NewReader(bytes.NewReader(bodyBytes), boundary)

		for {
			part, err := multipartReader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("bookAndRunSimulation: error reading multipart response: %v", err) // Debugging
			}

			switch part.FormName() {
			case "json":
				if err := json.NewDecoder(part).Decode(&bookResp); err != nil {
					return fmt.Errorf("bookAndRunSimulation: failed to unmarshal book response: %v", err)
				}
			case "file":
				configDir := filepath.Join(app.cfg.SimdSimulationsDir, "simulations", fmt.Sprintf("%d", bookResp.SID))
				log.Printf("BOOK CMD:  configDir = %s\n", configDir)
				os.MkdirAll(configDir, os.ModePerm)
				FQConfigFileName = filepath.Join(configDir, bookResp.ConfigFilename)

				out, err := os.Create(FQConfigFileName)
				if err != nil {
					log.Printf("Error creating config file: %v\n", err)
					return fmt.Errorf("bookAndRunSimulation: failed to create config file: %v", err)
				}
				defer out.Close()
				if _, err := io.Copy(out, part); err != nil {
					return fmt.Errorf("bookAndRunSimulation: failed to write config file: %v", err)
				}
				log.Printf("BOOK CMD: config file written: %s\n", FQConfigFileName)
			}
		}
		return startSimulator(bookResp.SID, FQConfigFileName)
	} else if strings.HasPrefix(contentType, "application/json") {
		//=========================================================
		// SINGLE-PART
		//=========================================================
		var respMessage struct {
			Status  string
			Message string
		}
		if err := json.Unmarshal(bodyBytes, &respMessage); err != nil {
			return fmt.Errorf(">>>> bookAndRunSimulation: failed to unmarshal response: %v", err)
		}
		if respMessage.Message == "no items in queue" {
			log.Printf(">>>> bookAndRunSimulation: dispatcher has no items in the queue\n")
			return nil // This is an expected response
		} else if respMessage.Status != "success" {
			if strings.Contains(respMessage.Message, "no items in queue") {
				log.Printf(">>>> bookAndRunSimulation: dispatcher has no items in the queue\n")
				return nil
			}
			log.Printf("**** ERROR **** bookAndRunSimulation: Failed to book simulation: %s", respMessage.Message)
		}
	}

	return nil
}

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/stmansour/simq/util"
)

// SimdStatus represents the structure of the status response from simd.
type SimdStatus struct {
	ProgramStarted        time.Time
	SimulationsInProgress int
	Paused                bool
	MaxSimulations        int
}

// GetSimdStatus contacts the simd HTTP API to retrieve the current status.
func GetSimdStatus(dcmd *CmdData, args []string) {
	// Create the full URL for the status endpoint
	fullURL, err := util.BuildURL(app.SimdURL, "/Status")
	if err != nil {
		fmt.Println("Error building URL:", err)
		return
	}
	// Make the HTTP GET request
	resp, err := http.Get(fullURL)
	if err != nil {
		fmt.Printf("Failed to contact simd @ %s: %v", fullURL, err)
		return
	}
	defer resp.Body.Close()

	// Read and parse the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response from simd: %v", err)
		return
	}

	// Check if the HTTP status code indicates success
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Non-OK HTTP status: %v", resp.StatusCode)
		return
	}

	// Unmarshal the JSON response into a SimdStatus struct
	var status SimdStatus
	if err := json.Unmarshal(body, &status); err != nil {
		log.Fatalf("Failed to parse JSON response: %v", err)
		return
	}

	fmt.Printf("   simd was started: %s\n", status.ProgramStarted.Format("2006-01-02 15:04:05"))
	fmt.Printf("             uptime: %v\n", time.Since(status.ProgramStarted))
	fmt.Printf("running simulations: in progress: %d\n", status.SimulationsInProgress)
	if status.Paused {
		fmt.Println("             status: simd is paused and will not start new simulations until it is unpaused")
	} else {
		fmt.Printf("             status: normal, can book up to %d more simulation(s) from the dispatcher.\n", status.MaxSimulations)
	}

}

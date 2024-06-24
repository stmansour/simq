package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/stmansour/simq/data"
)

// Command represents the structure of a command
type Command struct {
	Command  string          `json:"command"`
	Username string          `json:"username"`
	Data     json.RawMessage `json:"data"`
}

// CreateQueueEntryRequest represents the data for creating a queue entry
type CreateQueueEntryRequest struct {
	File        string `json:"file"`
	Name        string `json:"name"`
	Priority    int    `json:"priority"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

// commandDispatcher dispatches commands to appropriate handlers
func commandDispatcher(w http.ResponseWriter, r *http.Request) {
	var cmd Command
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	switch cmd.Command {
	case "NewSimulation":
		handleNewSimulation(w, cmd)
	case "Shutdown":
		handleShutdown(w, cmd)
	default:
		http.Error(w, "Unknown command", http.StatusBadRequest)
	}
}

// handleNewSimulation handles the NewSimulation command
func handleNewSimulation(w http.ResponseWriter, cmd Command) {
	var req CreateQueueEntryRequest
	if err := json.Unmarshal(cmd.Data, &req); err != nil {
		http.Error(w, "Invalid request data", http.StatusBadRequest)
		return
	}

	// Temporary placeholder for file handling
	req.File = "placeholder"

	queueItem := data.QueueItem{
		File:        req.File,
		Name:        req.Name,
		Priority:    req.Priority,
		Description: req.Description,
		URL:         req.URL,
		State:       data.StateQueued,
	}

	sid, err := app.qm.InsertItem(queueItem)
	if err != nil {
		http.Error(w, "Failed to insert queue item", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(fmt.Sprintf("Created queue item with SID %d", sid)))
}

// handleShutdown handles the Shutdown command
func handleShutdown(w http.ResponseWriter, cmd Command) {
	w.Write([]byte("Server is shutting down"))
	go func() {
		time.Sleep(1 * time.Second) // Give the response time to be sent
		app.quit <- os.Interrupt    // Signal the quit channel to initiate shutdown
	}()
}

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
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
	FileContent      string `json:"FileContent"`
	Name             string `json:"name"`
	Priority         int    `json:"priority"`
	Description      string `json:"description"`
	URL              string `json:"url"`
	OriginalFilename string `json:"original_filename"`
}

// UpdateItemRequest represents the data for updating a queue item
type UpdateItemRequest struct {
	SID         int    `json:"sid"`
	Priority    int    `json:"priority"`
	Description string `json:"description"`
}

// DeleteItemRequest represents the data for deleting a queue item
type DeleteItemRequest struct {
	SID int `json:"sid"`
}

// HandlerTableEntry represents an entry in the handler table
type HandlerTableEntry struct {
	Handler func(w http.ResponseWriter, r *http.Request, cmd *Command)
}

var handlerTable = map[string]HandlerTableEntry{
	"DeleteItem":     {Handler: handleDeleteItem},
	"GetActiveQueue": {Handler: handleGetActiveQueue},
	"NewSimulation":  {Handler: handleNewSimulation},
	"Shutdown":       {Handler: handleShutdown},
	"UpdateItem":     {Handler: handleUpdateItem},
}

// commandDispatcher dispatches commands to appropriate handlers
// commandDispatcher dispatches commands to appropriate handlers
func commandDispatcher(w http.ResponseWriter, r *http.Request) {
	var cmd Command
	var ok bool
	h := HandlerTableEntry{}

	// Check if the request is multipart/form-data
	if r.Header.Get("Content-Type") != "" && strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
		// Parse multipart form
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, "Failed to parse multipart form", http.StatusBadRequest)
			return
		}

		// Extract command fields from form data
		cmd.Command = r.FormValue("command")
		cmd.Username = r.FormValue("username")

		// Extract and marshal the data part
		dataPart := r.FormValue("data")
		cmd.Data = json.RawMessage(dataPart)
	} else {
		// Parse JSON payload
		if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}
	}

	log.Printf("\tcommand: %s", cmd.Command)

	if h, ok = handlerTable[cmd.Command]; !ok {
		log.Printf("\tUnknown command: %s", cmd.Command)
		http.Error(w, "Unknown command", http.StatusBadRequest)
		return
	}

	h.Handler(w, r, &cmd)
}

// handleNewSimulation handles the NewSimulation command
// It creates a new entry in the queue
// ---------------------------------------------------------------------------
// handleNewSimulation handles the NewSimulation command
func handleNewSimulation(w http.ResponseWriter, r *http.Request, cmd *Command) {
	var req CreateQueueEntryRequest
	err := json.Unmarshal(cmd.Data, &req)
	if err != nil {
		http.Error(w, "Failed to unmarshal request data", http.StatusBadRequest)
		return
	}

	// Retrieve the file from the form
	file, _, err := r.FormFile("file")
	if err != nil {
		log.Printf("Failed to get file from form: %v", err)
		http.Error(w, "Failed to get file from form", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Create the directory if it doesn't exist
	err = os.MkdirAll("qdconfigs", os.ModePerm)
	if err != nil {
		log.Printf("Failed to create directory: %v", err)
		http.Error(w, "Failed to create directory", http.StatusInternalServerError)
		return
	}

	// Create a new file in the qdconfigs directory
	tempFile, err := os.CreateTemp("qdconfigs", "config-*.json5")
	if err != nil {
		log.Printf("Failed to create qdconfigs directory: %v", err)
		http.Error(w, "Failed to create temp file", http.StatusInternalServerError)
		return
	}
	defer tempFile.Close()

	// Write the file content to the temporary file
	_, err = io.Copy(tempFile, file)
	if err != nil {
		log.Printf("Failed to write file content: %v", err)
		http.Error(w, "Failed to write file content", http.StatusInternalServerError)
		return
	}

	// Insert the queue item
	queueItem := data.QueueItem{
		File:        tempFile.Name(),
		Name:        req.Name,
		Priority:    req.Priority,
		Description: req.Description,
		URL:         req.URL,
		State:       data.StateQueued,
	}

	sid, err := app.qm.InsertItem(queueItem)
	if err != nil {
		log.Printf("Failed to insert new item to database: %v", err)
		http.Error(w, "Failed to insert queue item", http.StatusInternalServerError)
		return
	}

	// Create the new directory for the simulation using the SID
	err = os.MkdirAll(fmt.Sprintf("qdconfigs/%d", sid), os.ModePerm)
	if err != nil {
		log.Printf("Failed to create directory qdconfigs/%d: %v", sid, err)
		http.Error(w, "Failed to create directory", http.StatusInternalServerError)
		return
	}

	// Rename the file to include the original filename
	newFilePath := fmt.Sprintf("qdconfigs/%d/%s", sid, req.OriginalFilename)
	if err := os.Rename(tempFile.Name(), newFilePath); err != nil {
		log.Printf("Failed to rename %s to %s: %v", tempFile.Name(), newFilePath, err)
		http.Error(w, "Failed to rename file", http.StatusInternalServerError)
		return
	}

	msg := SvcStatus201{
		Status:  "success",
		Message: "Created queue item",
		ID:      int(sid),
	}
	w.WriteHeader(http.StatusCreated)
	SvcWriteResponse(w, &msg)
}

// handleShutdown handles the Shutdown command
func handleShutdown(w http.ResponseWriter, r *http.Request, cmd *Command) {
	log.Println("Shutdown command received")
	resp := SvcStatus200{
		Status:  "success",
		Message: "Shutting down",
	}
	SvcWriteResponse(w, &resp)
	go func() {
		time.Sleep(1 * time.Second) // Give the response time to be sent
		app.quit <- os.Interrupt    // Signal the quit channel to initiate shutdown
	}()
}

// handleGetActiveQueue handles the GetActiveQueue command
func handleGetActiveQueue(w http.ResponseWriter, r *http.Request, cmd *Command) {
	items, err := app.qm.GetQueuedAndExecutingItems()
	if err != nil {
		http.Error(w, "Failed to get active queue items", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	resp := struct {
		Status string
		Data   []data.QueueItem
	}{
		Status: "success",
		Data:   items,
	}
	SvcWriteResponse(w, &resp)
}

// handleUpdateItem handles the UpdateItem command
func handleUpdateItem(w http.ResponseWriter, r *http.Request, cmd *Command) {
	var req UpdateItemRequest
	if err := json.Unmarshal(cmd.Data, &req); err != nil {
		http.Error(w, "Invalid request data", http.StatusBadRequest)
		return
	}

	queueItem, err := app.qm.GetItemByID(req.SID)
	if err != nil {
		http.Error(w, "Item not found", http.StatusNotFound)
		return
	}

	queueItem.Priority = req.Priority
	queueItem.Description = req.Description

	if err := app.qm.UpdateItem(queueItem); err != nil {
		http.Error(w, "Failed to update queue item", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	msg := SvcStatus201{
		Status:  "success",
		Message: "Updated",
		ID:      int(queueItem.SID),
	}
	SvcWriteResponse(w, &msg)
}

// handleDeleteItem handles the DeleteItem command
func handleDeleteItem(w http.ResponseWriter, r *http.Request, cmd *Command) {
	var req DeleteItemRequest
	if err := json.Unmarshal(cmd.Data, &req); err != nil {
		http.Error(w, "Invalid request data", http.StatusBadRequest)
		return
	}

	if err := app.qm.DeleteItem(req.SID); err != nil {
		http.Error(w, "Failed to delete queue item", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	msg := SvcStatus201{
		Status:  "success",
		Message: "deleted",
		ID:      int(req.SID),
	}
	SvcWriteResponse(w, &msg)
}

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
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
	OriginalFilename string `json:"OriginalFilename"`
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

// BookedResponse represents the response for booking a simulation
type BookedResponse struct {
	Status         string
	SID            int64
	ConfigFilename string
}

var handlerTable = map[string]HandlerTableEntry{
	"Book":           {Handler: handleBook},
	"DeleteItem":     {Handler: handleDeleteItem},
	"GetActiveQueue": {Handler: handleGetActiveQueue},
	"NewSimulation":  {Handler: handleNewSimulation},
	"Shutdown":       {Handler: handleShutdown},
	"UpdateItem":     {Handler: handleUpdateItem},
}

// LogAndErrorReturn logs an error and returns an error
func LogAndErrorReturn(w http.ResponseWriter, err error) {
	log.Printf("Error: %v", err)
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

// commandDispatcher dispatches commands to appropriate handlers
// -----------------------------------------------------------------------------
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
		LogAndErrorReturn(w, fmt.Errorf("unknown command: %s", cmd.Command))
		return
	}

	h.Handler(w, r, &cmd)
}

// handleBook handles the Book command
// ---------------------------------------------------------------------------
func handleBook(w http.ResponseWriter, r *http.Request, cmd *Command) {
	// Decode the booking request
	var bookingRequest SimulationBookingRequest
	if err := json.Unmarshal(cmd.Data, &bookingRequest); err != nil {
		http.Error(w, "Invalid booking request data", http.StatusBadRequest)
		return
	}

	//---------------------------------------------------
	// Retrieve the highest priority job from the queue
	//---------------------------------------------------
	queueItem, err := app.qm.GetHighestPriorityQueuedItem()
	if err != nil {
		http.Error(w, "Failed to get highest priority queued item", http.StatusInternalServerError)
		return
	}

	//*****************************************************************************
	// Mark the item as booked
	//*****************************************************************************
	queueItem.State = data.StateBooked
	if err := app.qm.UpdateItem(queueItem); err != nil {
		http.Error(w, "Failed to update queue item", http.StatusInternalServerError)
		return
	}

	//-----------------------------------------------------------------------------
	// Create the response
	//-----------------------------------------------------------------------------
	response := BookedResponse{
		Status:         "success",
		SID:            queueItem.SID,
		ConfigFilename: "config.json5",
	}

	//-------------------------
	// multipart writer
	//-------------------------
	multipartWriter := multipart.NewWriter(w)
	w.Header().Set("Content-Type", multipartWriter.FormDataContentType())

	//-------------------------
	// JSON part
	//-------------------------
	jsonWriter, err := multipartWriter.CreateFormField("json")
	if err != nil {
		http.Error(w, "Failed to create JSON part", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(jsonWriter).Encode(response); err != nil {
		http.Error(w, "Failed to encode JSON response", http.StatusInternalServerError)
		return
	}

	//----------------------------
	// Config file part
	//----------------------------
	configFilePath := fmt.Sprintf("qdconfigs/%d/%s", queueItem.SID, response.ConfigFilename)
	configFile, err := os.Open(configFilePath)
	if err != nil {
		http.Error(w, "Failed to open config file", http.StatusInternalServerError)
		return
	}
	defer configFile.Close()

	fileWriter, err := multipartWriter.CreateFormFile("file", response.ConfigFilename)
	if err != nil {
		http.Error(w, "Failed to create file part", http.StatusInternalServerError)
		return
	}
	if _, err := io.Copy(fileWriter, configFile); err != nil {
		http.Error(w, "Failed to send config file", http.StatusInternalServerError)
		return
	}

	//----------------------------
	// Close the multipart writer
	//----------------------------
	if err := multipartWriter.Close(); err != nil {
		http.Error(w, "Failed to close multipart writer", http.StatusInternalServerError)
		return
	}
}

// handleNewSimulation handles the NewSimulation command
// It creates a new entry in the queue
// ---------------------------------------------------------------------------
func handleNewSimulation(w http.ResponseWriter, r *http.Request, cmd *Command) {
	// Parse the multipart form data
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		LogAndErrorReturn(w, fmt.Errorf("failed to parse multipart form: %v", err))
		return
	}

	// Get the command data part
	dataPart := r.FormValue("data")
	if dataPart == "" {
		http.Error(w, "Missing command data part", http.StatusBadRequest)
		return
	}

	// Unmarshal the command data into CreateQueueEntryRequest
	var req CreateQueueEntryRequest
	if err := json.Unmarshal([]byte(dataPart), &req); err != nil {
		http.Error(w, "Failed to unmarshal request data", http.StatusBadRequest)
		return
	}

	// Get the file from the form
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to get file from form", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read the file content
	fileContent, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file content", http.StatusInternalServerError)
		return
	}

	// Create the directory if it doesn't exist
	err = os.MkdirAll("qdconfigs", os.ModePerm)
	if err != nil {
		LogAndErrorReturn(w, fmt.Errorf("failed to create directory: %v", err))
		return
	}

	// Create a new file in the qdconfigs directory
	tempFile, err := os.CreateTemp("qdconfigs", "config-*.json5")
	if err != nil {
		LogAndErrorReturn(w, fmt.Errorf("failed to create qdconfigs directory: %v", err))
		return
	}
	defer tempFile.Close()

	if len(fileContent) == 0 {
		LogAndErrorReturn(w, fmt.Errorf("no file content. 0-length file"))
		return
	}

	// Write the file content to the temp file
	if _, err := tempFile.Write(fileContent); err != nil {
		LogAndErrorReturn(w, fmt.Errorf("failed to write file content: %v", err))
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
		LogAndErrorReturn(w, fmt.Errorf("failed to insert new item to database: %v", err))
		return
	}

	// Make the new directory
	err = os.MkdirAll(fmt.Sprintf("qdconfigs/%d", sid), os.ModePerm)
	if err != nil {
		LogAndErrorReturn(w, fmt.Errorf("failed to make directory qdconfigs/%d: %v", sid, err))
		return
	}

	// Rename the file to include the queue item ID and original filename
	newFilePath := fmt.Sprintf("qdconfigs/%d/%s", sid, req.OriginalFilename)
	if err := os.Rename(tempFile.Name(), newFilePath); err != nil {
		LogAndErrorReturn(w, fmt.Errorf("failed to rename %s to %s: %v", tempFile.Name(), newFilePath, err))
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
// -----------------------------------------------------------------------------
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
// -----------------------------------------------------------------------------
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
// -----------------------------------------------------------------------------
// handleDeleteItem handles the DeleteItem command
func handleDeleteItem(w http.ResponseWriter, r *http.Request, cmd *Command) {
	var req DeleteItemRequest
	if err := json.Unmarshal(cmd.Data, &req); err != nil {
		http.Error(w, "Invalid request data", http.StatusBadRequest)
		return
	}

	// Retrieve the queue item to get the associated file path
	_, err := app.qm.GetItemByID(req.SID)
	if err != nil {
		http.Error(w, "Item not found", http.StatusNotFound)
		return
	}

	// Delete the file and directory associated with the queue item
	dirPath := fmt.Sprintf("qdconfigs/%d", req.SID)
	if err := os.RemoveAll(dirPath); err != nil {
		LogAndErrorReturn(w, fmt.Errorf("failed to remove directory %s: %v", dirPath, err))
		return
	}

	// Delete the queue item from the database
	if err := app.qm.DeleteItem(req.SID); err != nil {
		LogAndErrorReturn(w, fmt.Errorf("failed to delete queue item %d: %v", req.SID, err))
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

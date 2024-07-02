package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/stmansour/simq/data"
	"github.com/stmansour/simq/util"
)

// Command represents the structure of a command
type Command struct {
	Command  string
	Username string
	Data     json.RawMessage
}

// CreateQueueEntryRequest represents the data for creating a queue entry
type CreateQueueEntryRequest struct {
	FileContent      string
	Name             string
	Priority         int
	Description      string
	URL              string
	OriginalFilename string
}

// UpdateItemRequest represents the data for updating a queue item
type UpdateItemRequest struct {
	SID         int64
	Priority    int
	Description string
	MachineID   string
	URL         string
	DtEstimate  string
	DtCompleted string
	CPUs        int
	Memory      string
}

// DeleteItemRequest represents the data for deleting a queue item
type DeleteItemRequest struct {
	SID int64
}

// HandlerTableEntry represents an entry in the handler table
type HandlerTableEntry struct {
	Handler func(w http.ResponseWriter, r *http.Request, h *HInfo)
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
	Message        string
	SID            int64
	ConfigFilename string
}

// HInfo holds information about the request
type HInfo struct {
	cmd       *Command
	BodyBytes []byte
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
	SvcErrorReturn(w, err)
}

// commandDispatcher dispatches commands to appropriate handlers
// -----------------------------------------------------------------------------
func commandDispatcher(w http.ResponseWriter, r *http.Request) {
	var cmd Command
	var ok bool
	h := HandlerTableEntry{}
	var bodyBytes []byte
	var err error

	//------------------------------
	// DEBUG
	//------------------------------
	if app.HTTPHdrsDbg {
		log.Println("Request Headers:")
		for k, v := range r.Header {
			log.Printf("%s: %s\n", k, v)
		}
	}

	//-------------------------------------------------
	// Could be multipart, could be single part...
	//-------------------------------------------------
	if r.Header.Get("Content-Type") != "" && strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
		//===============================================
		// MULTIPART
		//===============================================
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			SvcErrorReturn(w, fmt.Errorf("failed to parse multipart form"))
			return
		}

		cmd.Command = r.FormValue("command")
		cmd.Username = r.FormValue("username")
		dataPart := r.FormValue("data")
		cmd.Data = json.RawMessage(dataPart)

	} else {
		//===============================================
		// SINGLE-PART
		//===============================================
		bodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			SvcErrorReturn(w, fmt.Errorf("failed to read request body"))
			return
		}
		if app.HexASCIIDbg {
			PrintHexAndASCII(bodyBytes, 256)
		}
		if err := json.Unmarshal(bodyBytes, &cmd); err != nil {
			SvcErrorReturn(w, fmt.Errorf("invalid request payload"))
			return
		}
	}

	d := HInfo{BodyBytes: bodyBytes, cmd: &cmd}
	log.Printf("\tcommand: %s, username: %s", cmd.Command, cmd.Username)

	if h, ok = handlerTable[cmd.Command]; !ok {
		LogAndErrorReturn(w, fmt.Errorf("unknown command: %s", cmd.Command))
		return
	}

	h.Handler(w, r, &d)
}

// handleBook handles the Book command
// ---------------------------------------------------------------------------
func handleBook(w http.ResponseWriter, r *http.Request, d *HInfo) {
	//---------------------------------------------------
	// Decode the booking request
	//---------------------------------------------------
	var bookingRequest SimulationBookingRequest
	if err := json.Unmarshal(d.cmd.Data, &bookingRequest); err != nil {
		SvcErrorReturn(w, fmt.Errorf("invalid booking request data"))
		return
	}

	//---------------------------------------------------
	// Retrieve the highest priority job from the queue
	//---------------------------------------------------
	queueItem, err := app.qm.GetHighestPriorityQueuedItem()
	if err != nil {
		if err == sql.ErrNoRows {
			msg := SvcStatus201{
				Status:  "success",
				Message: "no items in queue",
				ID:      0,
			}
			w.WriteHeader(http.StatusOK)
			SvcWriteResponse(w, &msg)
		}
		SvcErrorReturn(w, fmt.Errorf("failed to get highest priority queued item"))
		return
	}

	//*****************************************************************************
	// Mark the item as booked
	//*****************************************************************************
	queueItem.State = data.StateBooked
	queueItem.MachineID = bookingRequest.MachineID
	if err := app.qm.UpdateItem(queueItem); err != nil {
		SvcErrorReturn(w, fmt.Errorf("failed to update queue item"))
		return
	}

	//-----------------------------------------------------------------------------
	// Create the response
	//-----------------------------------------------------------------------------
	response := BookedResponse{
		Status:         "success",
		Message:        "simulation booked",
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
		SvcErrorReturn(w, fmt.Errorf("failed to create JSON part"))
		return
	}
	if err := json.NewEncoder(jsonWriter).Encode(response); err != nil {
		SvcErrorReturn(w, fmt.Errorf("failed to encode JSON response"))
		return
	}

	//----------------------------
	// Config file part
	//----------------------------
	configFilePath := fmt.Sprintf("qdconfigs/%d/%s", queueItem.SID, response.ConfigFilename)
	configFile, err := os.Open(configFilePath)
	if err != nil {
		SvcErrorReturn(w, fmt.Errorf("failed to open config file"))
		return
	}
	defer configFile.Close()

	fileWriter, err := multipartWriter.CreateFormFile("file", response.ConfigFilename)
	if err != nil {
		SvcErrorReturn(w, fmt.Errorf("failed to create file part"))
		return
	}
	if _, err := io.Copy(fileWriter, configFile); err != nil {
		SvcErrorReturn(w, fmt.Errorf("failed to send config file"))
		return
	}

	//----------------------------
	// Close the multipart writer
	//----------------------------
	if err := multipartWriter.Close(); err != nil {
		SvcErrorReturn(w, fmt.Errorf("failed to close multipart writer"))
		return
	}
}

// handleNewSimulation handles the NewSimulation command
// It creates a new entry in the queue
// ---------------------------------------------------------------------------
func handleNewSimulation(w http.ResponseWriter, r *http.Request, d *HInfo) {
	// Parse the multipart form data
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		LogAndErrorReturn(w, fmt.Errorf("failed to parse multipart form: %v", err))
		return
	}

	// Get the command data part
	dataPart := r.FormValue("data")
	if dataPart == "" {
		SvcErrorReturn(w, fmt.Errorf("missing command data part"))
		return
	}

	// Unmarshal the command data into CreateQueueEntryRequest
	var req CreateQueueEntryRequest
	if err := json.Unmarshal([]byte(dataPart), &req); err != nil {
		SvcErrorReturn(w, fmt.Errorf("failed to unmarshal request data"))
		return
	}

	// Get the file from the form
	file, _, err := r.FormFile("file")
	if err != nil {
		SvcErrorReturn(w, fmt.Errorf("failed to get file from form"))
		return
	}
	defer file.Close()

	// Read the file content
	fileContent, err := io.ReadAll(file)
	if err != nil {
		SvcErrorReturn(w, fmt.Errorf("failed to read file content"))
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
		File:        req.OriginalFilename,
		Username:    d.cmd.Username,
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
		ID:      sid,
	}
	w.WriteHeader(http.StatusCreated)
	SvcWriteResponse(w, &msg)
}

// handleShutdown handles the Shutdown command
func handleShutdown(w http.ResponseWriter, r *http.Request, d *HInfo) {
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
func handleGetActiveQueue(w http.ResponseWriter, r *http.Request, d *HInfo) {
	items, err := app.qm.GetQueuedAndExecutingItems()
	if err != nil {
		SvcErrorReturn(w, fmt.Errorf("failed to get active queue items"))
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
func handleUpdateItem(w http.ResponseWriter, r *http.Request, d *HInfo) {
	z := string(rune(0x2026)) // the '...' character, we take this to mean "not set"

	//--------------------------------------------------------
	// The values for req indicate that the field is not set
	//--------------------------------------------------------
	req := UpdateItemRequest{
		Priority:    -1,
		Description: z,
		MachineID:   z,
		CPUs:        -1,
		Memory:      z,
		URL:         z,
		DtEstimate:  z,
		DtCompleted: z,
	}

	//--------------------------------------------------------
	// Unmarshal the data into the request struct. It will
	// only set the fields supplied by the caller...
	//--------------------------------------------------------
	if err := json.Unmarshal(d.cmd.Data, &req); err != nil {
		SvcErrorReturn(w, fmt.Errorf("invalid request data"))
		return
	}

	fmt.Printf("RECEIVED -UpdateItem: %+v\n", req)

	//--------------------------------------------------------
	// load the existing item to establish the base values
	//--------------------------------------------------------
	queueItem, err := app.qm.GetItemByID(req.SID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			SvcErrorReturn(w, fmt.Errorf("queue item %d not found", req.SID))
		} else {
			SvcErrorReturn(w, fmt.Errorf("error in GetItemByID: %v", err))
		}
		return
	}

	//--------------------------------------------------------
	// Update only the items that were supplied. The SID,
	// username, and ... cannot be changed
	//--------------------------------------------------------
	if req.Priority >= 0 {
		queueItem.Priority = req.Priority
	}
	if req.MachineID != z {
		queueItem.MachineID = req.MachineID
	}
	if req.URL != z {
		queueItem.URL = req.URL
	}
	if req.Description != z {
		queueItem.Description = req.Description
	}
	if req.DtEstimate != z && len(req.DtEstimate) > 0 {
		dt, err := util.StringToDate(req.DtEstimate)
		if err != nil {
			SvcErrorReturn(w, fmt.Errorf("invalid date: %s", req.DtEstimate))
			return
		}
		queueItem.DtEstimate.Time = dt
		queueItem.DtEstimate.Valid = true
		queueItem.State = data.StateExecuting
	}
	if req.DtCompleted != z && len(req.DtCompleted) > 0 {
		dt, err := util.StringToDate(req.DtCompleted)
		if err != nil {
			SvcErrorReturn(w, fmt.Errorf("invalid date: %s", req.DtCompleted))
			return
		}
		queueItem.DtCompleted.Time = dt
		queueItem.DtCompleted.Valid = true
		queueItem.State = data.StateCompleted
	}

	if err := app.qm.UpdateItem(queueItem); err != nil {
		SvcErrorReturn(w, fmt.Errorf("failed to update queue item"))
		return
	}

	w.WriteHeader(http.StatusOK)
	msg := SvcStatus201{
		Status:  "success",
		Message: "Updated",
		ID:      queueItem.SID,
	}
	SvcWriteResponse(w, &msg)
}

// handleDeleteItem handles the DeleteItem command
// -----------------------------------------------------------------------------
// handleDeleteItem handles the DeleteItem command
func handleDeleteItem(w http.ResponseWriter, r *http.Request, d *HInfo) {
	var req DeleteItemRequest
	if err := json.Unmarshal(d.cmd.Data, &req); err != nil {
		SvcErrorReturn(w, fmt.Errorf("invalid request data"))
		return
	}

	// Retrieve the queue item to get the associated file path
	_, err := app.qm.GetItemByID(req.SID)
	if err != nil {
		SvcErrorReturn(w, fmt.Errorf("item not found"))
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
		ID:      req.SID,
	}
	SvcWriteResponse(w, &msg)
}

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
	"os/exec"
	"path/filepath"
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

// MachineQueueRequest represents the data for creating a machine queue
type MachineQueueRequest struct {
	MachineID string
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

// EndSimulationRequest represents the data for ending a simulation
type EndSimulationRequest struct {
	Command  string
	Username string
	SID      int64  // simulation ID that has ended
	Filename string // the tar.gz file that contains the results
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

// SimulationRebookRequest represents the data for rebooking a simulation
type SimulationRebookRequest struct {
	SID       int64
	MachineID string
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
	"Book":              {Handler: handleBook},
	"DeleteItem":        {Handler: handleDeleteItem},
	"EndSimulation":     {Handler: handleEndSimulation},
	"GetActiveQueue":    {Handler: handleGetActiveQueue},
	"GetCompletedQueue": {Handler: handleGetCompletedQueue},
	"GetMachineQueue":   {Handler: handleGetMachineQueue},
	"NewSimulation":     {Handler: handleNewSimulation},
	"Rebook":            {Handler: handleBook},
	"Shutdown":          {Handler: handleShutdown},
	"UpdateItem":        {Handler: handleUpdateItem},
}

// LogAndErrorReturn logs an error and returns an error
func LogAndErrorReturn(w http.ResponseWriter, err error) {
	log.Printf("Error: %v", err)
	SvcErrorReturn(w, err)
}

// app.HTTPHdrsDbg = true
// app.HexASCIIDbg = true

// commandDispatcher dispatches commands to appropriate handlers
// -----------------------------------------------------------------------------
func commandDispatcher(w http.ResponseWriter, r *http.Request) {
	var cmd Command
	var ok bool
	var d HInfo
	h := HandlerTableEntry{}

	app.HTTPHdrsDbg = true
	app.HexASCIIDbg = true

	//--------------------------------
	// Check for Content-Type header
	//-------------------------------
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		SvcErrorReturn(w, fmt.Errorf("commandDispatcher: missing Content-Type header in request"))
		return
	}

	//--------------------------------------------
	// Process the request based on Content-Type
	//--------------------------------------------
	if strings.Contains(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			SvcErrorReturn(w, fmt.Errorf("commandDispatcher: failed to parse multipart form: %v", err))
			return
		}
		dataField := r.FormValue("data") // Extract the data part
		if dataField == "" {
			SvcErrorReturn(w, fmt.Errorf("commandDispatcher: missing data field in multipart request"))
			return
		}
		d.BodyBytes = []byte(dataField)
		if err := json.Unmarshal([]byte(dataField), &cmd); err != nil {
			SvcErrorReturn(w, fmt.Errorf("commandDispatcher: invalid data payload in multipart request: %v", err))
			return
		}
	} else {
		bodyBytes, err := io.ReadAll(r.Body) // Single-part request - unmarshal directly
		if err != nil {
			SvcErrorReturn(w, fmt.Errorf("commandDispatcher: failed to read request body: %v", err))
			return
		}
		if err := json.Unmarshal(bodyBytes, &cmd); err != nil {
			SvcErrorReturn(w, fmt.Errorf("commandDispatcher: invalid request payload: %v", err))
			return
		}

		d.BodyBytes = bodyBytes // Store the raw body bytes for use by the handlers
	}

	d.cmd = &cmd // Define d with the unmarshalled cmd struct and BodyBytes (if applicable)

	//--------------------
	// DEBUGGING
	//--------------------
	log.Printf("\tDispatcher: >>>> received command: %s, username: %s", cmd.Command, cmd.Username)

	//---------------------------------------------------------------
	// Access the handler table without mutex since it's read-only
	//---------------------------------------------------------------
	h, ok = handlerTable[cmd.Command]
	if !ok {
		log.Printf("Internal Error: handler not found for command: %s", cmd.Command)
		return
	}

	h.Handler(w, r, &d)
}

// handleEndSimulation handles the EndSimulation command.
//
//	The request body contains:
//	Cmd
//	    Command - the command, EndSimulation
//	    Username - the person or process making this call
//	    Data
//	        SID - the ID of the simulation
//	        Filename - the name of the tar.gz file
//
// ---------------------------------------------------------------------------
func handleEndSimulation(w http.ResponseWriter, r *http.Request, d *HInfo) {
	var cmd EndSimulationRequest

	if err := json.Unmarshal(d.BodyBytes, &cmd); err != nil {
		SvcErrorReturn(w, fmt.Errorf("handleEndSimulation: invalid end simulation request data"))
		return
	}

	log.Printf("EndSimulation: SID: %d, Filename: %s\n", cmd.SID, cmd.Filename)

	//----------------------------------------------------------------------------
	// BUILD THE DESTINATION DIRECTORY
	// /genome/simulation-results/YYYY/MM/DD/SID/results.tar.gz
	//
	// for testing: /opt/TestSimResultsRepo
	//----------------------------------------------------------------------------
	now := time.Now()
	year := now.Year()
	month := now.Month()
	day := now.Day()
	dirPath := filepath.Join(app.SimResultsDir,
		fmt.Sprintf("%d", year),
		fmt.Sprintf("%d", month),
		fmt.Sprintf("%d", day),
		fmt.Sprintf("%d", cmd.SID),
	)
	filename := filepath.Join(dirPath, cmd.Filename)

	//--------------------------------------------------
	// CREATE THE FULL PATH
	//--------------------------------------------------
	if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
		SvcErrorReturn(w, fmt.Errorf("handleEndSimulation: failed to create directory: %v", err))
		return
	}

	//----------------------------------------------------
	// EXTRACT THE FILE CONTENT
	//----------------------------------------------------
	file, _, err := r.FormFile("file")
	if err != nil {
		SvcErrorReturn(w, fmt.Errorf("handleEndSimulation: failed to get file from form"))
		return
	}
	defer file.Close()
	fileContent, err := io.ReadAll(file)
	if err != nil {
		SvcErrorReturn(w, fmt.Errorf("handleEndSimulation: failed to read file content"))
		return
	}

	//----------------------------------------------
	// CREATE THE TAR.GZ FILE
	//----------------------------------------------
	tarz, err := os.Create(filename)
	if err != nil {
		LogAndErrorReturn(w, fmt.Errorf("handleEndSimulation: failed to create file %s: %v", filename, err))
		return
	}
	defer tarz.Close()
	if len(fileContent) == 0 {
		LogAndErrorReturn(w, fmt.Errorf("handleEndSimulation: no file content. 0-length file"))
		return
	}

	//----------------------------------------------
	// WRITE THE TAR.GZ FILE
	//----------------------------------------------
	if _, err := tarz.Write(fileContent); err != nil {
		LogAndErrorReturn(w, fmt.Errorf("handleEndSimulation: failed to write to file %s: %v", filename, err))
		return
	}

	//----------------------------------------------
	// EXTRACT THE FILES FROM THE TAR.GZ FILE
	//----------------------------------------------
	originalDir, err := os.Getwd() // Save the current directory
	if err != nil {
		LogAndErrorReturn(w, fmt.Errorf("handleEndSimulation: failed to get current directory: %v", err))
		return
	}
	err = os.Chdir(dirPath) // Change to the target directory
	if err != nil {
		LogAndErrorReturn(w, fmt.Errorf("handleEndSimulation: failed to change directory to %s: %v", dirPath, err))
		return
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
		LogAndErrorReturn(w, fmt.Errorf("handleEndSimulation: failed to execute tar command after %d attempts: %v", maxRetries, err))
		return
	}

	err = os.Chdir(originalDir) // Return back to the original directory
	if err != nil {
		LogAndErrorReturn(w, fmt.Errorf("handleEndSimulation: failed to change back to the original directory: %v", err))
	}

	//----------------------------
	// REMOVE THE TAR.GZ FILE
	//----------------------------
	if err := os.Remove(filename); err != nil {
		LogAndErrorReturn(w, fmt.Errorf("handleEndSimulation: failed to remove config file"))
		return
	}

	//--------------------------------------------------------
	// UPDATE THE STATE OF THIS ITEM - RESULTS SAVED
	//--------------------------------------------------------
	queueItem, err := app.qm.GetItemByID(cmd.SID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			LogAndErrorReturn(w, fmt.Errorf("handleEndSimulation: queue item %d not found", cmd.SID))
		} else {
			LogAndErrorReturn(w, fmt.Errorf("handleEndSimulation: error in GetItemByID: %v", err))
		}
		return
	}

	queueItem.State = data.StateResultsSaved

	if err := app.qm.UpdateItem(queueItem); err != nil {
		LogAndErrorReturn(w, fmt.Errorf("handleEndSimulation: error in UpdateItem: %v", err))
		return
	}

	//------------------------------
	// SEND RESPONSE
	//------------------------------
	w.WriteHeader(http.StatusOK)
	resp := struct {
		Status  string
		Message string
	}{
		Status:  "success",
		Message: "Results stored in: " + dirPath,
	}
	SvcWriteResponse(w, &resp)
}

func executeTarCommand() error {
	tcmd := exec.Command("tar", "xzf", "results.tar.gz")
	tcmd.Stdout = os.Stdout
	tcmd.Stderr = os.Stderr
	return tcmd.Run()
}

// handleBook handles the Book command
// ---------------------------------------------------------------------------
func handleBook(w http.ResponseWriter, r *http.Request, d *HInfo) {
	var queueItem data.QueueItem
	var err error
	//---------------------------------------------------
	// Decode the booking request
	//---------------------------------------------------
	var bookingRequest SimulationBookingRequest
	var rebookRequest SimulationRebookRequest
	switch d.cmd.Command {
	case "Book":
		log.Printf("handling Book command\n")
		if err := json.Unmarshal(d.cmd.Data, &bookingRequest); err != nil {
			SvcErrorReturn(w, fmt.Errorf("handleBook: invalid booking request data"))
			return
		}
		//---------------------------------------------------
		// Retrieve the highest priority job from the queue
		//---------------------------------------------------
		queueItem, err = app.qm.GetHighestPriorityQueuedItem()
		if err != nil {
			if strings.Contains(err.Error(), "no queued items") {
				msg := SvcStatus201{
					Status:  "success",
					Message: "no queued items need booking",
					ID:      0,
				}
				w.WriteHeader(http.StatusOK)
				SvcWriteResponse(w, &msg)
				return
			}
			SvcErrorReturn(w, fmt.Errorf("handleBook: err: %s", err.Error()))
			return
		}
	case "Rebook":
		log.Printf("handling Rebook command\n")
		if err := json.Unmarshal(d.cmd.Data, &rebookRequest); err != nil {
			SvcErrorReturn(w, fmt.Errorf("handleBook: invalid rebook request data"))
			return
		}
		queueItem, err = app.qm.GetItemByID(rebookRequest.SID)
		if err != nil {
			SvcErrorReturn(w, fmt.Errorf("handleBook: err: %s", err.Error()))
			return
		}
		if queueItem.MachineID != rebookRequest.MachineID {
			log.Printf("*** WARNING *** Granted MachineID %s rebooking for SID %d originally assigned to MachineID %s", rebookRequest.MachineID, rebookRequest.SID, queueItem.MachineID)
		}
	default:
		SvcErrorReturn(w, fmt.Errorf("handleBook: invalid command"))
		return
	}

	//-----------------------------------------------------------------------------
	// FIND THE CONFIG FILE FOR THIS JOB
	//-----------------------------------------------------------------------------
	configDir := filepath.Join(app.QdConfigsDir, fmt.Sprintf("%d", queueItem.SID))
	configFilename, err := findConfigFile(configDir)
	if err != nil {
		SvcErrorReturn(w, fmt.Errorf("handleBook: error finding config file: %s", err.Error()))
		return
	}

	//-----------------------------------------------------------------------------
	// Create the MULTIPART response
	//-----------------------------------------------------------------------------
	response := BookedResponse{
		Status:         "success",
		Message:        "simulation booked",
		SID:            queueItem.SID,
		ConfigFilename: filepath.Base(configFilename),
	}
	multipartWriter := multipart.NewWriter(w)
	w.Header().Set("Content-Type", multipartWriter.FormDataContentType())

	//-------------------------
	// JSON part
	//-------------------------
	jsonWriter, err := multipartWriter.CreateFormField("json")
	if err != nil {
		SvcErrorReturn(w, fmt.Errorf("handleBook: failed to create JSON part"))
		return
	}
	if err := json.NewEncoder(jsonWriter).Encode(response); err != nil {
		SvcErrorReturn(w, fmt.Errorf("handleBook: failed to encode JSON response"))
		return
	}

	//----------------------------
	// Config file part
	//----------------------------
	configFilePath := filepath.Join(app.QdConfigsDir, fmt.Sprintf("%d", queueItem.SID), response.ConfigFilename)
	configFile, err := os.Open(configFilePath)
	if err != nil {
		SvcErrorReturn(w, fmt.Errorf("handleBook: failed to open config file"))
		return
	}
	defer configFile.Close()

	fileWriter, err := multipartWriter.CreateFormFile("file", response.ConfigFilename)
	if err != nil {
		SvcErrorReturn(w, fmt.Errorf("handleBook: failed to create file part"))
		return
	}
	if _, err := io.Copy(fileWriter, configFile); err != nil {
		SvcErrorReturn(w, fmt.Errorf("handleBook: failed to send config file"))
		return
	}

	//----------------------------
	// Close the multipart writer
	//----------------------------
	if err := multipartWriter.Close(); err != nil {
		SvcErrorReturn(w, fmt.Errorf("handleBook: failed to close multipart writer"))
		return
	}

	//*****************************************************************************
	// ONLY MARK AS BOOKED IF WE GET THIS FAR
	//*****************************************************************************
	queueItem.State = data.StateBooked
	if d.cmd.Command == "Rebook" {
		queueItem.MachineID = rebookRequest.MachineID
	} else {
		queueItem.MachineID = bookingRequest.MachineID
	}
	if err := app.qm.UpdateItem(queueItem); err != nil {
		SvcErrorReturn(w, fmt.Errorf("handleBook: failed to update queue item"))
		return
	}
}

// handleNewSimulation handles the NewSimulation command
// It creates a new entry in the queue
// ---------------------------------------------------------------------------
func handleNewSimulation(w http.ResponseWriter, r *http.Request, d *HInfo) {
	log.Printf("dispatcher >>>> NewSimulation handler\n")

	//-----------------------------------------------------------
	// Unmarshal the command data into CreateQueueEntryRequest
	//-----------------------------------------------------------
	var req CreateQueueEntryRequest
	if err := json.Unmarshal(d.cmd.Data, &req); err != nil {
		SvcErrorReturn(w, fmt.Errorf("handleNewSimulation: failed to unmarshal request data"))
		return
	}

	//------------------------------
	// Get the file from the form
	//------------------------------
	file, _, err := r.FormFile("file")
	if err != nil {
		SvcErrorReturn(w, fmt.Errorf("handleNewSimulation: failed to get file from form"))
		return
	}
	defer file.Close()
	fileContent, err := io.ReadAll(file)
	if err != nil {
		SvcErrorReturn(w, fmt.Errorf("handleNewSimulation: failed to read file content"))
		return
	}

	//----------------------------------------------
	// Create the directory if it doesn't exist
	//----------------------------------------------
	err = os.MkdirAll(app.QdConfigsDir, os.ModePerm)
	if err != nil {
		LogAndErrorReturn(w, fmt.Errorf("handleNewSimulation: failed to create directory: %v", err))
		return
	}

	//----------------------------------------------
	// Create a new file in the qdconfigs directory
	//----------------------------------------------
	tempFile, err := os.CreateTemp(app.QdConfigsDir, "config-*.json5")
	if err != nil {
		LogAndErrorReturn(w, fmt.Errorf("handleNewSimulation: failed to create temp file in directory %s: %v", app.QdConfigsDir, err))
		return
	}
	defer tempFile.Close()
	if len(fileContent) == 0 {
		LogAndErrorReturn(w, fmt.Errorf("handleNewSimulation: no file content. 0-length file"))
		return
	}

	//----------------------------------------------
	// Write the file content to a TEMPORARY file
	//----------------------------------------------
	if _, err := tempFile.Write(fileContent); err != nil {
		LogAndErrorReturn(w, fmt.Errorf("handleNewSimulation: failed to write file content: %v", err))
		return
	}

	//----------------------------------------------
	// Insert the queue item
	//----------------------------------------------
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
		LogAndErrorReturn(w, fmt.Errorf("handleNewSimulation: failed to insert new item to database: %v", err))
		return
	}

	//----------------------------------------------
	// Make the new directory
	//----------------------------------------------
	fpath := filepath.Join(app.QdConfigsDir, fmt.Sprintf("%d", sid))
	err = os.MkdirAll(fpath, os.ModePerm)
	if err != nil {
		LogAndErrorReturn(w, fmt.Errorf("handleNewSimulation: failed to make directory %s: %v", fpath, err))
		return
	}

	//---------------------------------------------------------------------
	// Rename the file to include the queue item ID and original filename
	//---------------------------------------------------------------------
	newFilePath := filepath.Join(app.QdConfigsDir, fmt.Sprintf("%d", sid), req.OriginalFilename)
	if err := os.Rename(tempFile.Name(), newFilePath); err != nil {
		LogAndErrorReturn(w, fmt.Errorf("handleNewSimulation: failed to rename %s to %s: %v", tempFile.Name(), newFilePath, err))
		return
	}

	//--------------------
	// Send back SUCCESS
	//--------------------
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
	log.Printf("dispatcher >>>> GetActiveQueue handler\n")
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

// handleGetMachineQueue returns the queue of all incomplete items for the
// specified machine
// -----------------------------------------------------------------------------
func handleGetMachineQueue(w http.ResponseWriter, r *http.Request, d *HInfo) {
	log.Printf("dispatcher >>>> GetMachineQueue handler\n")
	//-----------------------------------------------------------
	// Unmarshal the command data into MachineQueueRequest
	//-----------------------------------------------------------
	var req MachineQueueRequest
	if err := json.Unmarshal(d.cmd.Data, &req); err != nil {
		SvcErrorReturn(w, fmt.Errorf("failed to unmarshal request data"))
		return
	}
	items, err := app.qm.GetIncompleteItemsByMachineID(req.MachineID)
	if err != nil {
		SvcErrorReturn(w, fmt.Errorf("failed to get incomplete queue items for machine %s, error: %v", req.MachineID, err))
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

// handleGetCompletedQueue handles the GetCompletedQueue command
// -----------------------------------------------------------------------------
func handleGetCompletedQueue(w http.ResponseWriter, r *http.Request, d *HInfo) {
	log.Printf("dispatcher >>>> GetCompletedQueue handler\n")
	items, err := app.qm.GetCompletedItems()
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
	log.Printf("dispatcher >>>> UpdateItem handler\n")
	z := string(rune(0x2026)) // the '...' character

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
		SvcErrorReturn(w, fmt.Errorf("handleUpdateItem: invalid request data"))
		return
	}

	//--------------------------------------------------------
	// load the existing item to establish the base values
	//--------------------------------------------------------
	queueItem, err := app.qm.GetItemByID(req.SID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			SvcErrorReturn(w, fmt.Errorf("handleUpdateItem: queue item %d not found", req.SID))
		} else {
			SvcErrorReturn(w, fmt.Errorf("handleUpdateItem: error in GetItemByID: %v", err))
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
			SvcErrorReturn(w, fmt.Errorf("handleUpdateItem: invalid date: %s", req.DtEstimate))
			return
		}
		queueItem.DtEstimate.Time = dt
		queueItem.DtEstimate.Valid = true
		queueItem.State = data.StateExecuting
	}
	if req.DtCompleted != z && len(req.DtCompleted) > 0 {
		dt, err := util.StringToDate(req.DtCompleted)
		if err != nil {
			SvcErrorReturn(w, fmt.Errorf("handleUpdateItem: invalid date: %s", req.DtCompleted))
			return
		}
		queueItem.DtCompleted.Time = dt
		queueItem.DtCompleted.Valid = true
		queueItem.State = data.StateCompleted
	}

	if err := app.qm.UpdateItem(queueItem); err != nil {
		SvcErrorReturn(w, fmt.Errorf("handleUpdateItem: failed to update queue item"))
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
func handleDeleteItem(w http.ResponseWriter, r *http.Request, d *HInfo) {
	log.Printf("dispatcher >>>> DeleteItem handler\n")
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
	dirPath := filepath.Join(app.QdConfigsDir, fmt.Sprintf("%d", req.SID))
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

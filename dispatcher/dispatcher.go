package main

import (
	"encoding/json"
	"log"
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
func commandDispatcher(w http.ResponseWriter, r *http.Request) {
	var cmd Command
	var ok bool
	h := HandlerTableEntry{}

	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	log.Printf("\tcommand: %s", cmd.Command)

	if h, ok = handlerTable[cmd.Command]; !ok {
		log.Printf("\tUnknown command: %s", cmd.Command)
		http.Error(w, "Unknown command", http.StatusBadRequest)
	}

	h.Handler(w, r, &cmd)
}

// handleNewSimulation handles the NewSimulation command
func handleNewSimulation(w http.ResponseWriter, r *http.Request, cmd *Command) {
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
	//--------------------------------------------------------------
	// I know this is not a 201 for the return, it's a 200, but
	// we'll return the ID anyway.
	//--------------------------------------------------------------
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
	//--------------------------------------------------------------
	// I know this is not a 201 for the return, it's a 200, but
	// we'll return the ID anyway.
	//--------------------------------------------------------------
	msg := SvcStatus201{
		Status:  "success",
		Message: "deleted",
		ID:      int(req.SID),
	}
	SvcWriteResponse(w, &msg)
}

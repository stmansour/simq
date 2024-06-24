package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stmansour/simq/data"
	"github.com/stmansour/simq/util"
)

func initTest(t *testing.T) (*data.QueueManager, error) {
	ex, err := util.ReadExternalResources()
	if err != nil {
		t.Errorf("Failed to read external resources: %v", err)
		return nil, err
	}
	cmd := ex.GetSQLOpenString("simq")

	// Initialize the queue manager
	qm, err := data.NewQueueManager(cmd)
	if err != nil {
		t.Errorf("Failed to initialize queue manager: %v", err)
		return nil, err
	}
	if err := qm.RemoveSchemaForTesting(); err != nil {
		return nil, err
	}
	if err := qm.EnsureSchemaExists(); err != nil {
		return nil, err
	}
	return qm, nil
}

func TestCommandDispatcher(t *testing.T) {
	qm, err := initTest(t)
	if err != nil {
		t.Fatalf("Failed to initialize test: %v", err)
	}
	app.qm = qm

	// Test data for creating a queue entry
	createReq := CreateQueueEntryRequest{
		File:        "placeholder",
		Name:        "Test Simulation",
		Priority:    5,
		Description: "A test simulation",
		URL:         "http://localhost:8080",
	}
	reqData, _ := json.Marshal(createReq)

	cmd := Command{
		Command:  "NewSimulation",
		Username: "test-user",
		Data:     reqData,
	}
	cmdData, _ := json.Marshal(cmd)

	// Create a new HTTP request
	req, err := http.NewRequest("POST", "/command", bytes.NewBuffer(cmdData))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(commandDispatcher)

	// Call the handler
	handler.ServeHTTP(rr, req)

	// Check the status code
	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusCreated)
	}

	// Check the response body
	expected := "Created queue item with SID 1" // Adjust this based on the actual SID
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}

func TestHandleShutdown(t *testing.T) {
	qm, err := initTest(t)
	if err != nil {
		t.Fatalf("Failed to initialize test: %v", err)
	}
	app.qm = qm

	cmd := Command{
		Command:  "Shutdown",
		Username: "test-user",
	}
	cmdData, _ := json.Marshal(cmd)

	// Create a new HTTP request
	req, err := http.NewRequest("POST", "/command", bytes.NewBuffer(cmdData))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(commandDispatcher)

	// Call the handler
	handler.ServeHTTP(rr, req)

	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check the response body
	expected := "Server is shutting down"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}

func TestHandleGetActiveQueue(t *testing.T) {
	qm, err := initTest(t)
	if err != nil {
		return
	}
	app.qm = qm

	// Insert test items into the queue
	items := []data.QueueItem{
		{File: "file1", Name: "Simulation 1", Priority: 1, Description: "Test Simulation 1", URL: "http://localhost:8080", State: data.StateExecuting, DtEstimate: sql.NullTime{Time: time.Now().Add(10 * time.Hour), Valid: true}},
		{File: "file2", Name: "Simulation 2", Priority: 3, Description: "Test Simulation 2", URL: "http://localhost:8080", State: data.StateExecuting, DtEstimate: sql.NullTime{Time: time.Now().Add(8 * time.Hour), Valid: true}},
		{File: "file3", Name: "Simulation 3", Priority: 2, Description: "Test Simulation 3", URL: "http://localhost:8080", State: data.StateQueued},
		{File: "file4", Name: "Simulation 4", Priority: 5, Description: "Test Simulation 4", URL: "http://localhost:8080", State: data.StateBooked},
		{File: "file5", Name: "Simulation 5", Priority: 4, Description: "Test Simulation 5", URL: "http://localhost:8080", State: data.StateExecuting},
		{File: "file6", Name: "Simulation 6", Priority: 1, Description: "Test Simulation 6", URL: "http://localhost:8080", State: data.StateQueued},
		{File: "file7", Name: "Simulation 7", Priority: 2, Description: "Test Simulation 7", URL: "http://localhost:8080", State: data.StateBooked},
		{File: "file8", Name: "Simulation 8", Priority: 3, Description: "Test Simulation 8", URL: "http://localhost:8080", State: data.StateExecuting, DtEstimate: sql.NullTime{Time: time.Now().Add(12 * time.Hour), Valid: true}},
		{File: "file9", Name: "Simulation 9", Priority: 5, Description: "Test Simulation 9", URL: "http://localhost:8080", State: data.StateQueued},
		{File: "file10", Name: "Simulation 10", Priority: 4, Description: "Test Simulation 10", URL: "http://localhost:8080", State: data.StateBooked},
	}

	for _, item := range items {
		_, err := qm.InsertItem(item)
		if err != nil {
			t.Errorf("Failed to insert item: %v", err)
			return
		}
	}

	// Create a request for the GetActiveQueue command
	cmd := Command{Command: "GetActiveQueue", Username: "test-user"}
	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("Failed to marshal command: %v", err)
	}

	req, err := http.NewRequest("POST", "/command", bytes.NewBuffer(cmdBytes))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(commandDispatcher)
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check the response body is what we expect.
	expectedOrder := []int{2, 1, 8, 5, 6, 3, 7, 10, 4, 9}
	var queueItems []data.QueueItem
	if err := json.NewDecoder(rr.Body).Decode(&queueItems); err != nil {
		t.Fatalf("Failed to decode response body: %v", err)
	}

	for i, item := range queueItems {
		if item.SID != expectedOrder[i] {
			t.Errorf("Item order mismatch at position %d: got %d want %d", i, item.SID, expectedOrder[i])
		}
	}
}

package main

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestHandleNewSimulation(t *testing.T) {
	qm, err := initTest(t)
	if err != nil {
		t.Fatalf("Failed to initialize test: %v", err)
	}
	app.qm = qm

	// Read the config.json5 file
	fileContent, err := os.ReadFile("config.json5")
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	// Encode the file content as base64
	encodedContent := base64.StdEncoding.EncodeToString(fileContent)

	createReq := CreateQueueEntryRequest{
		FileContent:      encodedContent,
		OriginalFilename: "config.json5",
		Name:             "Test Simulation",
		Priority:         5,
		Description:      "A test simulation",
		URL:              "http://localhost:8080",
	}
	reqData, err := json.Marshal(createReq)
	if err != nil {
		t.Fatalf("Failed to marshal create request: %v", err)
	}

	// Create a new multipart form request
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	// Add the command field
	if err := w.WriteField("command", "NewSimulation"); err != nil {
		t.Fatalf("Failed to write command field: %v", err)
	}

	// Add the username field
	if err := w.WriteField("username", "test-user"); err != nil {
		t.Fatalf("Failed to write username field: %v", err)
	}

	// Add the data field
	if err := w.WriteField("data", string(reqData)); err != nil {
		t.Fatalf("Failed to write data field: %v", err)
	}

	// Add the file field
	fw, err := w.CreateFormFile("file", "config.json5")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	if _, err := fw.Write(fileContent); err != nil {
		t.Fatalf("Failed to write file content: %v", err)
	}
	w.Close()

	// Create a new HTTP request with the multipart form data
	req, err := http.NewRequest("POST", "/command", &b)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(commandDispatcher)

	// Call the handler
	handler.ServeHTTP(rr, req)

	// Check the status code
	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusCreated)
	}

	var resp SvcStatus201
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}
	if resp.Status != "success" {
		t.Errorf("handler returned unexpected status: got %q want %q", resp.Status, "success")
	}

	expectedFile := fmt.Sprintf("qdconfigs/%d/config.json5", resp.ID)
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("Expected file %s to be saved, but it does not exist", expectedFile)
	} else if err != nil {
		t.Errorf("Error checking for expected file: %v", err)
	}

	// Clean up the created file
	if err := os.Remove(expectedFile); err != nil {
		t.Errorf("Failed to clean up test file: %v", err)
	}
}

// func TestCommandDispatcher(t *testing.T) {
// 	qm, err := initTest(t)
// 	if err != nil {
// 		t.Fatalf("Failed to initialize test: %v", err)
// 	}
// 	app.qm = qm

// 	// Test data for creating a queue entry
// 	createReq := CreateQueueEntryRequest{
// 		Name:        "Test Simulation",
// 		Priority:    5,
// 		Description: "A test simulation",
// 		URL:         "http://localhost:8080",
// 	}
// 	reqData, _ := json.Marshal(createReq)

// 	cmd := Command{
// 		Command:  "NewSimulation",
// 		Username: "test-user",
// 		Data:     reqData,
// 	}
// 	cmdData, _ := json.Marshal(cmd)

// 	// Create a new HTTP request
// 	req, err := http.NewRequest("POST", "/command", bytes.NewBuffer(cmdData))
// 	if err != nil {
// 		t.Fatalf("Failed to create request: %v", err)
// 	}
// 	req.Header.Set("Content-Type", "application/json")

// 	// Create a ResponseRecorder to record the response
// 	rr := httptest.NewRecorder()
// 	handler := http.HandlerFunc(commandDispatcher)

// 	// Call the handler
// 	handler.ServeHTTP(rr, req)

// 	// Check the status code
// 	if status := rr.Code; status != http.StatusCreated {
// 		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusCreated)
// 	}

// 	// Check the response body
// 	var resp SvcStatus201
// 	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
// 		t.Errorf("Failed to unmarshal response: %v", err)
// 	}
// 	if resp.Status != "success" || resp.ID != 1 {
// 		t.Errorf("handler returned issues: status = %q want %q", resp.Status, "success")
// 	}
// }

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

	var resp SvcStatus200
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}
	if resp.Status != "success" {
		t.Errorf("handler returned issues: status = %q want %q", resp.Status, "success")
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

	var resp struct {
		Status string
		Data   []data.QueueItem
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}
	if resp.Status != "success" {
		t.Errorf("handler returned issues: status = %q want %q", resp.Status, "success")
	}

	// Check the response body is what we expect.
	expectedOrder := []int64{2, 1, 8, 5, 6, 3, 7, 10, 4, 9}
	for i, item := range resp.Data {
		if item.SID != expectedOrder[i] {
			t.Errorf("Item order mismatch at position %d: got %d want %d", i, item.SID, expectedOrder[i])
		}
	}
}

func mustMarshal(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal data: %v", err))
	}
	return data
}
func TestHandleUpdateItem(t *testing.T) {
	qm, err := initTest(t)
	if err != nil {
		return
	}
	app.qm = qm

	// Insert a new item
	newItem := data.QueueItem{
		File:        "/path/to/simulation.tar.gz",
		Name:        "Test Simulation",
		Priority:    5,
		Description: "A test simulation",
		URL:         "http://localhost:8080",
		State:       data.StateQueued,
	}
	sid, err := qm.InsertItem(newItem)
	if err != nil {
		t.Fatalf("Failed to insert item: %v", err)
	}

	// Create the update request
	updateRequest := UpdateItemRequest{
		SID:         int(sid),
		Priority:    10,
		Description: "Updated description",
	}
	updateCommand := Command{
		Command:  "UpdateItem",
		Username: "testuser",
		Data:     json.RawMessage(mustMarshal(updateRequest)),
	}

	// Send the update command
	reqBody, _ := json.Marshal(updateCommand)
	req, err := http.NewRequest("POST", "/command", bytes.NewBuffer(reqBody))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(commandDispatcher)
	handler.ServeHTTP(rr, req)

	// Check the response
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
	var resp SvcStatus201
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}
	if resp.Status != "success" || resp.ID != int(sid) {
		t.Errorf("handler returned issues: status = %q, id = %d want %q, %d", resp.Status, resp.ID, "success", sid)
	}

	// Verify the item was updated
	updatedItem, err := qm.GetItemByID(int(sid))
	if err != nil {
		t.Fatalf("Failed to get item: %v", err)
	}
	if updatedItem.Priority != updateRequest.Priority {
		t.Errorf("Priority not updated: got %v want %v", updatedItem.Priority, updateRequest.Priority)
	}
	if updatedItem.Description != updateRequest.Description {
		t.Errorf("Description not updated: got %v want %v", updatedItem.Description, updateRequest.Description)
	}
}

func TestHandleDeleteItem(t *testing.T) {
	qm, err := initTest(t)
	if err != nil {
		return
	}
	app.qm = qm

	// Insert a new item
	newItem := data.QueueItem{
		File:        "/path/to/simulation.tar.gz",
		Name:        "Test Simulation",
		Priority:    5,
		Description: "A test simulation",
		URL:         "http://localhost:8080",
		State:       data.StateQueued,
	}
	sid, err := qm.InsertItem(newItem)
	if err != nil {
		t.Fatalf("Failed to insert item: %v", err)
	}

	// Create the delete request
	deleteRequest := DeleteItemRequest{
		SID: int(sid),
	}
	deleteCommand := Command{
		Command:  "DeleteItem",
		Username: "testuser",
		Data:     json.RawMessage(mustMarshal(deleteRequest)),
	}

	// Send the delete command
	reqBody, _ := json.Marshal(deleteCommand)
	req, err := http.NewRequest("POST", "/command", bytes.NewBuffer(reqBody))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(commandDispatcher)
	handler.ServeHTTP(rr, req)

	// Check the response
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var resp SvcStatus201
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}
	if resp.Status != "success" || resp.ID != int(sid) {
		t.Errorf("handler returned issues: status = %q, id = %d want %q, %d", resp.Status, resp.ID, "success", sid)
	}

	// Verify the item was deleted
	_, err = qm.GetItemByID(int(sid))
	if err == nil {
		t.Errorf("Expected item to be deleted, but it still exists")
	}
}

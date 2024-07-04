package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stmansour/simq/data"
	"github.com/stmansour/simq/util"
	"github.com/stretchr/testify/assert"
)

func TestVersion(t *testing.T) {
	app.version = true
	doMain()
}

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

// TestHandleNewSimulation tests the handleNewSimulation function
func TestHandleNewSimulation(t *testing.T) {
	qm, err := initTest(t)
	if err != nil {
		t.Fatalf("Failed to initialize test: %v", err)
	}
	app.qm = qm
	generateNewSimulation(t)
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

	var resp SvcStatus200
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}
	if resp.Status != "success" {
		t.Errorf("handler returned issues: status = %q want %q", resp.Status, "success")
	}
}

func TestHandleGetActiveQueue(t *testing.T) {
	//------------------
	// INITIALIZE TEST
	//------------------
	qm, err := initTest(t)
	assert.NoError(t, err)
	app.qm = qm

	//----------------------------------
	// PUT SOME ITEMS IN THE QUEUE
	//----------------------------------
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

	//-------------------------------------
	// MARSHAL CMD BYTES...
	//-------------------------------------
	cmd := Command{Command: "GetActiveQueue", Username: "test-user"}
	bookData, err := json.Marshal(cmd)
	assert.NoError(t, err)

	//-------------------------------------
	// CREATE HTTP REQUEST
	//-------------------------------------
	req, err := http.NewRequest("POST", "/command", bytes.NewBuffer(bookData))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	//-------------------------------------
	// SEND REQUEST AND RECEIVE RESPONSE
	//-------------------------------------
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(commandDispatcher)
	handler.ServeHTTP(rr, req)
	cmdResp := rr.Result()
	defer cmdResp.Body.Close()
	bodyBytes, err := io.ReadAll(cmdResp.Body)
	assert.NoError(t, err)
	assert.EqualValues(t, cmdResp.StatusCode, http.StatusOK)

	//-------------------------------------
	// EXAMINE THE RESPONSE...
	//-------------------------------------
	var resp struct {
		Status string
		Data   []data.QueueItem
	}
	err = json.Unmarshal(bodyBytes, &resp)
	assert.NoError(t, err)
	assert.Equal(t, "success", resp.Status)

	//-------------------------------------
	// VERIFY THE ORDER OF THE ITEMS
	//-------------------------------------
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

// TestHandleDeleteItem tests the handleDeleteItem function
func TestHandleDeleteItem(t *testing.T) {
	var err error
	//------------------
	// INITIALIZE TEST
	//------------------
	app.qm, err = initTest(t)
	assert.NoError(t, err)

	//----------------------------------
	// INSERT QUEUE ITEM TO DELETE
	//----------------------------------
	newItem := data.QueueItem{
		File:        "dummyfile.tar.gz",
		Name:        "Test Simulation",
		Priority:    5,
		Description: "A test simulation",
		URL:         "http://localhost:8080",
		State:       data.StateQueued,
	}
	sid, err := app.qm.InsertItem(newItem)
	if err != nil {
		t.Fatalf("Failed to insert item: %v", err)
	}

	//----------------------------------------
	// CREATE CONFIG FILE FOR THE NEW ITEM
	//----------------------------------------
	dirPath := fmt.Sprintf("qdconfigs/%d", sid)
	err = os.MkdirAll(dirPath, os.ModePerm)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	filePath := fmt.Sprintf("%s/config.json5", dirPath)
	_, err = os.Create(filePath)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	//-------------------------------------
	// CREATE REQUEST DATA
	//-------------------------------------
	deleteRequest := DeleteItemRequest{
		SID: sid,
	}
	cmd := Command{
		Command:  "DeleteItem",
		Username: "testuser",
		Data:     json.RawMessage(mustMarshal(deleteRequest)),
	}
	//-------------------------------------
	// MARSHAL REQUEST BYTES...
	//-------------------------------------
	assert.NoError(t, err)
	dataBytes, err := json.Marshal(deleteRequest)
	assert.NoError(t, err)
	cmd.Data = json.RawMessage(dataBytes)
	bookData, err := json.Marshal(cmd)
	assert.NoError(t, err)

	//-------------------------------------
	// CREATE HTTP REQUEST
	//-------------------------------------
	req, err := http.NewRequest("POST", "/command", bytes.NewBuffer(bookData))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(commandDispatcher)
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	//-------------------------------------
	// VERIFY THE RESPONSE
	//-------------------------------------
	var resp SvcStatus201
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "success", resp.Status)
	assert.Equal(t, resp.ID, sid)

	//-----------------------------------------
	// VERIFY THE ITEM IS NO LONGER IN THE DB
	//-----------------------------------------
	_, err = app.qm.GetItemByID(sid)
	if err == nil {
		t.Errorf("Expected item to be deleted, but it still exists")
	}

	//---------------------------------------------
	// VERIFY THE DIRECTORY AND FILE WERE DELETED
	//---------------------------------------------
	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		t.Errorf("Expected directory %s to be deleted, but it still exists", dirPath)
	}
}

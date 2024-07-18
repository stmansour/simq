package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stmansour/simq/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestRequest(t *testing.T, data map[string]interface{}) (*httptest.ResponseRecorder, *http.Request) {
	//-------------------------------------------------------------
	// INNER COMMAND DATA -- specifies the data for the command
	//-------------------------------------------------------------
	dataBytes, err := json.Marshal(data)
	require.NoError(t, err)

	//-------------------------------------------------------------
	// OUTER COMMAND SHELL -- specifies the command and user
	//-------------------------------------------------------------
	cmd := Command{
		Command:  "UpdateItem",
		Username: "test-user",
	}
	cmd.Data = json.RawMessage(dataBytes) // RawMessage --> json.Marshal will not re-encode the data
	jsonData, err := json.Marshal(cmd)
	require.NoError(t, err)

	//-------------------------------------------------------------
	// jsonData is now ready to be sent
	//-------------------------------------------------------------
	req, err := http.NewRequest("POST", "/command", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	//-------------------------------------------------------------
	// Create a ResponseRecorder to record the response, then
	// get the response...
	//-------------------------------------------------------------
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(commandDispatcher)
	handler.ServeHTTP(rr, req)

	return rr, req
}

func TestHandleUpdateItem(t *testing.T) {
	//------------------------------
	// INITIALIZE ALL...
	//------------------------------
	qm, err := initTest(t)
	if err != nil {
		t.Fatalf("Failed to initialize test: %v", err)
	}
	app.qm = qm
	app.HTTPHdrsDbg = true
	app.HexASCIIDbg = true

	//--------------------------------------
	// CREATE A NEW ITEM IN THE DATABASE
	//--------------------------------------
	baseItem := data.QueueItem{
		File:        "test.file",
		Username:    "testuser",
		Name:        "Test Item",
		Priority:    1,
		Description: "Test Description",
		MachineID:   "machine1",
		URL:         "http://test.com",
		State:       0,
		DtEstimate:  sql.NullTime{Time: time.Now().Add(24 * time.Hour), Valid: true},
		DtCompleted: sql.NullTime{Valid: false},
	}

	sid, err := app.qm.InsertItem(baseItem)
	require.NoError(t, err)
	baseItem.SID = sid

	defer func() {
		err := app.qm.DeleteItem(sid)
		assert.NoError(t, err)
	}()

	//===================================================================
	// BEGIN THE TESTS...
	// First test: Update all fields
	//===================================================================
	t.Run("UpdateAllFields", func(t *testing.T) {
		//--------------------------------------------------
		// PREPARE THE REQUEST DATA
		//--------------------------------------------------
		data := map[string]interface{}{
			"SID":         sid,
			"Priority":    2,
			"Description": "Updated Description",
			"MachineID":   "machine2",
			"URL":         "http://updated.com",
			"DtEstimate":  time.Now().Add(48 * time.Hour).Format(time.RFC3339),
			"DtCompleted": time.Now().Format(time.RFC3339),
		}
		rr, req := createTestRequest(t, data)

		//--------------------------------------------------
		// SEND THE REQUEST AND VERIFY THE RESPONSE
		//--------------------------------------------------
		bodyBytes, err := io.ReadAll(rr.Body)
		require.NoError(t, err)
		req.Body.Close()
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		assert.Equal(t, http.StatusOK, rr.Code)
		var response SvcStatus201
		err = json.Unmarshal(bodyBytes, &response)
		assert.NoError(t, err)
		assert.Equal(t, "success", response.Status)
		assert.Equal(t, "Updated", response.Message)
		assert.Equal(t, sid, response.ID)

		//--------------------------------------------------
		// VERIFY THE DATABASE WAS CORRECTLY UPDATED
		//--------------------------------------------------
		updatedItem, err := app.qm.GetItemByID(sid)
		assert.NoError(t, err)
		assert.Equal(t, 2, updatedItem.Priority)
		assert.Equal(t, "Updated Description", updatedItem.Description)
		assert.Equal(t, "machine2", updatedItem.MachineID)
		assert.Equal(t, "http://updated.com", updatedItem.URL)
	})

	//===================================================================
	// Next test: Update only priority and URL
	//===================================================================
	t.Run("UpdatePartialFields", func(t *testing.T) {
		data := map[string]interface{}{
			"SID":      sid,
			"Priority": 3,
			"URL":      "http://partial-update.com",
		}

		rr, req := createTestRequest(t, data)
		bodyBytes, err := io.ReadAll(rr.Body)
		require.NoError(t, err)
		req.Body.Close()
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		assert.Equal(t, http.StatusOK, rr.Code)
		var response SvcStatus201
		err = json.Unmarshal(bodyBytes, &response)
		assert.NoError(t, err)
		assert.Equal(t, "success", response.Status)
		assert.Equal(t, "Updated", response.Message)
		assert.Equal(t, sid, response.ID)

		//--------------------------------------------
		// VERIFY THE DATABSE WAS CORRECTLY UPDATED
		//--------------------------------------------
		updatedItem, err := app.qm.GetItemByID(sid)
		assert.NoError(t, err)
		assert.Equal(t, 3, updatedItem.Priority)
		assert.Equal(t, "http://partial-update.com", updatedItem.URL)
		assert.Equal(t, "Updated Description", updatedItem.Description) // Should not have changed
	})

	t.Run("ItemNotFound", func(t *testing.T) {
		data := map[string]interface{}{
			"SID":      999999, // Assuming this SID doesn't exist
			"Priority": 1,
		}

		rr, req := createTestRequest(t, data)

		bodyBytes, err := io.ReadAll(rr.Body)
		require.NoError(t, err)
		req.Body.Close()
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		var response SvcStatus201
		err = json.Unmarshal(bodyBytes, &response)
		assert.NoError(t, err)
		assert.Equal(t, response.Status, "error")
		assert.Equal(t, response.Message, "handleUpdateItem: queue item 999999 not found")
	})

	t.Run("InvalidDate", func(t *testing.T) {
		data := map[string]interface{}{
			"SID":        sid,
			"DtEstimate": "invalid-date",
		}

		rr, req := createTestRequest(t, data)

		bodyBytes, err := io.ReadAll(rr.Body)
		require.NoError(t, err)
		req.Body.Close()
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		var response SvcStatus201
		err = json.Unmarshal(bodyBytes, &response)
		assert.NoError(t, err)
		assert.Equal(t, response.Status, "error")
		assert.Equal(t, response.Message, "handleUpdateItem: invalid date: invalid-date")
	})
}

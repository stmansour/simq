package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stmansour/simq/data"
	"github.com/stretchr/testify/assert"
)

// Mock implementation of util.GetMachineUUID
func GetMachineUUID() (string, error) {
	return "mock-machine-id", nil
}

// Mock implementation of app.cfg.DispatcherURL
var DispatcherURL = "http://localhost:8080/"

// Mock implementation of app.HTTPHdrsDbg
var HTTPHdrsDbg = false

// generateNewSimulation puts a new simulation in the database
// -----------------------------------------------------------------------------
func generateNewSimulation(t *testing.T) {
	cmd := Command{
		Command:  "NewSimulation",
		Username: "simd",
	}
	createReq := CreateQueueEntryRequest{
		OriginalFilename: "config.json5",
		Name:             "Test Simulation",
		Priority:         5,
		Description:      "A test simulation",
		URL:              "http://localhost:8090",
	}
	//------------------------------------
	// START WITH A NEW MULTIPART WRITER
	//------------------------------------
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	//-------------------------------------
	// CREATE THE JSON PART
	//-------------------------------------
	dataBytes, err := json.Marshal(createReq)
	assert.NoError(t, err)
	cmd.Data = json.RawMessage(dataBytes)
	jsonData, err := json.Marshal(cmd)
	assert.NoError(t, err)
	err = writer.WriteField("data", string(jsonData))
	assert.NoError(t, err)

	//------------------------------------
	// CREATE THE FILE PART
	//------------------------------------
	filename := "config.json5"
	file, err := os.Open(filename)
	assert.NoError(t, err)
	defer file.Close()
	filePart, err := writer.CreateFormFile("file", filename)
	assert.NoError(t, err)
	_, err = io.Copy(filePart, file)
	assert.NoError(t, err)

	//------------------------------------
	// CLOSE THE MULTIPART WRITER
	//------------------------------------
	err = writer.Close()
	assert.NoError(t, err)

	//--------------------------------------------
	// SEND THE REQUEST AND RECEIVE THE RESPONSE
	//--------------------------------------------
	req, err := http.NewRequest("POST", "http://localhost:8080/command", body)
	assert.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	commandDispatcher(rr, req) // this is the shortcut way, call the handler directly
	resp := rr.Result()
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.EqualValues(t, resp.StatusCode, http.StatusCreated)

	//---------------------------
	// VALIDATE RESPONSE...
	//---------------------------
	var statResp SvcStatus201
	err = json.Unmarshal(bodyBytes, &statResp)
	assert.NoError(t, err)
	assert.Equal(t, "success", statResp.Status)

	//---------------------------
	// VALIDATE FILE CREATED...
	//---------------------------
	expectedFile := fmt.Sprintf("qdconfigs/%d/config.json5", statResp.ID)
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("Expected file %s to be saved, but it does not exist", expectedFile)
	} else if err != nil {
		t.Errorf("Error checking for expected file: %v", err)
	}
}

// TestBookCommand tests the Book command
// -----------------------------------------------------------------------------
func TestBookCommand(t *testing.T) {
	var err error
	app.qm, err = initTest(t)
	if err != nil {
		t.Fatalf("Failed to initialize test: %v", err)
	}

	//------------------------------------
	// FIRST, CREATE A NEW SIMULATION
	//------------------------------------
	generateNewSimulation(t)

	//------------------------------------
	// CREATE STRUCTS FOR REQUEST
	//------------------------------------
	cmd := Command{
		Command:  "Book",
		Username: "simd",
	}

	cmdDataStruct := struct {
		MachineID       string
		CPUs            int
		Memory          string
		CPUArchitecture string
		Availability    string
	}{
		CPUs:            10,
		Memory:          "64GB",
		CPUArchitecture: "ARM64",
		Availability:    "always",
	}

	//-------------------------------------
	// MARSHAL CMD BYTES...
	//-------------------------------------
	cmdDataStruct.MachineID, err = GetMachineUUID()
	assert.NoError(t, err)
	dataBytes, err := json.Marshal(cmdDataStruct)
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

	//-------------------------------------
	// SEND REQUEST AND RECEIVE RESPONSE
	//-------------------------------------
	rr := httptest.NewRecorder()
	commandDispatcher(rr, req)
	resp := rr.Result()
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.EqualValues(t, resp.StatusCode, http.StatusOK)

	//---------------------------------------------------------
	// MULTIPART RESPONSE EXPECTED OR SINGLE PART FOR ERROR
	//---------------------------------------------------------
	var bookResp BookedResponse
	contentType := resp.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/") {
		//------------
		// MULTIPART
		//------------
		boundary := strings.Split(contentType, "boundary=")[1]
		multipartReader := multipart.NewReader(bytes.NewReader(bodyBytes), boundary)
		for {
			part, err := multipartReader.NextPart()
			if err == io.EOF {
				break
			}
			assert.NoError(t, err)

			switch part.FormName() {
			case "json":
				err := json.NewDecoder(part).Decode(&bookResp)
				assert.NoError(t, err)
			case "file":
				configDir := fmt.Sprintf("simulations/%d", bookResp.SID)
				os.MkdirAll(configDir, os.ModePerm)
				configPath := fmt.Sprintf("%s/%s", configDir, bookResp.ConfigFilename)
				out, err := os.Create(configPath)
				assert.NoError(t, err)
				defer out.Close()
				_, err = io.Copy(out, part)
				assert.NoError(t, err)
				defer os.Remove(configPath) // Clean up file after test
			}
		}
	} else if strings.HasPrefix(contentType, "application/json") {
		//---------------
		// SINGLE PART
		//---------------
		err := json.Unmarshal(bodyBytes, &bookResp)
		assert.NoError(t, err)
		if bookResp.Message == "no items in queue" {
			// This is an expected response
			return
		} else if bookResp.Status != "success" {
			t.Fatalf("failed to book simulation: %s", bookResp.Message)
		}
	}

	//---------------------------------------------
	// UPDATE ITEM STATE TO BOOKED
	//---------------------------------------------
	item, err := app.qm.GetItemByID(bookResp.SID)
	if err != nil {
		t.Fatalf("Failed to get item: %v", err)
	}
	item.State = data.StateBooked
	err = app.qm.UpdateItem(item)
	assert.NoError(t, err)
}

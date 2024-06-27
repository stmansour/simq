package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/stmansour/simq/data"
	"github.com/yosuke-furukawa/json5/encoding/json5"
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
	OriginalFilename string `json:"OriginalFilename"`
	Name             string `json:"name"`
	Priority         int    `json:"priority"`
	Description      string `json:"description"`
	URL              string `json:"url"`
}

// Config represents the structure of a config
type Config struct {
	SimulationName string `json:"SimulationName"`
	C1             string `json:"C1"`
	C2             string `json:"C2"`
}

const (
	defaultPriority = 5
	defaultURL      = "http://localhost:8250/command"
)

func addJob(username, file string) {
	config, err := readConfig(file)
	if err != nil {
		fmt.Printf("Error reading config file: %v\n", err)
		return
	}

	data := CreateQueueEntryRequest{
		OriginalFilename: filepath.Base(file),
		Name:             config.SimulationName,
		Priority:         defaultPriority,
		Description:      "",
		URL:              defaultURL,
	}

	dataBytes, _ := json.Marshal(data)
	cmd := Command{
		Command:  "NewSimulation",
		Username: username,
		Data:     json.RawMessage(dataBytes),
	}

	resp := sendMultipartRequest(cmd, file)
	if resp != nil {
		fmt.Printf("Add Job Response: %s\n", string(resp))
	}
}

func readConfig(file string) (Config, error) {
	var config Config
	configBytes, err := os.ReadFile(file)
	if err != nil {
		return config, err
	}

	err = json5.Unmarshal(configBytes, &config)
	return config, err
}

func listJobs(username string) {
	cmd := Command{
		Command:  "GetActiveQueue",
		Username: username,
	}

	body := sendRequest(cmd)

	resp := struct {
		Status string
		Data   []data.QueueItem
	}{}
	err := json.Unmarshal(body, &resp)
	if err != nil {
		fmt.Printf("Error unmarshalling response: %v\n", err)
		return
	}

	for _, item := range resp.Data {
		fmt.Printf("SID: %3d. Pri: %2d, St: %d, Name: %s, File: %s\n", item.SID, item.Priority, item.State, item.Name, item.File)
	}
}

func deleteJob(username string, sid int64) {
	data := struct {
		SID int64 `json:"sid"`
	}{
		SID: sid,
	}

	dataBytes, _ := json.Marshal(data)
	cmd := Command{
		Command:  "DeleteItem",
		Username: username,
		Data:     json.RawMessage(dataBytes),
	}

	resp := sendRequest(cmd)
	if resp != nil {
		fmt.Printf("Delete Job Response: %s\n", string(resp))
	}
}

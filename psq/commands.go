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
	Command  string
	Username string
	Data     json.RawMessage
}

// CreateQueueEntryRequest represents the data for creating a queue entry
type CreateQueueEntryRequest struct {
	FileContent      string
	OriginalFilename string
	Name             string
	Priority         int
	Description      string
	URL              string
}

// Config represents the structure of a config
type Config struct {
	SimulationName string
	C1             string
	C2             string
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
		URL:              "",
	}

	dataBytes, _ := json.Marshal(data)
	cmd := Command{
		Command:  "NewSimulation",
		Username: username,
		Data:     json.RawMessage(dataBytes),
	}

	resp, err := sendMultipartRequest(cmd, file)
	if err != nil {
		fmt.Printf("Error sending request: %v\n", err)
	}
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
	states := []string{"Qd", "Bk", "Ex", "Fn", "Er"}
	// 0         1         2         3         4         5         6
	// 01234567890123456789012345678901234567890123456789
	// 2024/05/11 HH:MM
	fmt.Printf("SID PRI ST %-15s %-15s %-16s %-40s %-25s \n", "Username", "File", "Estimate", "MachineID", "Name")
	for _, item := range resp.Data {
		nm := item.Name
		if len(nm) > 25 {
			nm = nm[:24] + string(rune(0x2026))
		}
		mid := item.MachineID
		if len(mid) > 40 {
			mid = mid[:34] + string(rune(0x2026))
		}
		estimate := ""
		if item.DtEstimate.Valid {
			estimate = item.DtEstimate.Time.Format("2006/01/02 15:04")
		}

		fmt.Printf("%3d %3d %2s %-15s %-15s %-16s %-40s %-15s\n", item.SID, item.Priority, states[item.State], item.Username, item.File, estimate, mid, nm)
	}
}

func deleteJob(username string, sid int64) {
	data := struct {
		SID int64
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

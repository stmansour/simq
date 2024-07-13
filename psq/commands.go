package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/stmansour/simq/data"
	"github.com/stmansour/simq/util"
	"github.com/yosuke-furukawa/json5/encoding/json5"
)

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

func addJob(cmd *CmdData, args []string) {
	file := args[0]
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
	command := util.Command{
		Command:  "NewSimulation",
		Username: cmd.Username,
		Data:     json.RawMessage(dataBytes),
	}

	resp, err := util.SendMultipartRequest(defaultURL, &command, file)
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

func listJobs(cmd *CmdData, args []string) {
	command := util.Command{
		Command:  "GetActiveQueue",
		Username: cmd.Username,
	}
	listCore(&command)
}

func listDoneJobs(cmd *CmdData, args []string) {
	command := util.Command{
		Command:  "GetCompletedQueue",
		Username: cmd.Username,
	}
	listCore(&command)
}

func listCore(command *util.Command) {
	body := util.SendRequest(app.DispatcherURL, command)

	resp := struct {
		Status string
		Data   []data.QueueItem
	}{}

	err := json.Unmarshal(body, &resp)
	if err != nil {
		if strings.Contains(err.Error(), "unexpected end of JSON input") {
			fmt.Printf("No jobs found\n")
			return
		}
		fmt.Printf("Error unmarshalling response: %v\n", err)
		return
	}
	states := []string{"Qd", "Bk", "Ex", "Fn", "Ar", "Er"}
	// 0         1         2         3         4         5         6
	// 01234567890123456789012345678901234567890123456789
	// 2024/05/11 HH:MM

	DtIsEstimate := command.Command == "GetActiveQueue"
	dt := ""
	DtCN := "Estimate"
	if !DtIsEstimate {
		DtCN = "Completed"
	}
	fmt.Printf("SID PRI ST %-15s %-15s %-20s %-40s %-25s \n", "Username", "File", DtCN, "MachineID", "Name")
	for _, item := range resp.Data {
		nm := item.Name
		if len(nm) > 25 {
			nm = nm[:24] + string(rune(0x2026))
		}
		mid := item.MachineID
		if len(mid) > 40 {
			mid = mid[:34] + string(rune(0x2026))
		}
		dt = ""
		if DtIsEstimate {
			if item.DtEstimate.Valid {
				dt = item.DtEstimate.Time.In(time.Local).Format("Jan 2, 2006 03:04pm")
			}
		} else {
			if item.DtCompleted.Valid {
				dt = item.DtCompleted.Time.In(time.Local).Format("Jan 2, 2006 03:04pm")
			}
		}

		fmt.Printf("%3d %3d %2s %-15s %-15s %-20s %-40s %-15s\n", item.SID, item.Priority, states[item.State], item.Username, item.File, dt, mid, nm)
	}
}

func deleteJob(cmd *CmdData, args []string) {
	sid, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		fmt.Println("Error: 'delete' command requires a valid simulation ID.")
		return
	}
	cmd.SID = sid

	data := struct {
		SID int64
	}{
		SID: cmd.SID,
	}

	dataBytes, _ := json.Marshal(data)
	command := util.Command{
		Command:  "DeleteItem",
		Username: cmd.Username,
		Data:     json.RawMessage(dataBytes),
	}

	resp := util.SendRequest(defaultURL, &command)
	if resp != nil {
		fmt.Printf("Delete Job Response: %s\n", string(resp))
	}
}

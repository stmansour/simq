package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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

// CmdGetSID represents the structure of a command
type CmdGetSID struct {
	SID int64
}

// Config represents the structure of a config
type Config struct {
	SimulationName string
	Username       string
	Data           []byte
}

const (
	defaultPriority = 5
)

// getSID reads the queue details for the specified simulation ID
// --------------------------------------------------------------------
func getSID(cmd *CmdData, args []string) {
	var err error
	var databt CmdGetSID
	databt.SID, err = strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		fmt.Printf("Error: invalid simulation ID: %s\n.", args[0])
		return
	}

	dataBytes, err := json.Marshal(databt)
	if err != nil {
		fmt.Printf("Error marshaling ID: %s\n.", err.Error())
		return
	}
	command := util.Command{
		Command:  "GetSID",
		Username: cmd.Username,
		Data:     json.RawMessage(dataBytes),
	}

	respBytes := util.SendRequest(app.DispatcherURL, &command)
	var resp struct {
		Status string
		Data   data.QueueItem
	}

	err = json.Unmarshal(respBytes, &resp)
	if err != nil {
		fmt.Printf("Error unmarshaling response: %s\n", err.Error())
		return
	}

	if resp.Status != "success" {
		fmt.Printf("Error: Response status: %s\n", resp.Status)
		return
	}

	fmt.Printf("SID: %d\n", resp.Data.SID)
	fmt.Printf("State: %d\n", resp.Data.State)
	fmt.Printf("Username: %s\n", resp.Data.Username)
	fmt.Printf("File: %s\n", resp.Data.File)
	fmt.Printf("Name: %s\n", resp.Data.Name)
	fmt.Printf("MachineID: %s\n", resp.Data.MachineID)
	fmt.Printf("Priority: %d\n", resp.Data.Priority)
	fmt.Printf("Description: %s\n", resp.Data.Description)
	fmt.Printf("URL: %s\n", resp.Data.URL)
	fmt.Printf("Created: %s\n", resp.Data.Created.In(time.Local).Format("Jan 2, 2006 03:04pm"))
	fmt.Printf("Modified: %s\n", resp.Data.Modified.In(time.Local).Format("Jan 2, 2006 03:04pm"))
	dt := ""
	if resp.Data.State < 3 {
		if resp.Data.DtEstimate.Valid {
			dt = "Estimated Completion: " + resp.Data.DtEstimate.Time.In(time.Local).Format("Jan 2, 2006 03:04pm")

		}
	} else {
		if resp.Data.DtCompleted.Valid {
			dt = "Completed: " + resp.Data.DtCompleted.Time.In(time.Local).Format("Jan 2, 2006 03:04pm")
		}
	}
	if len(dt) > 0 {
		fmt.Printf("%s\n", dt)
	}

}

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

	resp, err := util.SendMultipartRequest(app.DispatcherURL, &command, file)
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

// QueueItem is an item in the queue
type QueueItem struct {
	SID         int64
	File        string
	Username    string
	Name        string
	Priority    int
	Description string
	MachineID   string
	URL         string
	State       int
	DtEstimate  sql.NullTime
	DtCompleted sql.NullTime
	Created     time.Time
	Modified    time.Time
}

const (
	sidWidth       = 3
	priorityWidth  = 3
	stateWidth     = 2
	usernameWidth  = 15
	fileWidth      = 15
	dtWidth        = 20
	machineIDWidth = 40
	nameWidth      = 25
)

// listCore is the core function for listing jobs
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

	DtIsEstimate := command.Command == "GetActiveQueue"
	DtCN := "Estimate"
	if !DtIsEstimate {
		DtCN = "Completed"
	}

	// Define box-drawing characters
	topLeft, topRight, bottomLeft, bottomRight := "┌", "┐", "└", "┘"
	horizontal, vertical := "─", "│"
	leftT, rightT, topT, bottomT, cross := "├", "┤", "┬", "┴", "┼"

	// Print top border
	fmt.Print(topLeft)
	for i, width := range []int{sidWidth, priorityWidth, stateWidth, usernameWidth, fileWidth, dtWidth, machineIDWidth, nameWidth} {
		fmt.Print(strings.Repeat(horizontal, width))
		if i < 7 {
			fmt.Print(topT)
		}
	}
	fmt.Print(topRight + "\n")

	// Print header
	fmt.Printf("%s%-*s%s%-*s%s%-*s%s%-*s%s%-*s%s%-*s%s%-*s%s%-*s%s\n",
		vertical, sidWidth, "SID",
		vertical, priorityWidth, "PRI",
		vertical, stateWidth, "ST",
		vertical, usernameWidth, "Username",
		vertical, fileWidth, "File",
		vertical, dtWidth, DtCN,
		vertical, machineIDWidth, "MachineID",
		vertical, nameWidth, "Name",
		vertical)

	// Print separator
	fmt.Print(leftT)
	for i, width := range []int{sidWidth, priorityWidth, stateWidth, usernameWidth, fileWidth, dtWidth, machineIDWidth, nameWidth} {
		fmt.Print(strings.Repeat(horizontal, width))
		if i < 7 {
			fmt.Print(cross)
		}
	}
	fmt.Print(rightT + "\n")

	// Print data rows
	for _, item := range resp.Data {
		dt := ""
		if DtIsEstimate {
			if item.DtEstimate.Valid {
				dt = item.DtEstimate.Time.In(time.Local).Format("Jan 2, 2006 03:04pm")
			}
		} else {
			if item.DtCompleted.Valid {
				dt = item.DtCompleted.Time.In(time.Local).Format("Jan 2, 2006 03:04pm")
			}
		}

		fmt.Printf("%s%-*d%s%-*d%s%-*s%s%-*s%s%-*s%s%-*s%s%-*s%s%-*s%s\n",
			vertical, sidWidth, item.SID,
			vertical, priorityWidth, item.Priority,
			vertical, stateWidth, states[item.State],
			vertical, usernameWidth, truncateMiddle(item.Username, usernameWidth),
			vertical, fileWidth, truncateMiddle(item.File, fileWidth),
			vertical, dtWidth, dt,
			vertical, machineIDWidth, truncateMiddle(item.MachineID, machineIDWidth),
			vertical, nameWidth, truncateMiddle(item.Name, nameWidth),
			vertical)
	}

	fmt.Print(bottomLeft)
	for i, width := range []int{sidWidth, priorityWidth, stateWidth, usernameWidth, fileWidth, dtWidth, machineIDWidth, nameWidth} {
		fmt.Print(strings.Repeat(horizontal, width))
		if i < 7 {
			fmt.Print(bottomT)
		}
	}
	fmt.Print(bottomRight + "\n")
}

func truncateMiddle(s string, width int) string {
	if utf8.RuneCountInString(s) <= width {
		return s + strings.Repeat(" ", width-utf8.RuneCountInString(s))
	}
	half := (width - 1) / 2
	return s[:half] + "…" + s[len(s)-half:] + strings.Repeat(" ", width-utf8.RuneCountInString(s[:half]+"…"+s[len(s)-half:]))
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

	resp := util.SendRequest(app.DispatcherURL, &command)
	if resp != nil {
		fmt.Printf("Delete Job Response: %s\n", string(resp))
	}
}

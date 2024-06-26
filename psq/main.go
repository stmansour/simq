package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"strconv"
	"strings"

	"github.com/stmansour/simq/data"
	"github.com/stmansour/simq/util"
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
	File     string `json:"file"`
	Name     string `json:"name"`
	Priority int    `json:"priority"`
	URL      string `json:"url"`
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

var app struct {
	action string
	file   string
	sid    int64
}

func main() {
	action := flag.String("action", "", "Action to perform: add, list, delete")
	file := flag.String("file", "config.json5", "Path to config file (default: config.json5)")
	sid := flag.Int64("sid", 0, "Simulation ID for delete action")

	flag.Parse()
	app.action = *action

	usr, err := user.Current()
	if err != nil {
		fmt.Printf("Error getting current user: %v\n", err)
		return
	}
	username := usr.Username

	// Handle command-line arguments
	if app.action != "" {
		switch app.action {
		case "add":
			addJob(username, *file)
		case "list":
			listJobs(username)
		case "delete":
			deleteJob(username, *sid)
		default:
			fmt.Println("Invalid action. Use add, list, or delete.")
		}
		return
	}

	// Start interactive mode
	interactiveMode(username)
}

func interactiveMode(username string) {
	fmt.Printf("PSQ Version %s\nType 'help' for a list of commands.\n", util.Version())
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("psq> ")
		var input string
		input, _ = reader.ReadString('\n')
		input = strings.TrimSpace(input)
		args := strings.Fields(input)
		if len(args) == 0 {
			continue
		}

		command := strings.ToLower(args[0])
		switch command {
		case "add":
			var file string
			if len(args) > 1 {
				file = args[1]
			} else {
				file = "config.json5"
			}
			addJob(username, file)
		case "list":
			listJobs(username)
		case "delete":
			if len(args) > 1 {
				sid, err := strconv.ParseInt(args[1], 10, 64)
				if err != nil {
					fmt.Println("Error: 'delete' command requires a simulation ID.")
				}
				deleteJob(username, sid)
			} else {
				fmt.Println("Error: 'delete' command requires a simulation ID.")
			}
		case "exit", "quit":
			fmt.Println("Exiting interactive mode.")
			return
		case "help":
			fmt.Println("Available commands:")
			fmt.Println("  add [config file] - Add a new simulation (default: config.json5)")
			fmt.Println("  list - List all active simulations")
			fmt.Println("  delete <simulation ID> - Delete a simulation by ID")
			fmt.Println("  exit, quit - Exit the interactive mode")
		default:
			fmt.Println("Unknown command. Type 'help' for a list of commands.")
		}
	}
}

func addJob(username, file string) {
	config, err := readConfig(file)
	if err != nil {
		fmt.Printf("Error reading config file: %v\n", err)
		return
	}

	data := CreateQueueEntryRequest{
		File:     file,
		Name:     config.SimulationName,
		Priority: defaultPriority,
		URL:      defaultURL,
	}

	dataBytes, _ := json.Marshal(data)
	cmd := Command{
		Command:  "NewSimulation",
		Username: username,
		Data:     json.RawMessage(dataBytes),
	}

	sendRequest(cmd)
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

	sendRequest(cmd)
}

func sendRequest(cmd Command) []byte {
	cmdBytes, _ := json.Marshal(cmd)
	resp, err := http.Post(defaultURL, "application/json", bytes.NewBuffer(cmdBytes))
	if err != nil {
		fmt.Printf("Error sending request: %v\n", err)
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		return nil
	}
	return body
}

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chzyer/readline"
	"github.com/stmansour/simq/util"
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
	usr, err := user.Current()
	if err != nil {
		log.Fatalf("Failed to get current user: %v", err)
	}

	historyFile := filepath.Join(usr.HomeDir, ".psq", "history")
	err = os.MkdirAll(filepath.Dir(historyFile), os.ModePerm)
	if err != nil {
		log.Fatalf("Failed to create history directory: %v", err)
	}

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "psq> ",
		HistoryFile:     historyFile,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		log.Fatalf("Failed to initialize readline: %v", err)
	}
	defer rl.Close()

	fmt.Printf("PSQ Version %s\nType 'help' for a command list, Up Arrow for previous command, and Down Arrow for next command.\n", util.Version())

	for {
		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				continue
			}
			break
		}

		input := strings.TrimSpace(line)
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
					fmt.Println("Error: 'delete' command requires a valid simulation ID.")
				} else {
					deleteJob(username, sid)
				}
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

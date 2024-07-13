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

// CmdData represents the data for a command
type CmdData struct {
	Username string
	SID      int64
}

// DCommand represents the structure of a command
type DCommand struct {
	Command  string
	ArgCount int
	Handler  func(*CmdData, []string)
	Help     string
}

var app struct {
	action        string
	file          string
	sid           int64
	DispatcherURL string
}

// Commands represents the list of commands
var Commands []DCommand

func init() {
	Commands = []DCommand{
		{Command: "add", ArgCount: 1, Handler: addJob, Help: "Add a simulation to the queue"},
		{Command: "done", ArgCount: 0, Handler: listDoneJobs, Help: "List completed simulations"},
		{Command: "list", ArgCount: 0, Handler: listJobs, Help: "List pending simulations"},
		{Command: "listdone", ArgCount: 0, Handler: listDoneJobs, Help: "List completed simulations"},
		{Command: "delete", ArgCount: 1, Handler: deleteJob, Help: "Delete a simulation from the queue"},
		{Command: "exit", ArgCount: 0, Handler: handleExit, Help: "Exit the program"},
		{Command: "quit", ArgCount: 0, Handler: handleExit, Help: "Exit the program"},
		{Command: "help", ArgCount: 0, Handler: handleHelp, Help: "Show this help message"},
	}
}

func main() {
	app.DispatcherURL = "http://192.168.5.100:8250/" // default dispatcher URL is on plato server

	action := flag.String("action", "", "Action to perform: add, list, delete")
	dsp := flag.String("d", "", "URL to dispatcher, default: "+app.DispatcherURL)
	file := flag.String("file", "config.json5", "Path to config file (default: config.json5)")
	sid := flag.Int64("sid", 0, "Simulation ID for delete action")

	if err := util.LoadHomeDirConfig(".psqrc", &app); err != nil {
		if ! strings.Contains(err.Error(),"no such file or directory") {
		    fmt.Printf("Error loading config file: %v\n", err)
		    return
		}
	}

	flag.Parse()
	app.action = *action
	if len(*dsp) > 0 {
		app.DispatcherURL = *dsp
	}

	usr, err := user.Current()
	if err != nil {
		fmt.Printf("Error getting current user: %v\n", err)
		return
	}
	cmd := &CmdData{
		Username: usr.Username,
		SID:      *sid,
	}

	line := ""
	if *action != "" {
		for i := range Commands {
			if Commands[i].Command == *action {
				line += Commands[i].Command
				break
			}
		}
	}

	// Handle command-line arguments
	if app.action != "" {
		switch app.action {
		case "add":
			line += " " + *file
		case "delete":
			line += " " + strconv.Itoa(int(*sid))
		}
		handleCmd(line, cmd)
	}

	// Start interactive mode
	interactiveMode(cmd)
}

func interactiveMode(cmd *CmdData) {
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

	fmt.Printf("PSQ Version %s\n", util.Version())
  fmt.Printf("Targeting Dispatcher at: %s\n",app.DispatcherURL)
	fmt.Printf("Type 'help' for a command list, Up Arrow for previous command, and Down Arrow for next command.\n")

	for {
		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				continue
			}
			break
		}
		handleCmd(line, cmd)
	}
}

func handleCmd(line string, cmd *CmdData) {
	args := strings.Fields(strings.TrimSpace(line))
	if len(args) == 0 {
		return
	}

	command := strings.ToLower(args[0])
	for _, dcmd := range Commands {
		if dcmd.Command == command {
			if len(args)-1 != dcmd.ArgCount {
				fmt.Printf("%s requires %d argument(s).\n", dcmd.Command, dcmd.ArgCount)
				return
			}
			dcmd.Handler(cmd, args[1:])
			return
		}
	}
	fmt.Println("Unknown command. Type 'help' for a list of commands.")
}

func handleExit(cmd *CmdData, args []string) {
	os.Exit(0)
}

func handleHelp(cmd *CmdData, args []string) {
	for _, dcmd := range Commands {
		fmt.Printf("%s: %s\n", dcmd.Command, dcmd.Help)
	}
}

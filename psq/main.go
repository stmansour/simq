package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/user"
	"path"
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
	action         string
	file           string
	sid            int64
	DispatcherURL  string
	DispatcherHost string
	cwd            string
	version        bool
}

// Commands represents the list of commands
var Commands []DCommand

func init() {
	Commands = []DCommand{
		{Command: "a|add", ArgCount: 1, Handler: addJob, Help: "add <filename> - add a simulation to the queue"},
		{Command: "delete", ArgCount: 1, Handler: deleteJob, Help: "delete <sid> - delete a simulation from the queue"},
		{Command: "disp|dispatcher", ArgCount: 1, Handler: setDispatcherURL, Help: "dispatcher <url> - Set the URL for the dispatcher"},
		{Command: "d|done", ArgCount: 0, Handler: listDoneJobs, Help: "List completed simulations"},
		{Command: "e|exit|q|quit", ArgCount: 0, Handler: handleExit, Help: "Exit the program"},
		{Command: "help|?", ArgCount: 0, Handler: handleHelp, Help: "Show this help message"},
		{Command: "l|list", ArgCount: 0, Handler: listJobs, Help: "List pending simulations"},
		{Command: "p|pri|priority", ArgCount: 2, Handler: setPriority, Help: "priority <sid> <priority> - set the priority for <sid> to <priority>"},
		{Command: "q|quit", ArgCount: 0, Handler: handleExit, Help: "Exit the program"},
		{Command: "r|redo", ArgCount: 1, Handler: handleRedo, Help: "redo <sid> - redo simulation <sid>"},
		{Command: "sid", ArgCount: 1, Handler: getSID, Help: "sid <sid> - list details for a simulation ID. Also works with just <sid>."},
	}
}

func main() {
	var err error
	app.DispatcherHost = "http://216.16.195.147:8250/" // default dispatcher URL is on plato server
	app.cwd, err = os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current working directory: %v", err)
	}

	action := flag.String("action", "", "Action to perform: add, list, delete")
	dsp := flag.String("d", "", "URL to dispatcher, default: "+app.DispatcherHost)
	file := flag.String("file", "config.json5", "Path to config file (default: config.json5)")
	sid := flag.Int64("sid", 0, "Simulation ID for delete action")
	flag.BoolVar(&app.version, "v", false, "print the program version string")

	if err := util.LoadHomeDirConfig(".psqrc", &app); err != nil {
		if !strings.Contains(err.Error(), "no such file or directory") {
			fmt.Printf("Error loading config file: %v\n", err)
			return
		}
	}

	flag.Parse()
	if app.version {
		fmt.Println("psq version:", util.Version())
		return
	}
	app.action = *action
	if len(*dsp) > 0 {
		app.DispatcherHost = *dsp
	}
	if app.DispatcherURL, err = JoinURL(app.DispatcherHost, "command"); err != nil {
		fmt.Printf("Error joining dispatcher URL: %v\n", err)
		return
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
			ss := strings.Split(Commands[i].Command, "|")
			for j := 0; j < len(ss); j++ {
				if ss[j] == *action {
					line += ss[j]
					break
				}
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
		return
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

	// fmt.Printf("PSQ Version %s\n", util.Version())
	// fmt.Printf("Targeting Dispatcher at: %s\n", app.DispatcherHost)
	// fmt.Printf("Type 'help' for a command list, Up Arrow for previous command, and Down Arrow for next command.\n")
	printStartupMessage()

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
		ss := strings.Split(dcmd.Command, "|")
		for j := 0; j < len(ss); j++ {
			if ss[j] == command {
				if len(args)-1 != dcmd.ArgCount {
					fmt.Printf("%s requires %d argument(s).\n", dcmd.Command, dcmd.ArgCount)
					return
				}
				dcmd.Handler(cmd, args[1:])
				return
			}
		}
	}
	// See if the command is a SID
	_, err := strconv.Atoi(command)
	if err == nil {
		a := []string{command}
		getSID(cmd, a)
		return
	}

	fmt.Println("Unknown command. Type 'help' for a list of commands.")
}

func handleExit(cmd *CmdData, args []string) {
	os.Exit(0)
}

func handleHelp(cmd *CmdData, args []string) {
	for _, dcmd := range Commands {
		ss := strings.Split(dcmd.Command, "|")
		s := strings.Join(ss, ", ")
		fmt.Printf("%s: %s\n", s, dcmd.Help)
	}
}
func setDispatcherURL(cmd *CmdData, args []string) {
	var err error
	var url string
	if len(args) > 0 && len(args[0]) > 0 {
		if url, err = JoinURL(args[0], "command"); err != nil {
			fmt.Printf("Error joining dispatcher URL: %v\n", err)
			return
		}
		app.DispatcherHost = args[0]
		app.DispatcherURL = url
		fmt.Printf("Dispatcher URL set to: %s\n", app.DispatcherURL)
	}
}

// JoinURL joins base URL with endpoint.
func JoinURL(base string, endpoint string) (string, error) {
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}

	// Join paths correctly
	baseURL.Path = path.Join(baseURL.Path, endpoint)

	return baseURL.String(), nil
}

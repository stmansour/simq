package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/stmansour/simq/data"
	"github.com/stmansour/simq/util"
)

var app struct {
	qm            *data.QueueManager
	port          int
	server        *http.Server
	quit          chan os.Signal
	version       bool
	shutdownwait  int
	DispatcherURL string
	HexASCIIDbg   bool // if true print reply buffers in hex and ASCII
	HTTPHdrsDbg   bool // if true print HTTP headers
	SimResultsDir string
	QdConfigsDir  string
	mutex         sync.Mutex
}

func readCommandLineArgs() {
	flag.BoolVar(&app.version, "v", false, "print the program version string")
	flag.Parse()
}

func setMyNetworkAddress() {
	app.port = 8250
	naddrs, err := util.GetNetworkInfo()
	if err != nil {
		log.Fatalf("Failed to get network info: %v", err)
	}
	for i := 0; i < len(naddrs); i++ {
		if strings.Contains(naddrs[i].IPAddress, "127.0.0.1") {
			continue
		}
		app.DispatcherURL = fmt.Sprintf("http://%s:%d/", naddrs[i].IPAddress, app.port)
	}
}

func doMain() {
	if app.version {
		fmt.Println("dispatcher version:", util.Version())
		return
	}
	//-----------------------------------------
	// OUTPUT MESSAGES TO A LOGFILE
	//-----------------------------------------
	exdir, err := util.GetExecutableDir()
	if err != nil {
		log.Fatalf("Failed to get executable directory: %v", err)
	}
	fname := filepath.Join(exdir, "dispatcher.log")
	logFile, err := os.OpenFile(fname, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	log.Printf("---------------------------------------------------------------------\n")
	log.Printf("Dispatcher version: %s\n", util.Version())
	log.Printf("Initiated: %s\n", time.Now().Format(time.RFC3339))

	//-----------------------------------------
	// READ IN CONFIGURATION
	//-----------------------------------------
	ex, err := util.ReadExternalResources()
	if err != nil {
		log.Fatalf("Failed to read external resources: %v", err)
	}
	if ex, err = util.LoadConfig(ex, "dispatcher.json5"); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Database: %s\n", ex.DbName)
	cmd := ex.GetSQLOpenString(ex.DbName)
	setMyNetworkAddress()
	log.Printf("Dispatcher Network Address: %s\n", app.DispatcherURL)

	//-----------------------------------------
	//  OPEN DATABASE FOR THE QUEUE
	//-----------------------------------------
	app.qm, err = data.NewQueueManager(cmd)
	if err != nil {
		log.Fatalf("Failed to initialize queue manager: %v", err)
	}

	//-----------------------------------------
	// INFLOW AND OUTFLOW DIRECTORIES
	//-----------------------------------------
	app.SimResultsDir = ex.SimResultsDir
	app.QdConfigsDir = ex.DispatcherQueueDir

	//-----------------------------------------
	// SET UP HTTP LISTENER
	//-----------------------------------------
	srvAddr := fmt.Sprintf(":%d", app.port)
	mux := http.NewServeMux()
	mux.HandleFunc("/command", commandDispatcher)
	app.server = &http.Server{
		Addr:    srvAddr,
		Handler: mux,
	}

	//-----------------------------------------
	// START THE HTTP LISTENER
	//-----------------------------------------
	app.shutdownwait = 5
	go func() {
		if err := app.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()

	//---------------------------------
	// Graceful shutdown handling
	//---------------------------------
	app.quit = make(chan os.Signal, 1)
	signal.Notify(app.quit, os.Interrupt)
	<-app.quit
	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(app.shutdownwait)*time.Second)
	defer cancel()

	if err := app.server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}

func main() {
	readCommandLineArgs()
	doMain()
}

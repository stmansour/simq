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
	log.Printf("Dispatcher Network Address: %s\n", app.DispatcherURL)
}

func doMain() {
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

	ex, err := util.ReadExternalResources()
	if err != nil {
		log.Fatalf("Failed to read external resources: %v", err)
	}
	if ex, err = util.LoadConfig(ex, "dispatcher.json5"); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	cmd := ex.GetSQLOpenString(ex.DbName)
	setMyNetworkAddress()

	app.qm, err = data.NewQueueManager(cmd)
	if err != nil {
		log.Fatalf("Failed to initialize queue manager: %v", err)
	}

	app.SimResultsDir = ex.SimResultsDir
	app.QdConfigsDir = ex.DispatcherQueueDir

	srvAddr := fmt.Sprintf(":%d", app.port)
	mux := http.NewServeMux()
	mux.HandleFunc("/command", commandDispatcher)
	app.server = &http.Server{
		Addr:    srvAddr,
		Handler: mux,
	}

	if app.version {
		log.Println(util.Version())
	} else {
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
	}
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

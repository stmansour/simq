package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
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
}

func readCommandLineArgs() {
	flag.BoolVar(&app.version, "v", false, "print the program version string")
	flag.Parse()
}

func doMain() {
	app.port = 8250
	ex, err := util.ReadExternalResources()
	if err != nil {
		log.Fatalf("Failed to read external resources: %v", err)
	}
	cmd := ex.GetSQLOpenString("simq")

	app.qm, err = data.NewQueueManager(cmd)
	if err != nil {
		log.Fatalf("Failed to initialize queue manager: %v", err)
	}

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
			log.Println("Server is running on port", app.port)
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

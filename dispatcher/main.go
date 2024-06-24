package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/stmansour/simq/data"
	"github.com/stmansour/simq/util"
)

var app struct {
	qm     *data.QueueManager
	port   int
	server *http.Server
	quit   chan os.Signal
}

func main() {
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

	mux := http.NewServeMux()
	mux.HandleFunc("/command", commandDispatcher)
	app.server = &http.Server{
		Addr:    ":8250",
		Handler: mux,
	}

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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := app.server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}

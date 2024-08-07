package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/stmansour/simq/util"
)

// SimdStatus represents the structure of the status response from simd.
type SimdStatus struct {
	ProgramStarted        time.Time
	SimulationsInProgress int
	Paused                bool
	MaxSimulations        int
}

// StatusResponse is a generic status reply to a query
type StatusResponse struct {
	Status  string
	Message string
}

// StatusHandler handles the Status command
// -----------------------------------------------------------------------------
func StatusHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Status command executed")
	resp := SimdStatus{
		ProgramStarted:        app.DtStart,
		SimulationsInProgress: len(app.sims),
		Paused:                app.Paused,
		MaxSimulations:        app.cfg.MaxSimulations,
	}
	util.SvcWriteResponse(w, &resp)
}

// PauseBookingHandler handles the PauseBooking command
// -----------------------------------------------------------------------------
func PauseBookingHandler(w http.ResponseWriter, r *http.Request) {
	var msg string
	var logmsg string

	if !app.Paused {
		app.Paused = true
		msg = fmt.Sprintf("Booking paused at %s", time.Now().Format("2006-01-02 15:04:05"))
		logmsg = fmt.Sprintf("Booking paused at %s", time.Now().Format("2006-01-02 15:04:05"))
	} else {
		msg = "Booking was already paused."
		logmsg = msg
	}

	reply := StatusResponse{
		Status:  "OK",
		Message: msg,
	}
	log.Printf("%s", logmsg)
	util.SvcWriteResponse(w, &reply)
}

// ResumeBookingHandler handles the PauseBooking command
// -----------------------------------------------------------------------------
func ResumeBookingHandler(w http.ResponseWriter, r *http.Request) {
	var msg string
	var logmsg string

	if app.Paused {
		app.Paused = false
		msg = fmt.Sprintf("Booking resumed at %s", time.Now().Format("2006-01-02 15:04:05"))
		logmsg = msg
	} else {
		msg = "Booking mode was already in effect."
		logmsg = msg
	}

	reply := StatusResponse{
		Status:  "OK",
		Message: msg,
	}
	log.Printf("%s", logmsg)
	util.SvcWriteResponse(w, &reply)
}

// ShutdownHandler handles the Shutdown command
// -----------------------------------------------------------------------------
func ShutdownHandler(w http.ResponseWriter, r *http.Request) {
	// Placeholder implementation
	msg := "shutdown initiated"
	reply := StatusResponse{
		Status:  "OK",
		Message: msg,
	}
	log.Printf("%s", msg)
	util.SvcWriteResponse(w, &reply)
	app.cancel()
}

// CheckUpdatesHandler handles the CheckUpdates command
// -----------------------------------------------------------------------------
func CheckUpdatesHandler(w http.ResponseWriter, r *http.Request) {
	// Placeholder implementation
	fmt.Fprintln(w, "CheckUpdates command received")
	log.Println("CheckUpdates command executed")
}

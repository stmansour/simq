package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"unicode"
)

// SvcStatus200 is a simple status message return
type SvcStatus200 struct {
	Status  string
	Message string
}

// SvcStatus201 is a simple status message for use when a new resource is created
type SvcStatus201 struct {
	Status  string
	Message string
	ID      int64
}

// SvcWrite is a general write routine for service calls... it is a bottleneck
// where we can place debug statements as needed.
func SvcWrite(w http.ResponseWriter, b []byte) {
	w.Write(b)
}

// SvcErrorReturn formats an error return to the grid widget and sends it
func SvcErrorReturn(w http.ResponseWriter, err error) {
	var e SvcStatus200
	e.Status = "error"
	e.Message = err.Error()
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.Marshal(e)
	SvcWrite(w, b)
}

// SvcWriteResponse finishes the transaction with the W2UI client
func SvcWriteResponse(w http.ResponseWriter, g interface{}) {
	w.Header().Set("Content-Type", "application/json") // we're marshaling the data as json
	b, err := json.Marshal(g)
	if err != nil {
		LogAndErrorReturn(w, fmt.Errorf("error marshaling json data: %s", err.Error()))
		return
	}
	SvcWrite(w, b)
}

// PrintHexAndASCII formats the data in buffer so we can get an idea of what it holds
func PrintHexAndASCII(buffer []byte, maxChars int) {
	if maxChars < 1 || maxChars > len(buffer) {
		maxChars = len(buffer)
	}

	for i := 0; i < maxChars; i += 16 {
		// Print hex values
		for j := 0; j < 16; j++ {
			if i+j < maxChars {
				fmt.Printf("%02X ", buffer[i+j])
			} else {
				fmt.Print("   ")
			}
		}

		// Print ASCII values
		fmt.Print(" | ")
		for j := 0; j < 16; j++ {
			if i+j < maxChars {
				b := buffer[i+j]
				if unicode.IsPrint(rune(b)) {
					fmt.Printf("%c", b)
				} else {
					fmt.Print(".")
				}
			}
		}
		fmt.Println()
	}
}

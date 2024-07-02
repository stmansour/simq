package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"unicode"
)

// GetMacUUID returns the UUID of the current machine on macOS
func GetMacUUID() (string, error) {
	cmd := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	output := out.String()
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "IOPlatformUUID") {
			parts := strings.Split(line, " = ")
			if len(parts) > 1 {
				uuid := strings.Trim(parts[1], "\"")
				return uuid, nil
			}
		}
	}
	return "", nil
}

// GetLinuxUUID returns the UUID of the current machine
func GetLinuxUUID() (string, error) {
	data, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		data, err = os.ReadFile("/var/lib/dbus/machine-id")
		if err != nil {
			return "", err
		}
	}
	return strings.TrimSpace(string(data)), nil
}

// GetMachineUUID returns the UUID of the current machine
func GetMachineUUID() (string, error) {
	switch runtime.GOOS {
	// case "windows":
	// 	return GetWindowsUUID()
	case "linux":
		return GetLinuxUUID()
	case "darwin":
		return GetMacUUID()
	default:
		return "", fmt.Errorf("unsupported platform")
	}
}

// PrintHexAndASCII prints a buffer in hex and ASCII format
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

// SvcWrite is a general write routine for service calls... it is a bottleneck
// where we can place debug statements as needed.
// -----------------------------------------------------------------------------
func SvcWrite(w http.ResponseWriter, b []byte) {
	w.Write(b)
}

// SvcErrorReturn formats an error return to the grid widget and sends it
// -----------------------------------------------------------------------------
func SvcErrorReturn(w http.ResponseWriter, err error) {
	var e SvcStatus200
	e.Status = "error"
	e.Message = err.Error()
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.Marshal(e)
	SvcWrite(w, b)
}

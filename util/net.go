package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// NetworkInfo describes a network address for this computer
type NetworkInfo struct {
	IPAddress string
	Hostname  string
}

// Command represents the structure of a command
type Command struct {
	Command  string
	Username string
	Data     json.RawMessage
}

// GetNetworkInfo returns a list of network info
func GetNetworkInfo() ([]NetworkInfo, error) {
	var networkInfoList []NetworkInfo

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get network interfaces: %v", err)
	}

	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				if v.IP.To4() != nil { // Only consider IPv4 addresses
					hostname, _ := net.LookupAddr(v.IP.String())
					info := NetworkInfo{
						IPAddress: v.IP.String(),
						Hostname:  "",
					}
					if len(hostname) > 0 {
						info.Hostname = hostname[0]
					}
					networkInfoList = append(networkInfoList, info)
				}
			}
		}
	}

	return networkInfoList, nil
}

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

// SendRequest sends a request to the server
// -----------------------------------------------------------------
func SendRequest(url string, cmd *Command) []byte {
	cmdBytes, _ := json.Marshal(cmd)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(cmdBytes))
	if err != nil {
		fmt.Printf("Error sending request: %v\n", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Received non-OK HTTP status: %s\n", resp.Status)
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		return nil
	}

	// PrintHexAndASCII(body, 256)
	return body
}

// SendMultipartRequest sends a multipart request to the server
// -----------------------------------------------------------------
func SendMultipartRequest(url string, cmd *Command, filePath string) ([]byte, error) {
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)

	// Marshal the Command struct
	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %v", err)
	}

	// Create the JSON part
	part, err := writer.CreateFormField("data")
	if err != nil {
		return nil, fmt.Errorf("failed to create form field: %v", err)
	}
	_, err = part.Write(cmdBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to write command data: %v", err)
	}

	// Add file part
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	part, err = writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %v", err)
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return nil, fmt.Errorf("error copying file content: %v", err)
	}

	writer.Close()

	req, err := http.NewRequest("POST", url, &b)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("received non-OK HTTP status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}
	return body, nil
}

// CheckPort tries to establish a TCP connection to the given port and returns true if successful
func CheckPort(port int) bool {
	address := fmt.Sprintf("localhost:%d", port)
	conn, err := net.DialTimeout("tcp", address, 1*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// ScanPorts scans the specified range of ports and returns a list of ports with listeners
func ScanPorts(startPort, endPort int) []int {
	var openPorts []int
	for port := startPort; port <= endPort; port++ {
		if CheckPort(port) {
			openPorts = append(openPorts, port)
		}
	}
	return openPorts
}

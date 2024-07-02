package util

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// NetworkInfo describes a network address for this computer
type NetworkInfo struct {
	IPAddress string
	Hostname  string
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

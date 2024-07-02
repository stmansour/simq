package main

// import (
// 	"fmt"

// 	"github.com/StackExchange/wmi"
// )

// // GetWindowsUUID returns the UUID of the current machine
// func GetWindowsUUID() (string, error) {
// 	type Win32_ComputerSystemProduct struct {
// 		UUID string
// 	}
// 	var dst []Win32_ComputerSystemProduct
// 	q := wmi.CreateQuery(&dst, "")
// 	err := wmi.Query(q, &dst)
// 	if err != nil {
// 		return "", err
// 	}
// 	if len(dst) == 0 {
// 		return "", fmt.Errorf("no UUID found")
// 	}
// 	return dst[0].UUID, nil
// }

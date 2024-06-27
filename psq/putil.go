package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

func sendRequest(cmd Command) []byte {
	cmdBytes, _ := json.Marshal(cmd)
	resp, err := http.Post(defaultURL, "application/json", bytes.NewBuffer(cmdBytes))
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
	return body
}

func sendMultipartRequest(cmd Command, filePath string) []byte {
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)

	// Add command part
	_ = writer.WriteField("command", cmd.Command)

	// Add username part
	_ = writer.WriteField("username", cmd.Username)

	// Add data part
	dataBytes, _ := json.Marshal(cmd.Data)
	_ = writer.WriteField("data", string(dataBytes))

	// Add file part
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return nil
	}
	defer file.Close()

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		fmt.Printf("Error creating form file: %v\n", err)
		return nil
	}
	_, err = io.Copy(part, file)
	if err != nil {
		fmt.Printf("Error copying file content: %v\n", err)
		return nil
	}

	writer.Close()

	req, err := http.NewRequest("POST", defaultURL, &b)
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return nil
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error sending request: %v\n", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		fmt.Printf("Received non-OK HTTP status: %s\n", resp.Status)
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		return nil
	}
	return body
}

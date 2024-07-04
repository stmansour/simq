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

func sendMultipartRequest(cmd Command, filePath string) ([]byte, error) {
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

	req, err := http.NewRequest("POST", defaultURL, &b)
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

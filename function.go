package crawal

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// Constants for configuration
const (
	defaultTimeout = 30 * time.Second
	defaultPerms   = 0755
)

// DownloadFile downloads a file from the given URL and saves it to the specified path
// with the given filename. If the filename is empty, it uses the base name from the URL.
func DownloadFile(URL, fileName string, pathTo string) error {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: defaultTimeout,
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, URL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Send request
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer response.Body.Close()

	// Check response status
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-200 response code: %d", response.StatusCode)
	}

	// Determine filename
	if fileName == "" {
		fileName = path.Base(URL)
	}

	// Get file extension from URL if not already present
	ext := filepath.Ext(URL)
	if ext == "" && !strings.Contains(fileName, ".") {
		// Try to determine extension from Content-Type
		contentType := response.Header.Get("Content-Type")
		if strings.Contains(contentType, "jpeg") || strings.Contains(contentType, "jpg") {
			ext = ".jpg"
		} else if strings.Contains(contentType, "png") {
			ext = ".png"
		} else if strings.Contains(contentType, "gif") {
			ext = ".gif"
		} else if strings.Contains(contentType, "webp") {
			ext = ".webp"
		}
	}

	// Clean filename
	fileName = strings.ReplaceAll(fileName, " ", "_")
	fileName = strings.ReplaceAll(fileName, "/", "-")
	fileName = strings.ReplaceAll(fileName, "\\", "-")

	// Create full file path
	fullPath := filepath.Join(pathTo, fileName+ext)

	// Create the file
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Write the bytes to the file
	_, err = io.Copy(file, response.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// IntInArray checks if an integer exists in an array of integers
func IntInArray(arr []int, val int) bool {
	for _, a := range arr {
		if a == val {
			return true
		}
	}
	return false
}

// CreateFolder creates a new folder at the specified path relative to the user's home directory
func CreateFolder(path string) (string, error) {
	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	// Create the new folder path
	newFolderPath := filepath.Join(homeDir, path)

	// Create the directory and all necessary parents
	err = os.MkdirAll(newFolderPath, defaultPerms)
	if err != nil {
		return "", fmt.Errorf("failed to create folder: %w", err)
	}

	fmt.Println("New folder created at:", newFolderPath)
	return newFolderPath, nil
}

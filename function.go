package crawal

import (
	"context"
	"database/sql"
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
func DownloadFile(url, fileName string, pathTo string) error {
	// Create HTTP client with timeout
	client := &http.Client{Timeout: defaultTimeout}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-200 response code: %d", resp.StatusCode)
	}

	// Determine filename
	if fileName == "" {
		fileName = path.Base(url)
	}

	// Get file extension from URL if not already present
	ext := filepath.Ext(fileName)
	if ext == "" {
		// Try to determine extension from Content-Type
		contentType := resp.Header.Get("Content-Type")
		switch {
		case strings.Contains(contentType, "jpeg") || strings.Contains(contentType, "jpg"):
			ext = ".jpg"
		case strings.Contains(contentType, "png"):
			ext = ".png"
		case strings.Contains(contentType, "gif"):
			ext = ".gif"
		case strings.Contains(contentType, "webp"):
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
	_, err = io.Copy(file, resp.Body)
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

// FetchApi fetches data from the API
func FetchApi(client *http.Client, url string) ([]byte, error) {
	res, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return resBody, nil
}

// GetExistingWallpaperIDs retrieves the IDs of wallpapers already in the database
func GetExistingWallpaperIDs(db *sql.DB, query string) ([]int, error) {
	ids, err := db.Query(query)
	if err != nil {
		if err == sql.ErrNoRows {
			return []int{}, nil
		}
		return nil, err
	}
	defer ids.Close()

	var existingIDs []int
	for ids.Next() {
		var id int
		if err := ids.Scan(&id); err != nil {
			return nil, err
		}
		existingIDs = append(existingIDs, id)
	}

	return existingIDs, nil
}

package crawal

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func DownloadFile(URL, fileName string, pathTo string) error {
	//Get the response bytes from the url
	response, err := http.Get(URL)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return errors.New("received non 200 response code")
	}

	//Create an empty file
	if fileName == "" {
		fileName = path.Base(URL)
	}
	ext := filepath.Ext(URL)
	fileName = strings.ReplaceAll(fileName, " ", "_")
	file, err := os.Create(pathTo + "/" + fileName + ext)
	if err != nil {
		return err
	}
	defer file.Close()

	//Write the bytes to the field
	_, err = io.Copy(file, response.Body)
	if err != nil {
		return err
	}

	return nil
}

func IntInArray(arr []int, str int) bool {
	for _, a := range arr {
		if a == str {
			return true
		}
	}
	return false
}

func CreateFolder(path string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("Error getting home directory: %v", err)
	}

	// Change to the home directory
	err = os.Chdir(homeDir)
	if err != nil {
		return "", fmt.Errorf("Error changing to home directory: %v", err)
	}

	// Create the new folder
	err = os.MkdirAll(path, 0755)
	if err != nil {
		return "", fmt.Errorf("Error creating new folder: %v", err)
	}

	// Print the full path of the new folder
	newFolderPath := filepath.Join(homeDir, path)
	fmt.Println("New folder created at:", newFolderPath)
	return newFolderPath, nil
}

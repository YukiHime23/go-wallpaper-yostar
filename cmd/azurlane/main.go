package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	crawal "github.com/YukiHime23/go-craw-wallpaper-ys"
)

// Constants for configuration
const (
	defaultPath           = "AzurLane_Wallpaper"
	defaultWorkerCount    = 5
	defaultQueueSize      = 100
	defaultRequestTimeout = 30 * time.Second
)

// ResponseApi represents the API response structure
type ResponseApi struct {
	StatusCode int     `json:"statusCode"`
	Data       ResData `json:"data"`
}

// ResData represents the data structure in the API response
type ResData struct {
	Count int         `json:"count"`
	Rows  []Wallpaper `json:"rows"`
}

// Wallpaper represents a wallpaper item from the API
type Wallpaper struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Cover       string `json:"cover"`
	Works       string `json:"works"`
	Type        int    `json:"type"`
	Sort        int    `json:"sort_index"`
	PublishTime int    `json:"publish_time"`
	New         bool   `json:"new"`
}

// AzurLane represents a wallpaper to be downloaded
type AzurLane struct {
	FileName    string `json:"file_name"`
	IdWallpaper int    `json:"id_wallpaper"`
	Url         string `json:"url"`
}

var (
	apiListWallpaperAzurLane    = "https://azurlane.yo-star.com/api/admin/special/public-list?page_index=1&page_num=1200&type=1"
	domainLoadWallpaperAzurLane = "https://webusstatic.yo-star.com/"
)

func main() {
	// Parse command line flags
	pathP := flag.String("path", defaultPath, "Path to the directory where wallpapers should be saved.")
	flag.Parse()

	// Create output directory
	newPath, err := crawal.CreateFolder(*pathP)
	if err != nil {
		log.Fatalf("Failed to create folder: %v", err)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: defaultRequestTimeout,
	}

	// Fetch wallpaper list
	wallpapers, err := fetchWallpapers(client)
	if err != nil {
		log.Fatalf("Failed to fetch wallpapers: %v", err)
	}

	// Get database connection
	db := crawal.GetSqliteDb()
	defer db.Close()

	// Get existing wallpaper IDs
	existingIDs, err := getExistingWallpaperIDs(db)
	if err != nil {
		log.Fatalf("Failed to get existing wallpaper IDs: %v", err)
	}

	// Filter out existing wallpapers
	wallpapersToDownload := filterNewWallpapers(wallpapers, existingIDs)

	// Create a channel for the wallpaper queue
	queue := make(chan AzurLane, defaultQueueSize)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < defaultWorkerCount; i++ {
		wg.Add(1)
		go crawURL(db, queue, newPath, &wg)
	}

	// Feed the queue
	go func() {
		for _, wallpaper := range wallpapersToDownload {
			queue <- wallpaper
			log.Printf("File %s has been enqueued", wallpaper.FileName)
		}
		close(queue)
	}()

	// Wait for all workers to complete
	wg.Wait()
	log.Println("All workers are done, exiting program.")
}

// fetchWallpapers retrieves the list of wallpapers from the API
func fetchWallpapers(client *http.Client) ([]Wallpaper, error) {
	res, err := client.Get(apiListWallpaperAzurLane)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var resApi ResponseApi
	if err = json.Unmarshal(resBody, &resApi); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return resApi.Data.Rows, nil
}

// getExistingWallpaperIDs retrieves the IDs of wallpapers already in the database
func getExistingWallpaperIDs(db *sql.DB) ([]int, error) {
	ids, err := db.Query("SELECT id_wallpaper FROM azur_lane")
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

// filterNewWallpapers filters out wallpapers that already exist in the database
func filterNewWallpapers(wallpapers []Wallpaper, existingIDs []int) []AzurLane {
	listWallpp := make([]AzurLane, 0, len(wallpapers))
	for _, row := range wallpapers {
		if crawal.IntInArray(existingIDs, row.ID) {
			continue
		}

		al := AzurLane{
			Url:         domainLoadWallpaperAzurLane + row.Works,
			FileName:    strings.ReplaceAll(row.Title+" ("+row.Artist+").jpeg", "/", "-"),
			IdWallpaper: row.ID,
		}

		listWallpp = append(listWallpp, al)
	}
	return listWallpp
}

// crawURL downloads wallpapers and inserts them into the database
func crawURL(db *sql.DB, queue <-chan AzurLane, path string, wg *sync.WaitGroup) {
	defer wg.Done()

	// Prepare the SQL statement once for better performance
	insertStmt, err := db.Prepare("INSERT INTO azur_lane VALUES (?, ?, ?)")
	if err != nil {
		log.Printf("Error preparing SQL statement: %v", err)
		return
	}
	defer insertStmt.Close()

	for al := range queue {
		// Download the file
		if err := crawal.DownloadFile(al.Url, al.FileName, path); err != nil {
			log.Printf("Error downloading file %s: %v", al.FileName, err)
			continue
		}
		log.Printf(`-> download done "%s" <-`, al.FileName)

		// Insert into database
		_, err := insertStmt.Exec(al.IdWallpaper, al.FileName, al.Url)
		if err != nil {
			log.Printf("Error inserting data for %s: %v", al.FileName, err)
			continue
		}
	}
	log.Println("Worker done and exit")
}

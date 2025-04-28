package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	craw "github.com/YukiHime23/go-craw-wallpaper-ys"
	_ "github.com/mattn/go-sqlite3"
)

// Constants for configuration
const (
	defaultPath           = "AetherGazer_Wallpaper"
	defaultWorkerCount    = 5
	defaultQueueSize      = 100
	defaultRequestTimeout = 30 * time.Second
	dbPath                = "data-aether-gazer.db"
)

// ResponseApi represents the API response structure
type ResponseApi struct {
	Code int     `json:"code"`
	Data ResData `json:"data"`
	Msg  string  `json:"msg"`
}

// ResData represents the data structure in the API response
type ResData struct {
	Count int         `json:"count"`
	Rows  []Wallpaper `json:"rows"`
}

// Wallpaper represents a wallpaper item from the API
type Wallpaper struct {
	ID                int    `json:"id"`
	Title             string `json:"title"`
	Type              string `json:"type"`
	ContentImg        string `json:"contentImg"`
	MobileContentImg1 string `json:"mobileContentImg1"`
	StickerUrl        string `json:"stickerUrl"`
	Creator           string `json:"creator"`
}

// ImageDownload represents an image to be downloaded
type ImageDownload struct {
	URL      string
	FileName string
	Path     string
}

var (
	apiListWallpaperAetherGazer = "https://aethergazer.com/api/gallery/list?pageIndex=1&pageNum=1200&type=wallpaper"
)

// initDB initializes the SQLite database and creates the necessary tables
func initDB() (*sql.DB, error) {
	// Connect to the SQLite database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Check if the table exists, if not create it
	createTable := `
		CREATE TABLE IF NOT EXISTS aether_gazer (
			id INT PRIMARY KEY,
			title VARCHAR(255) NOT NULL,
			type VARCHAR(100) NOT NULL,
			content_img VARCHAR(255) NOT NULL,
			mobile_content_img1 VARCHAR(255),
			sticker_url VARCHAR(255),
			creator VARCHAR(100)
		);
	`
	_, err = db.Exec(createTable)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	return db, nil
}

func main() {
	// Parse command line flags
	pathP := flag.String("path", defaultPath, "Path to the directory where wallpapers should be saved.")
	flag.Parse()

	// Create subdirectories for different image types
	contentImgPath, err := craw.CreateFolder(filepath.Join(*pathP, "contentImg"))
	if err != nil {
		log.Fatalf("Failed to create contentImg folder: %v", err)
	}
	mobileContentImgPath, err := craw.CreateFolder(filepath.Join(*pathP, "mobileContentImg"))
	if err != nil {
		log.Fatalf("Failed to create mobileContentImg folder: %v", err)
	}

	// Initialize database
	db, err := initDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: defaultRequestTimeout,
	}

	// Fetch wallpaper list
	wallpapers, err := fetchWallpapers(client)
	if err != nil {
		log.Fatalf("Failed to fetch wallpapers: %v", err)
	}

	// Get existing wallpaper IDs
	existingIDs, err := craw.GetExistingWallpaperIDs(db, "SELECT id FROM aether_gazer")
	if err != nil {
		log.Fatalf("Failed to get existing wallpaper IDs: %v", err)
	}

	// Prepare images for download
	imagesToDownload := prepareImagesForDownload(wallpapers, existingIDs, contentImgPath, mobileContentImgPath)

	// Create a channel for the image queue
	queue := make(chan ImageDownload, defaultQueueSize)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < defaultWorkerCount; i++ {
		wg.Add(1)
		go downloadWorker(queue, &wg)
	}

	// Feed the queue
	go func() {
		for _, img := range imagesToDownload {
			queue <- img
			log.Printf("Image %s has been enqueued", img.FileName)
		}
		close(queue)
	}()

	// Wait for all workers to complete
	wg.Wait()
	log.Println("All workers are done, exiting program.")
}

// fetchWallpapers retrieves the list of wallpapers from the API
func fetchWallpapers(client *http.Client) ([]Wallpaper, error) {
	resBody, err := craw.FetchApi(client, apiListWallpaperAetherGazer)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch wallpapers: %w", err)
	}

	var resApi ResponseApi
	if err = json.Unmarshal(resBody, &resApi); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return resApi.Data.Rows, nil
}

// prepareImagesForDownload prepares the list of images to download
func prepareImagesForDownload(wallpapers []Wallpaper, existingIDs []int, contentImgPath, mobileContentImgPath string) []ImageDownload {
	imagesToDownload := make([]ImageDownload, 0, len(wallpapers)*2) // Estimate 2 images per wallpaper

	for _, wallpaper := range wallpapers {
		// Skip if already in database
		if craw.IntInArray(existingIDs, wallpaper.ID) {
			continue
		}

		// Clean filename
		baseFileName := strings.ReplaceAll(wallpaper.Title, " ", "_")
		baseFileName = strings.ReplaceAll(baseFileName, "/", "-")
		baseFileName = strings.ReplaceAll(baseFileName, "\\", "-")

		// Add content image if available
		if wallpaper.ContentImg != "" {
			imagesToDownload = append(imagesToDownload, ImageDownload{
				URL:      wallpaper.ContentImg,
				FileName: fmt.Sprintf("%s_%d_content", baseFileName, wallpaper.ID),
				Path:     contentImgPath,
			})
		}

		// Add mobile content image if available
		if wallpaper.MobileContentImg1 != "" {
			imagesToDownload = append(imagesToDownload, ImageDownload{
				URL:      wallpaper.MobileContentImg1,
				FileName: fmt.Sprintf("%s_%d_mobile", baseFileName, wallpaper.ID),
				Path:     mobileContentImgPath,
			})
		}
	}

	return imagesToDownload
}

// downloadWorker downloads images from the queue
func downloadWorker(queue <-chan ImageDownload, wg *sync.WaitGroup) {
	defer wg.Done()

	for img := range queue {
		// Download the file
		if err := craw.DownloadFile(img.URL, img.FileName, img.Path); err != nil {
			log.Printf("Error downloading image %s: %v", img.FileName, err)
			continue
		}
		log.Printf(`-> download done "%s" <-`, img.FileName)
	}
	log.Println("Worker done and exit")
}

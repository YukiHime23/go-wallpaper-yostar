package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"slices"
	"sync"
	"time"

	ys "github.com/YukiHime23/go-wallpaper-yostar"
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
type responseApi struct {
	Code int     `json:"code"`
	Data resData `json:"data"`
	Msg  string  `json:"msg"`
}

// ResData represents the data structure in the API response
type resData struct {
	Count int         `json:"count"`
	Rows  []wallpaper `json:"rows"`
}

// Wallpaper represents a wallpaper item from the API
type wallpaper struct {
	ID                int    `json:"id"`
	Title             string `json:"title"`
	Type              string `json:"type"`
	ContentImg        string `json:"contentImg"`
	MobileContentImg1 string `json:"mobileContentImg1"`
	StickerUrl        string `json:"stickerUrl"`
	Creator           string `json:"creator"`
}

// ImageDownload represents an image to be downloaded
type imageDownload struct {
	IdGallery string `json:"id_gallery"`
	URL       string `json:"url"`
	FileName  string `json:"file_name"`
	Path      string `json:"path"`
	Type      string `json:"type"`
}

var (
	apiListWallpaperAetherGazer = "https://aethergazer.com/api/gallery/list?pageIndex=1&pageNum=12000&type=wallpaper"
)

func main() {
	// Parse command line flags
	pathP := flag.String("path", defaultPath, "Path to the directory where wallpapers should be saved.")
	flag.Parse()

	// Create subdirectories for different image types
	contentImgPath, err := ys.CreateFolder(filepath.Join(*pathP, "contentImg"))
	if err != nil {
		log.Fatalf("Failed to create contentImg folder: %v", err)
	}
	mobileContentImgPath, err := ys.CreateFolder(filepath.Join(*pathP, "mobileContentImg"))
	if err != nil {
		log.Fatalf("Failed to create mobileContentImg folder: %v", err)
	}

	// Initialize database
	db := ys.GetSqliteDb()

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
	existingIDs, err := ys.GetExistingWallpaperIDs(db, "SELECT id_gallery FROM yostar_gallery WHERE game = 'aether_gazer'")
	if err != nil {
		log.Fatalf("Failed to get existing wallpaper IDs: %v", err)
	}

	// Prepare images for download
	imagesToDownload := prepareImagesForDownload(wallpapers, existingIDs, contentImgPath, mobileContentImgPath)

	// Create a channel for the image queue
	queue := make(chan imageDownload, defaultQueueSize)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < defaultWorkerCount; i++ {
		wg.Add(1)
		go downloadWorker(db, queue, &wg)
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
func fetchWallpapers(client *http.Client) ([]wallpaper, error) {
	resBody, err := ys.FetchApi(client, apiListWallpaperAetherGazer)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch wallpapers: %w", err)
	}

	var resApi responseApi
	if err = json.Unmarshal(resBody, &resApi); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return resApi.Data.Rows, nil
}

// prepareImagesForDownload prepares the list of images to download
func prepareImagesForDownload(wallpapers []wallpaper, existingIDs []string, contentImgPath, mobileContentImgPath string) []imageDownload {
	imagesToDownload := make([]imageDownload, 0, len(wallpapers)*2) // Estimate 2 images per wallpaper

	for _, wallpaper := range wallpapers {
		// Skip if already in database
		if slices.Contains(existingIDs, fmt.Sprintf("%d", wallpaper.ID)) {
			continue
		}

		// Add content image if available
		if wallpaper.ContentImg != "" {
			imagesToDownload = append(imagesToDownload, imageDownload{
				IdGallery: fmt.Sprintf("%d", wallpaper.ID),
				URL:       wallpaper.ContentImg,
				FileName:  fmt.Sprintf("%s(%s)", wallpaper.Title, wallpaper.Creator),
				Path:      contentImgPath,
				Type:      "wallpaper",
			})
		}

		// Add mobile content image if available
		if wallpaper.MobileContentImg1 != "" {
			imagesToDownload = append(imagesToDownload, imageDownload{
				IdGallery: fmt.Sprintf("%d", wallpaper.ID),
				URL:       wallpaper.MobileContentImg1,
				FileName:  fmt.Sprintf("%s(%s)", wallpaper.Title, wallpaper.Creator),
				Path:      mobileContentImgPath,
				Type:      "mobile",
			})
		}
	}

	return imagesToDownload
}

// downloadWorker downloads images from the queue
func downloadWorker(db *sql.DB, queue <-chan imageDownload, wg *sync.WaitGroup) {
	defer wg.Done()

	for img := range queue {
		// Download the file
		if err := ys.DownloadFile(img.URL, img.FileName, img.Path); err != nil {
			log.Printf("Error downloading image %s: %v", img.FileName, err)
			continue
		}
		log.Printf(`-> download done "%s" <-`, img.FileName)

		// Insert into database
		_, err := db.Exec("INSERT INTO yostar_gallery(id_gallery, game, type, file_name, url) VALUES (?, ?, ?, ?, ?)", img.IdGallery, "aether_gazer", img.Type, img.FileName, img.URL)
		if err != nil {
			log.Printf("Error inserting data for %s: %v", img.FileName, err)
			continue
		}
	}
	log.Println("Worker done and exit")
}

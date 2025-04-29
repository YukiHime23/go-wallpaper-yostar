package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"slices"
	"sync"
	"time"

	craw "github.com/YukiHime23/go-craw-wallpaper-ys"
)

type responseApi struct {
	Code int     `json:"code"`
	Data resData `json:"data"`
	Msg  string  `json:"msg"`
}

type resData struct {
	Count int            `json:"count"`
	Rows  []wallpaperRow `json:"rows"`
}

type wallpaperRow struct {
	ID          int    `json:"id"`
	PC          string `json:"pc"`
	Mobile1     string `json:"mobile1"`
	Mobile2     string `json:"mobile2"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type majongSoul struct {
	IdGallery string `json:"id_gallery"`
	FileName  string `json:"file_name"`
	Url       string `json:"url"`
}

const (
	apiListWallpaperMahjongSoul = "https://mahjongsoul.yo-star.com/api/assets/wallpaper?pageIndex=1&pageNum=12000"
	defaultPath                 = "MahjongSoul_Wallpaper"
	defaultWorkerCount          = 5
	defaultQueueSize            = 100
	defaultRequestTimeout       = 30 * time.Second
)

func main() {
	// Parse command line flags
	pathP := flag.String("path", defaultPath, "Path to the directory where wallpapers should be saved.")
	flag.Parse()

	// Create output directory
	newPath, err := craw.CreateFolder(*pathP)
	if err != nil {
		log.Fatalf("Failed to create folder: %v", err)
	}

	// Initialize database
	db, err := craw.InitDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: defaultRequestTimeout,
	}

	// Fetch wallpaper list
	wallpapers, err := fetchWallpapers(client, apiListWallpaperMahjongSoul)
	if err != nil {
		log.Fatalf("Failed to fetch wallpapers: %v", err)
	}

	// Get existing wallpaper IDs
	existingIDs, err := craw.GetExistingWallpaperIDs(db, "SELECT id, id_gallery FROM yostar_gallery WHERE type = 'mahjong_soul'")
	if err != nil {
		log.Fatalf("Failed to get existing wallpaper IDs: %v", err)
	}

	// Filter out existing wallpapers
	wallpapersToDownload := filterNewWallpapers(wallpapers, existingIDs)

	// Create a channel for the wallpaper queue
	queue := make(chan majongSoul, defaultQueueSize)

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
func fetchWallpapers(client *http.Client, url string) ([]wallpaperRow, error) {
	resBody, err := craw.FetchApi(client, url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch wallpapers: %w", err)
	}

	var resApi responseApi
	if err = json.Unmarshal(resBody, &resApi); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return resApi.Data.Rows, nil
}

// filterNewWallpapers filters out wallpapers that already exist in the database
func filterNewWallpapers(wallpapers []wallpaperRow, existingIDs []string) []majongSoul {
	listWallpp := make([]majongSoul, 0, len(wallpapers))
	for _, row := range wallpapers {
		if slices.Contains(existingIDs, fmt.Sprintf("%d", row.ID)) {
			continue
		}

		al := majongSoul{
			IdGallery: fmt.Sprintf("%d", row.ID),
			Url:       row.PC,
			FileName:  row.Title,
		}

		listWallpp = append(listWallpp, al)
	}
	return listWallpp
}

// crawURL downloads wallpapers and inserts them into the database
func crawURL(db *sql.DB, queue <-chan majongSoul, path string, wg *sync.WaitGroup) {
	defer wg.Done()

	// Prepare the SQL statement once for better performance
	insertStmt, err := db.Prepare("INSERT INTO yostar_gallery(id_gallery, game, type, file_name, url) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		log.Printf("Error preparing SQL statement: %v", err)
		return
	}
	defer insertStmt.Close()

	for al := range queue {
		// Download the file
		if err := craw.DownloadFile(al.Url, al.FileName, path); err != nil {
			log.Printf("Error downloading file %s: %v", al.FileName, err)
			continue
		}
		log.Printf(`-> download done "%s" <-`, al.FileName)

		// Insert into database
		_, err := insertStmt.Exec(al.IdGallery, "mahjong_soul", "wallpaper", al.FileName, al.Url)
		if err != nil {
			log.Printf("Error inserting data for %s: %v", al.FileName, err)
			continue
		}
	}
	log.Println("Worker done and exit")
}

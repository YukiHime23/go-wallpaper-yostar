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

	ys "github.com/YukiHime23/go-wallpaper-yostar"
)

type responseApi struct {
	Retcode int     `json:"retcode"`
	Data    resData `json:"data"`
}

type resData struct {
	PageCountNum int      `json:"pageCountNum"`
	FankitList   []fankit `json:"fankitList"`
}

type wallpaper struct {
	L string `json:"l"`
	M string `json:"m"`
	S string `json:"s"`
}

type Asset struct {
	Count int    `json:"count"`
	ID    string `json:"_id"`
	Index string `json:"index"`
	URL   string `json:"url"`
}

type fankit struct {
	Wallpaper      wallpaper `json:"wallpaper"`
	WallpaperCount int       `json:"wallpaperCount"`
	ZipCount       int       `json:"zipCount"`
	ID             string    `json:"_id"`
	Type           string    `json:"type"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	ArtistName     string    `json:"artistName"`
	ArtistLink     string    `json:"artistLink"`
	Assets         []Asset   `json:"assets"`
	Zip            string    `json:"zip"`
	ZipSize        string    `json:"zipSize"`
	IsPublic       bool      `json:"ispublic"`
	Index          int       `json:"index"`
	CreatedAt      string    `json:"createdAt"`
	V              int       `json:"__v"`
}

type Arknight struct {
	IdGallery string `json:"id_gallery"`
	FileName  string `json:"file_name"`
	Url       string `json:"url"`
}

var (
	apiListWallpaperArknight = "https://arknights.global/api/cms/fankit/queryFankit?pageIndex=1&pageNum=1200&type=1"
	baseUrlLoadWallpaper     = "https://webusstatic.yo-star.com/"
	defaultPath              = "Arknight_Wallpaper"
)

const (
	defaultWorkerCount    = 5
	defaultQueueSize      = 100
	defaultRequestTimeout = 30 * time.Second
)

func main() {
	// Parse command line flags
	pathP := flag.String("path", defaultPath, "Path to the directory where wallpapers should be saved.")
	flag.Parse()

	// Create output directory
	newPath, err := ys.CreateFolder(*pathP)
	if err != nil {
		log.Fatalf("Failed to create folder: %v", err)
	}

	// Initialize database
	db := ys.GetSqliteDb()

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: defaultRequestTimeout,
	}

	// Fetch wallpaper list
	wallpapers, err := fetchWallpapers(client, apiListWallpaperArknight)
	if err != nil {
		log.Fatalf("Failed to fetch wallpapers: %v", err)
	}

	// Get existing wallpaper IDs
	existingIDs, err := ys.GetExistingWallpaperIDs(db, "SELECT id_gallery FROM yostar_gallery WHERE game = 'arknight'")
	if err != nil {
		log.Fatalf("Failed to get existing wallpaper IDs: %v", err)
	}

	// Filter out existing wallpapers
	wallpapersToDownload := filterNewWallpapers(wallpapers, existingIDs)

	// Create a channel for the wallpaper queue
	queue := make(chan Arknight, defaultQueueSize)

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
func fetchWallpapers(client *http.Client, url string) ([]fankit, error) {
	resBody, err := ys.FetchApi(client, url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch wallpapers: %w", err)
	}

	var resApi responseApi
	if err = json.Unmarshal(resBody, &resApi); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return resApi.Data.FankitList, nil
}

// filterNewWallpapers filters out wallpapers that already exist in the database
func filterNewWallpapers(wallpapers []fankit, existingIDs []string) []Arknight {
	listWallpp := make([]Arknight, 0, len(wallpapers))
	for _, row := range wallpapers {
		if slices.Contains(existingIDs, row.ID) {
			continue
		}

		al := Arknight{
			IdGallery: row.ID,
			Url:       baseUrlLoadWallpaper + row.Wallpaper.L,
			FileName:  fmt.Sprintf("%s (%s)", row.Title, row.ArtistName),
		}

		listWallpp = append(listWallpp, al)
	}
	return listWallpp
}

// crawURL downloads wallpapers and inserts them into the database
func crawURL(db *sql.DB, queue <-chan Arknight, path string, wg *sync.WaitGroup) {
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
		if err := ys.DownloadFile(al.Url, al.FileName, path); err != nil {
			log.Printf("Error downloading file %s: %v", al.FileName, err)
			continue
		}
		log.Printf(`-> download done "%s" <-`, al.FileName)

		// Insert into database
		_, err := insertStmt.Exec(al.IdGallery, "arknight", "wallpaper", al.FileName, al.Url)
		if err != nil {
			log.Printf("Error inserting data for %s: %v", al.FileName, err)
			continue
		}
	}
	log.Println("Worker done and exit")
}

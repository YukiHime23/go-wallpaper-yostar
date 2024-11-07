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

	"github.com/YukiHime23/go-craw-wallpaper-ys"
)

type ResponseApi struct {
	StatusCode int     `json:"statusCode"`
	Data       ResData `json:"data"`
}

type ResData struct {
	Count int         `json:"count"`
	Rows  []Wallpaper `json:"rows"`
}

type Wallpaper struct {
	ID                int     `json:"id"`
	Title             string  `json:"title"`
	Type              string  `json:"type"`
	Description       string  `json:"description"`
	Lang              string  `json:"lang"`
	ContentImg        string  `json:"contentImg"`
	MobileContentImg1 string  `json:"mobileContentImg1"`
	MobileContentImg2 string  `json:"mobileContentImg2"`
	StickerList       *string `json:"stickerlist"`
	PcThumbnail       string  `json:"pcThumbnail"`
	MobileThumbnail   string  `json:"mobileThumbnail"`
	ModelURL          string  `json:"modelUrl"`
	StickerURL        string  `json:"stickerUrl"`
	PvURL             string  `json:"pvUrl"`
	Creator           string  `json:"creator"`
	SortIndex         int     `json:"sortIndex"`
	DownloadCount     int     `json:"downloadCount"`
	ClickCount        int     `json:"clickCount"`
	CreatedAt         int64   `json:"createdAt"`
	UpdatedAt         int64   `json:"updatedAt"`
}

type AetherGazer struct {
	FileName    string `json:"file_name"`
	IdWallpaper int    `json:"id_wallpaper"`
	Url         string `json:"url"`
}

var (
	ApiListWallpaperAetherGazer = "https://www.arknights.global:8082/fankit/queryFankit?pageIndex=1&pageNum=6&type=1"
)

func main() {
	var pathFile string
	pathP := flag.String("path", "", "Path to the directory where wallpapers should be saved.")
	flag.Parse()
	if pathP == nil || *pathP == "" {
		pathFile = "AetherGazer_Wallpaper"
	} else {
		pathFile = *pathP
	}

	newPath, err := crawal.CreateFolder(pathFile)
	if err != nil {
		log.Fatal(err)
	}

	res, err := http.Get(ApiListWallpaperAetherGazer)
	if err != nil {
		log.Fatal("call api error: ", err)
	}

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatal("read body error: ", err)
	}

	var resApi ResponseApi
	if err = json.Unmarshal(resBody, &resApi); err != nil {
		log.Fatal("json Unmarshal error: ", err)
	}

	db := crawal.GetSqliteDb()
	createTable(db)

	var idExist []int
	// get id exist
	ids, err := db.Query("SELECT id_wallpaper FROM aether_gazer")
	if err != nil && err != sql.ErrNoRows {
		log.Fatal("select id error: ", err)
	}
	defer ids.Close()

	var id int
	for ids.Next() {
		ids.Scan(&id)
		idExist = append(idExist, id)
	}

	listWallpp := make([]AetherGazer, 0)
	for _, row := range resApi.Data.Rows {
		if crawal.IntInArray(idExist, row.ID) {
			continue
		}

		var ag AetherGazer
		ag.Url = row.ContentImg
		ag.FileName = strings.ReplaceAll(row.Title+" ("+row.Creator+").jpeg", "/", "-")
		ag.IdWallpaper = row.ID

		listWallpp = append(listWallpp, ag)
	}
	var wg sync.WaitGroup
	queue := startCraw(listWallpp)

	for i := 1; i <= 5; i++ {
		wg.Add(1)
		go crawURL(db, queue, newPath, &wg)
	}
	wg.Wait()

	fmt.Println("All workers are done, exiting program.")
	defer db.Close()
}

func createTable(db *sql.DB) {
	// Kiểm tra xem bảng có tồn tại hay không, nếu không thì tạo mới
	createTable := `
		CREATE TABLE IF NOT EXISTS aether_gazer (
			id_wallpaper INT PRIMARY KEY,
			file_name VARCHAR(255) NOT NULL,
			url VARCHAR(255) NOT NULL
		);
	`
	_, err := db.Exec(createTable)
	if err != nil {
		log.Fatal(err)
	}
}

func crawURL(db *sql.DB, queue <-chan AetherGazer, path string, wg *sync.WaitGroup) {
	defer wg.Done()

	for al := range queue {
		if err := crawal.DownloadFile(al.Url, al.FileName, path); err != nil {
			log.Fatal("download file error: ", err)
		}
		fmt.Printf(`-> download done "%s" <-`, al.FileName)

		insertData := "INSERT INTO aether_gazer VALUES (?, ?, ?)"
		_, err := db.Exec(insertData, al.IdWallpaper, al.FileName, al.Url)
		if err != nil {
			log.Fatal(err)
		}

	}
	fmt.Println("Worker done and exit")
}

func startCraw(list []AetherGazer) <-chan AetherGazer {
	queue := make(chan AetherGazer, 100)

	go func() {
		for _, v := range list {
			queue <- v
			fmt.Printf("File %s has been enqueued\n", v.FileName)
		}
		close(queue)
	}()

	return queue
}

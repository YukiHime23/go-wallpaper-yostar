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

	crawal "github.com/YukiHime23/go-craw-wallpaper-ys"
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

type AzurLane struct {
	FileName    string `json:"file_name"`
	IdWallpaper int    `json:"id_wallpaper"`
	Url         string `json:"url"`
}

var (
	ApiListWallpaperAzurLane    = "https://azurlane.yo-star.com/api/admin/special/public-list?page_index=1&page_num=1200&type=1"
	DomainLoadWallpaperAzurLane = "https://webusstatic.yo-star.com/"
)

func main() {
	var pathFile string
	pathP := flag.String("path", "", "Path to the directory where wallpapers should be saved.")
	flag.Parse()
	if pathP == nil || *pathP == "" {
		pathFile = "AzurLane_Wallpaper"
	} else {
		pathFile = *pathP
	}

	newPath, err := crawal.CreateFolder(pathFile)
	if err != nil {
		log.Fatal(err)
	}

	res, err := http.Get(ApiListWallpaperAzurLane)
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

	var idExist []int
	// get id exist
	ids, err := db.Query("SELECT id_wallpaper FROM azur_lane")
	if err != nil && err != sql.ErrNoRows {
		log.Fatal("select id error: ", err)
	}
	defer ids.Close()

	var id int
	for ids.Next() {
		ids.Scan(&id)
		idExist = append(idExist, id)
	}

	listWallpp := make([]AzurLane, 0)
	for _, row := range resApi.Data.Rows {
		if crawal.IntInArray(idExist, row.ID) {
			continue
		}

		var al AzurLane
		al.Url = DomainLoadWallpaperAzurLane + row.Works
		al.FileName = strings.ReplaceAll(row.Title+" ("+row.Artist+").jpeg", "/", "-")
		al.IdWallpaper = row.ID

		listWallpp = append(listWallpp, al)
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

func crawURL(db *sql.DB, queue <-chan AzurLane, path string, wg *sync.WaitGroup) {
	defer wg.Done()

	for al := range queue {
		if err := crawal.DownloadFile(al.Url, al.FileName, path); err != nil {
			log.Fatal("download file error: ", err)
		}
		fmt.Printf(`-> download done "%s" <-`, al.FileName)

		insertData := "INSERT INTO azur_lane VALUES (?, ?, ?)"
		_, err := db.Exec(insertData, al.IdWallpaper, al.FileName, al.Url)
		if err != nil {
			log.Fatal(err)
		}

	}
	fmt.Println("Worker done and exit")
}

func startCraw(list []AzurLane) <-chan AzurLane {
	queue := make(chan AzurLane, 100)

	go func() {
		for _, v := range list {
			queue <- v
			fmt.Printf("File %s has been enqueued\n", v.FileName)
		}
		close(queue)
	}()

	return queue
}

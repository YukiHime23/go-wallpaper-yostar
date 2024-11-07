package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

	craw "github.com/YukiHime23/go-craw-wallpaper-ys"
)

type ResponseApi struct {
	StatusCode int     `json:"code"`
	Data       ResData `json:"data"`
	Msg        string  `json:"msg"`
}

type ResData struct {
	Count int         `json:"count"`
	Rows  []Wallpaper `json:"rows"`
}

type Wallpaper struct {
	GalleryID         int    `json:"gallery_id"`
	Title             string `json:"title"`
	Type              string `json:"type"`
	ContentImg        string `json:"contentImg"`
	MobileContentImg1 string `json:"mobileContentImg1"`
	MobileContentImg2 string `json:"mobileContentImg2"`
	PcThumbnail       string `json:"pcThumbnail"`
	MobileThumbnail   string `json:"mobileThumbnail"`
	StickerURL        string `json:"stickerUrl"`
	Creator           string `json:"creator"`
}

var (
	ApiListWallpaperAetherGazer = "https://aethergazer.com/api/gallery/list?pageIndex=1&pageNum=1200&type=wallpaper"
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

	newPath, err := craw.CreateFolder(pathFile)
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

	var db *sql.DB
	db = createTable(db)

	var idExist []int
	// get id exist
	ids, err := db.Query("SELECT gallery_id FROM `aether_gazer`")
	if err != nil && err != sql.ErrNoRows {
		log.Fatal("select gallery_id error: ", err)
	}
	defer ids.Close()

	var id int
	for ids.Next() {
		ids.Scan(&id)
		idExist = append(idExist, id)
	}

	listWallpp := make([]Wallpaper, 0)
	for _, row := range resApi.Data.Rows {
		if craw.IntInArray(idExist, row.GalleryID) {
			continue
		}

		listWallpp = append(listWallpp, row)
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

func createTable(db *sql.DB) *sql.DB {
	var err error
	// Kết nối đến cơ sở dữ liệu SQLite
	dataSource := "/home/kiennt1/Documents/go-craw-wallpaper-ys/" + "data-aether-gazer.db"
	db, err = sql.Open("sqlite3", dataSource)
	if err != nil {
		log.Fatal(err)
	}

	// Kiểm tra xem bảng có tồn tại hay không, nếu không thì tạo mới
	query :=
		`CREATE TABLE IF NOT EXISTS aether_gazer (
			gallery_id INT NOT NULL,
			title VARCHAR(255) NOT NULL,
			type VARCHAR(100) NOT NULL,
			content_img VARCHAR(255) NOT NULL,
			mobile_content_img1 VARCHAR(255) NOT NULL,
			mobile_content_img2 VARCHAR(255) NOT NULL,
			pc_thumbnail VARCHAR(255) NOT NULL,
			mobile_thumbnail VARCHAR(255) NOT NULL,
			sticker_url VARCHAR(255) NOT NULL,
			creator VARCHAR(100) NOT NULL
		);
	`
	_, err = db.Exec(query)
	if err != nil {
		log.Fatal(err)
	}

	return db
}

func crawURL(db *sql.DB, queue <-chan Wallpaper, path string, wg *sync.WaitGroup) {
	defer wg.Done()

	for al := range queue {
		fName := fmt.Sprint(al.Creator, "_", al.Title)
		if err := craw.DownloadFile(al.ContentImg, fName, path); err != nil {
			log.Fatal("download file error: ", err)
		}
		fmt.Printf(`-> download done "%s" <-`, fName)

		insertData := "INSERT INTO aether_gazer (gallery_id, title, type, content_img, mobile_content_img1, mobile_content_img2, pc_thumbnail, mobile_thumbnail, sticker_url, creator) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
		_, err := db.Exec(insertData, al.GalleryID, al.Title, al.Type, al.ContentImg, al.MobileContentImg1, al.MobileContentImg2, al.PcThumbnail, al.MobileThumbnail, al.StickerURL, al.Creator)
		if err != nil {
			log.Fatal(err)
		}
	}
	fmt.Println("Worker done and exit")
}

func startCraw(list []Wallpaper) <-chan Wallpaper {
	queue := make(chan Wallpaper, 100)

	go func() {
		for _, v := range list {
			queue <- v
			fmt.Printf("File %s has been enqueued\n", v.Title)
		}
		close(queue)
	}()

	return queue
}

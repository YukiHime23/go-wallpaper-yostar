package crawal

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

const dbPath = "yostar-gallery.db"

func init() {
	var err error
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	createTable := `
		CREATE TABLE IF NOT EXISTS yostar_gallery (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			id_gallery VARCHAR(255) NOT NULL,
			game VARCHAR(255) NOT NULL,
			type VARCHAR(255) NOT NULL,
			file_name VARCHAR(255) NOT NULL,
			url VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`
	_, err = db.Exec(createTable)
	if err != nil {
		db.Close()
		log.Fatalf("failed to create table: %v", err)
	}
	fmt.Println("=======DB created=======")
}

func GetSqliteDb() *sql.DB {
	return db
}

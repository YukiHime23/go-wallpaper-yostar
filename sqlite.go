package crawal

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

const dbPath = "yostar-gallery.db"

// initDB initializes the SQLite database and creates the necessary tables
func InitDB() (*sql.DB, error) {
	// Connect to the SQLite database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Check if the table exists, if not create it
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
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	return db, nil
}

func GetSqliteDb() *sql.DB {
	return db
}

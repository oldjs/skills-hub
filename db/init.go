package db

import (
	"database/sql"
	"log"
	"sync"

	_ "modernc.org/sqlite"
)

var (
	db   *sql.DB
	once sync.Once
)

func Init(dbPath string) error {
	var err error
	once.Do(func() {
		db, err = sql.Open("sqlite", dbPath)
		if err != nil {
			return
		}

		_, err = db.Exec(`
			CREATE TABLE IF NOT EXISTS skills (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				slug TEXT UNIQUE NOT NULL,
				display_name TEXT NOT NULL,
				summary TEXT,
				score REAL DEFAULT 0,
				updated_at INTEGER,
				version TEXT,
				categories TEXT,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)
		`)
		if err != nil {
			return
		}

		_, err = db.Exec(`
			CREATE TABLE IF NOT EXISTS sync_log (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				keywords TEXT,
				count INTEGER
			)
		`)
		if err != nil {
			return
		}

		_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_skills_slug ON skills(slug)`)
		if err != nil {
			return
		}

		_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_skills_display_name ON skills(display_name)`)
		if err != nil {
			return
		}

		log.Println("Database initialized successfully")
	})
	return err
}

func GetDB() *sql.DB {
	return db
}

func Close() {
	if db != nil {
		db.Close()
	}
}

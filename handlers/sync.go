package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"skills-hub/db"
)

func SyncHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Only POST method is allowed",
		})
		return
	}

	go func() {
		log.Println("Manual sync triggered via API")
		db.SetSyncing(true)
		defer db.SetSyncing(false)

		if err := db.SyncFromClawHub(); err != nil {
			log.Printf("Sync failed: %v", err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "started",
		"message":   "Sync started in background",
		"startedAt": time.Now().Format(time.RFC3339),
	})
}

func SyncStatusHandler(w http.ResponseWriter, r *http.Request) {
	database := db.GetDB()
	row := database.QueryRow("SELECT synced_at, count FROM sync_log ORDER BY id DESC LIMIT 1")

	var lastSync *time.Time
	var count int
	err := row.Scan(&lastSync, &count)

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"isSyncing": db.IsSyncing(),
			"lastSync":  nil,
			"count":     0,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"isSyncing": db.IsSyncing(),
		"lastSync":  lastSync.Format(time.RFC3339),
		"count":     count,
	})
}

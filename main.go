package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"skills-hub/db"
	"skills-hub/handlers"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./skills.db"
	}

	if err := db.Init(dbPath); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	handlers.InitTemplates("./templates")

	go startAutoSync(6 * time.Hour)

	if len(os.Args) > 1 && os.Args[1] == "--sync" {
		log.Println("Running initial sync...")
		db.SetSyncing(true)
		defer db.SetSyncing(false)
		if err := db.SyncFromClawHub(); err != nil {
			log.Printf("Initial sync failed: %v", err)
		}
	}

	mux := http.NewServeMux()
	setupRoutes(mux)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("Skills Hub server starting on :%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
}

func setupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", handlers.HomeHandler)
	mux.HandleFunc("/search", handlers.SearchHandler)
	mux.HandleFunc("/skill", handlers.SkillHandler)
	mux.HandleFunc("/sync", handlers.SyncHandler)
	mux.HandleFunc("/api/search", handlers.SearchAPIHandler)
	mux.HandleFunc("/api/sync/status", handlers.SyncStatusHandler)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
}

func startAutoSync(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Println("Auto-sync triggered")
			db.SetSyncing(true)
			if err := db.SyncFromClawHub(); err != nil {
				log.Printf("Auto-sync failed: %v", err)
			}
			db.SetSyncing(false)
		}
	}
}

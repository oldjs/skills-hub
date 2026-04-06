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
	if err := handlers.InitAuth(); err != nil {
		log.Fatalf("Failed to initialize auth: %v", err)
	}
	defer handlers.CloseAuth()

	handlers.InitTemplates("./templates")

	go startAutoSync(6 * time.Hour)

	if len(os.Args) > 1 && os.Args[1] == "--sync" {
		log.Println("Running initial sync for all active tenants...")
		db.SyncAllActiveTenants()
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
	mux.HandleFunc("/login", handlers.UserLogin)
	mux.HandleFunc("/register", handlers.UserRegister)
	mux.HandleFunc("/logout", handlers.UserLogout)
	mux.HandleFunc("/captcha", handlers.CaptchaHandler)
	mux.HandleFunc("/send-code", handlers.SendCodeHandler)
	mux.HandleFunc("/switch-tenant", handlers.RequireAuth(handlers.SwitchTenant))
	mux.HandleFunc("/api/search", handlers.RequireAuth(handlers.SearchAPIHandler))
	mux.HandleFunc("/api/sync/status", handlers.RequireAuth(handlers.SyncStatusHandler))
	mux.HandleFunc("/", handlers.RequireAuth(handlers.HomeHandler))
	mux.HandleFunc("/search", handlers.RequireAuth(handlers.SearchHandler))
	mux.HandleFunc("/skill", handlers.RequireAuth(handlers.SkillHandler))

	mux.HandleFunc("/admin", handlers.RequirePlatformAdmin(handlers.AdminTenantsHandler))
	mux.HandleFunc("/admin/tenant", handlers.RequirePlatformAdmin(handlers.AdminTenantDetailHandler))
	mux.HandleFunc("/admin/tenant/create", handlers.RequirePlatformAdmin(handlers.AdminTenantCreateHandler))
	mux.HandleFunc("/admin/tenant/update", handlers.RequirePlatformAdmin(handlers.AdminTenantUpdateHandler))
	mux.HandleFunc("/admin/tenant/invite", handlers.RequirePlatformAdmin(handlers.AdminTenantInviteHandler))
	mux.HandleFunc("/admin/tenant/invite/revoke", handlers.RequirePlatformAdmin(handlers.AdminTenantInviteRevokeHandler))
	mux.HandleFunc("/admin/tenant/member/update", handlers.RequirePlatformAdmin(handlers.AdminTenantMemberUpdateHandler))
	mux.HandleFunc("/admin/tenant/member/remove", handlers.RequirePlatformAdmin(handlers.AdminTenantMemberRemoveHandler))
	mux.HandleFunc("/admin/tenant/sync", handlers.RequirePlatformAdmin(handlers.AdminTenantSyncHandler))

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
}

func startAutoSync(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Println("Auto-sync triggered")
			db.SyncAllActiveTenants()
		}
	}
}

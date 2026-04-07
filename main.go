package main

import (
	"context"
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
		Handler:      handlers.SecurityHeaders(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown failed: %v", err)
	}
}

func setupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/login", handlers.UserLogin)
	mux.HandleFunc("/register", handlers.UserRegister)
	mux.HandleFunc("/logout", handlers.UserLogout)
	mux.HandleFunc("/captcha", handlers.CaptchaHandler)
	mux.HandleFunc("/send-code", handlers.SendCodeHandler)
	mux.HandleFunc("/switch-tenant", handlers.RequireAuth(handlers.SwitchTenant))
	mux.HandleFunc("/api/v1/search", handlers.APIV1SearchHandler)
	mux.HandleFunc("/api/v1/skills/", handlers.APIV1SkillDetailHandler)
	mux.HandleFunc("/api/v1/download/", handlers.APIV1DownloadHandler)
	mux.HandleFunc("/api/v1/categories", handlers.APIV1CategoriesHandler)
	mux.HandleFunc("/api/v1/stats", handlers.APIV1StatsHandler)
	mux.HandleFunc("/api/search", handlers.RequireAuth(handlers.SearchAPIHandler))
	mux.HandleFunc("/api/sync/status", handlers.RequireAuth(handlers.SyncStatusHandler))
	mux.HandleFunc("/", handlers.RequireAuth(handlers.HomeHandler))
	mux.HandleFunc("/search", handlers.RequireAuth(handlers.SearchHandler))
	mux.HandleFunc("/skill", handlers.RequireAuth(handlers.SkillHandler))
	mux.HandleFunc("/upload", handlers.RequireAuth(handlers.UploadHandler))
	mux.HandleFunc("/api/rate", handlers.RequireAuth(handlers.RateSkillHandler))
	mux.HandleFunc("/api/comment", handlers.RequireAuth(handlers.CommentSkillHandler))
	mux.HandleFunc("/api/markdown/preview", handlers.RequireAuth(handlers.MarkdownPreviewHandler))

	mux.HandleFunc("/admin", handlers.RequirePlatformAdmin(handlers.AdminDashboardHandler))
	mux.HandleFunc("/admin/skills", handlers.RequirePlatformAdmin(handlers.AdminSkillsHandler))
	mux.HandleFunc("/admin/skill", handlers.RequirePlatformAdmin(handlers.AdminSkillDetailHandler))
	mux.HandleFunc("/admin/skill/update", handlers.RequirePlatformAdmin(handlers.AdminSkillUpdateHandler))
	mux.HandleFunc("/admin/skill/review", handlers.RequirePlatformAdmin(handlers.AdminSkillReviewHandler))
	mux.HandleFunc("/admin/comments", handlers.RequirePlatformAdmin(handlers.AdminCommentsHandler))
	mux.HandleFunc("/admin/comment/delete", handlers.RequirePlatformAdmin(handlers.AdminCommentDeleteHandler))
	mux.HandleFunc("/admin/users", handlers.RequirePlatformAdmin(handlers.AdminUsersHandler))
	mux.HandleFunc("/admin/user/update", handlers.RequirePlatformAdmin(handlers.AdminUserUpdateHandler))
	mux.HandleFunc("/admin/tenants", handlers.RequirePlatformAdmin(handlers.AdminTenantsHandler))
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

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
	// 搜索结果缓存共用 auth 的 Redis 客户端
	db.SetCacheClient(handlers.GetRedisClient())

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
		Handler:      handlers.RequestLogger(handlers.SecurityHeaders(mux)),
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
	mux.HandleFunc("/healthz", handlers.HealthCheckHandler)
	mux.HandleFunc("/sitemap.xml", handlers.SitemapHandler)
	mux.HandleFunc("/robots.txt", handlers.RobotsTxtHandler)
	mux.HandleFunc("/login", handlers.UserLogin)
	mux.HandleFunc("/register", handlers.UserRegister)
	mux.HandleFunc("/logout", handlers.UserLogout)
	mux.HandleFunc("/captcha", handlers.CaptchaHandler)
	mux.HandleFunc("/send-code", handlers.SendCodeHandler)
	mux.HandleFunc("/switch-tenant", handlers.RequireAuth(handlers.SwitchTenant))
	mux.HandleFunc("/account", handlers.RequireAuth(handlers.AccountHandler))
	mux.HandleFunc("/account/api-keys/create", handlers.RequireAuth(handlers.AccountCreateAPIKeyHandler))
	mux.HandleFunc("/account/api-keys/revoke", handlers.RequireAuth(handlers.AccountRevokeAPIKeyHandler))
	mux.HandleFunc("/api/v1/search", handlers.APIV1SearchHandler)
	mux.HandleFunc("/api/v1/skills/", handlers.APIV1SkillDetailHandler)
	mux.HandleFunc("/api/v1/download/", handlers.APIV1DownloadHandler)
	mux.HandleFunc("/api/v1/upload", handlers.APIV1UploadHandler)
	mux.HandleFunc("/api/v1/categories", handlers.APIV1CategoriesHandler)
	mux.HandleFunc("/api/v1/stats", handlers.APIV1StatsHandler)
	mux.HandleFunc("/api/v1/openapi.json", handlers.OpenAPISpecHandler)
	mux.HandleFunc("/api/v1/docs", handlers.OpenAPIDocsHandler)
	mux.HandleFunc("/api/search", handlers.RequireAuth(handlers.SearchAPIHandler))
	mux.HandleFunc("/api/sync/status", handlers.RequireAuth(handlers.SyncStatusHandler))
	mux.HandleFunc("/", handlers.RequireAuth(handlers.HomeHandler))
	mux.HandleFunc("/search", handlers.OptionalAuth(handlers.SearchHandler))
	mux.HandleFunc("/leaderboard", handlers.OptionalAuth(handlers.LeaderboardHandler))
	mux.HandleFunc("/skill", handlers.OptionalAuth(handlers.SkillHandler))
	mux.HandleFunc("/user", handlers.OptionalAuth(handlers.ProfileHandler))
	mux.HandleFunc("/account/profile", handlers.RequireAuth(handlers.AccountProfileUpdateHandler))
	mux.HandleFunc("/upload", handlers.RequireAuth(handlers.UploadHandler))
	mux.HandleFunc("/api/upload/preview", handlers.RequireAuth(handlers.UploadPreviewHandler))
	mux.HandleFunc("/api/bookmark", handlers.RequireAuth(handlers.BookmarkToggleHandler))
	mux.HandleFunc("/api/notifications", handlers.RequireAuth(handlers.NotificationsAPIHandler))
	mux.HandleFunc("/api/notifications/read", handlers.RequireAuth(handlers.NotificationReadHandler))
	mux.HandleFunc("/api/notifications/read-all", handlers.RequireAuth(handlers.NotificationReadAllHandler))
	mux.HandleFunc("/account/bookmarks", handlers.RequireAuth(handlers.AccountBookmarksHandler))
	mux.HandleFunc("/collections", handlers.RequireAuth(handlers.CollectionListHandler))
	mux.HandleFunc("/collections/create", handlers.RequireAuth(handlers.CollectionCreateHandler))
	mux.HandleFunc("/collections/add-skill", handlers.RequireAuth(handlers.CollectionAddSkillHandler))
	mux.HandleFunc("/collections/delete", handlers.RequireAuth(handlers.CollectionDeleteHandler))
	mux.HandleFunc("/api/rate", handlers.RequireAuth(handlers.RateSkillHandler))
	mux.HandleFunc("/api/comment", handlers.RequireAuth(handlers.CommentSkillHandler))
	mux.HandleFunc("/api/comment/vote", handlers.RequireAuth(handlers.CommentVoteHandler))
	mux.HandleFunc("/api/markdown/preview", handlers.RequireAuth(handlers.MarkdownPreviewHandler))

	mux.HandleFunc("/admin", handlers.RequireAdmin(handlers.AdminDashboardHandler))
	mux.HandleFunc("/admin/skills", handlers.RequireAdmin(handlers.AdminSkillsHandler))
	mux.HandleFunc("/admin/skill", handlers.RequireAdmin(handlers.AdminSkillDetailHandler))
	mux.HandleFunc("/admin/skill/update", handlers.RequireAdmin(handlers.AdminSkillUpdateHandler))
	mux.HandleFunc("/admin/skill/review", handlers.RequireAdmin(handlers.AdminSkillReviewHandler))
	mux.HandleFunc("/admin/skill/rollback", handlers.RequireAdmin(handlers.AdminSkillRollbackHandler))
	mux.HandleFunc("/admin/skills/batch-review", handlers.RequireAdmin(handlers.AdminBatchReviewHandler))
	mux.HandleFunc("/admin/logs/export", handlers.RequirePlatformAdmin(handlers.AdminLogsExportHandler))
	mux.HandleFunc("/admin/comments", handlers.RequireAdmin(handlers.AdminCommentsHandler))
	mux.HandleFunc("/admin/comment/delete", handlers.RequireAdmin(handlers.AdminCommentDeleteHandler))
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
	mux.HandleFunc("/admin/email-templates", handlers.RequirePlatformAdmin(handlers.AdminEmailTemplatesHandler))
	mux.HandleFunc("/admin/email-templates/update", handlers.RequirePlatformAdmin(handlers.AdminEmailTemplateUpdateHandler))
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

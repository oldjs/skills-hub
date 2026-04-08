package handlers

import (
	"log/slog"
	"net/http"

	"skills-hub/db"
)

func renderAdminPage(w http.ResponseWriter, r *http.Request, name string, data PageData) {
	stats, err := db.GetAdminDashboardStats()
	if err != nil {
		slog.Error("admin dashboard stats load failed", "error", err)
		http.Error(w, "后台数据加载失败", http.StatusInternalServerError)
		return
	}
	data.CurrentPage = "admin"
	data.AdminStats = stats
	RenderTemplate(w, r, name, data)
}

func adminActorID(r *http.Request) int64 {
	sess := GetCurrentSession(r)
	if sess == nil {
		return 0
	}
	return sess.UserID
}

func recordAdminAction(r *http.Request, action, targetType string, targetID int64, details string) {
	userID := adminActorID(r)
	if userID == 0 {
		return
	}
	if err := db.LogAdminAction(userID, action, targetType, targetID, details); err != nil {
		slog.Error("admin action log failed", "error", err)
	}
}

package handlers

import (
	"net/http"

	"skills-hub/db"
)

func AdminDashboardHandler(w http.ResponseWriter, r *http.Request) {
	stats, err := db.GetAdminDashboardStats()
	if err != nil {
		http.Error(w, "统计数据加载失败", http.StatusInternalServerError)
		return
	}

	logs, err := db.ListAdminActionLogs(12)
	if err != nil {
		http.Error(w, "操作日志加载失败", http.StatusInternalServerError)
		return
	}

	pendingSkills, err := db.ListAdminSkills("pending")
	if err != nil {
		http.Error(w, "待审核 Skills 加载失败", http.StatusInternalServerError)
		return
	}
	if len(pendingSkills) > 6 {
		pendingSkills = pendingSkills[:6]
	}

	renderAdminPage(w, r, "admin_dashboard.html", PageData{
		Title:        "管理员后台 - Skills Hub",
		AdminSection: "dashboard",
		AdminStats:   stats,
		AdminLogs:    logs,
		AdminSkills:  pendingSkills,
		Info:         r.URL.Query().Get("info"),
		Error:        r.URL.Query().Get("error"),
	})
}

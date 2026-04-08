package handlers

import (
	"encoding/json"
	"log/slog"
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

	// 图表数据
	regData, _ := db.GetDailyRegistrations(30)
	skillData, _ := db.GetDailySkillGrowth(30)
	activeUsers, _ := db.GetMostActiveUsers(7, 5)

	regJSON, _ := json.Marshal(regData)
	skillJSON, _ := json.Marshal(skillData)

	if regData == nil {
		regJSON = []byte("[]")
	}
	if skillData == nil {
		skillJSON = []byte("[]")
	}

	// 走 renderAdminPage 的基础数据
	base := PageData{
		Title:        "管理员后台 - Skills Hub",
		AdminSection: "dashboard",
		AdminStats:   stats,
		AdminLogs:    logs,
		AdminSkills:  pendingSkills,
		Info:         r.URL.Query().Get("info"),
		Error:        r.URL.Query().Get("error"),
	}

	// 额外图表数据通过 renderAdminPageWithExtra 传入
	extra := map[string]interface{}{
		"RegChartData":   string(regJSON),
		"SkillChartData": string(skillJSON),
		"ActiveUsers":    activeUsers,
	}

	renderAdminPageWithExtra(w, r, "admin_dashboard.html", base, extra)
}

// renderAdminPageWithExtra 在 renderAdminPage 的基础上加额外数据
func renderAdminPageWithExtra(w http.ResponseWriter, r *http.Request, name string, data PageData, extra map[string]interface{}) {
	stats, err := db.GetAdminDashboardStats()
	if err != nil {
		slog.Error("admin dashboard stats load failed", "error", err)
	} else if data.AdminStats == nil {
		data.AdminStats = stats
	}

	if data.AdminSection == "" {
		data.AdminSection = "dashboard"
	}
	if data.AdminStats != nil {
		// AdminStats already populated
	}

	// 把 PageData + extra 合并后传给 RenderTemplate
	merged := make(map[string]interface{})
	// PageData 先走标准 merge
	mergedData := data
	_ = mergedData // 由 RenderTemplate 内部处理

	// 用 map 模式调用 RenderTemplate，extra 字段会被合并
	RenderTemplate(w, r, name, map[string]interface{}{
		"Title":              data.Title,
		"AdminSection":       data.AdminSection,
		"AdminStats":         data.AdminStats,
		"PendingReviewCount": data.AdminStats.PendingSkills,
		"AdminLogs":          data.AdminLogs,
		"AdminSkills":        data.AdminSkills,
		"Info":               data.Info,
		"Error":              data.Error,
		"RegChartData":       extra["RegChartData"],
		"SkillChartData":     extra["SkillChartData"],
		"ActiveUsers":        extra["ActiveUsers"],
	})
	_ = merged
}

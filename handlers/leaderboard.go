package handlers

import (
	"log/slog"
	"net/http"

	"skills-hub/db"
	"skills-hub/models"
)

// GET /leaderboard — 技能排行榜
func LeaderboardHandler(w http.ResponseWriter, r *http.Request) {
	sess := GetCurrentSession(r)
	tenantID := resolveViewTenantID(sess)

	// 没有可用租户时展示空排行榜
	if tenantID == 0 {
		RenderTemplate(w, r, "leaderboard.html", map[string]interface{}{
			"Title":        "技能排行榜 - Skills Hub",
			"CurrentPage":  "leaderboard",
			"TopSkills":    []models.Skill{},
			"ActiveSkills": []models.Skill{},
			"Info":         "暂无数据，请先创建租户或同步技能。",
		})
		return
	}

	// 热门榜
	topSkills, err := db.GetSkillLeaderboard(tenantID, 10)
	if err != nil {
		slog.Error("leaderboard top skills failed", "error", err)
		topSkills = []models.Skill{}
	}

	// 近期活跃榜
	activeSkills, err := db.GetRecentlyActiveSkills(tenantID, 10)
	if err != nil {
		slog.Error("leaderboard active skills failed", "error", err)
		activeSkills = []models.Skill{}
	}

	data := PageData{
		Title:           "技能排行榜 - Skills Hub",
		MetaDescription: "Skills Hub 技能排行榜，按评分和活跃度排名的热门 OpenClaw 技能。",
		CanonicalURL:    canonicalURL("/leaderboard"),
		CurrentPage:     "leaderboard",
	}

	RenderTemplate(w, r, "leaderboard.html", map[string]interface{}{
		"Title":           data.Title,
		"MetaDescription": data.MetaDescription,
		"CanonicalURL":    data.CanonicalURL,
		"CurrentPage":     data.CurrentPage,
		"TopSkills":       topSkills,
		"ActiveSkills":    activeSkills,
	})
}

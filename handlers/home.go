package handlers

import (
	"log/slog"
	"net/http"

	"skills-hub/db"
	"skills-hub/models"
)

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/index.html" {
		RenderNotFound(w, r)
		return
	}

	sess := GetCurrentSession(r)
	if sess == nil || sess.CurrentTenantID == 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	pageError := ""
	page, perPage := parsePaginationParams(r)

	skills, totalSkills, currentPage, err := db.GetFilteredSkillsPage(sess.CurrentTenantID, "", "", "", page, perPage)
	if err != nil {
		slog.Error("home skills load failed", "error", err)
		skills = []models.Skill{}
		totalSkills = 0
		currentPage = 1
		pageError = "技能数据加载失败，当前页面展示可能不完整"
	}

	categories, err := db.GetCategories(sess.CurrentTenantID)
	if err != nil {
		slog.Error("home categories load failed", "error", err)
		categories = []string{}
		if pageError == "" {
			pageError = "分类数据加载失败，当前页面展示可能不完整"
		}
	}

	data := PageData{
		Title:           "Skills Hub - OpenClaw 技能中心",
		MetaDescription: "Skills Hub 是 OpenClaw 智能体技能市场，探索、发现并安装全球开发者构建的优质 AI Skills。",
		MetaKeywords:    "OpenClaw,Skills,AI,智能体,技能市场,插件,自动化",
		CanonicalURL:    canonicalURL("/"),
		Skills:          skills,
		Categories:      categories,
		CurrentPage:     "home",
		Pagination:      NewPaginationData(currentPage, perPage, totalSkills),
		TotalSkills:     totalSkills,
		CategoryCount:   len(categories),
		Error:           pageError,
	}

	RenderTemplate(w, r, "index.html", data)
}

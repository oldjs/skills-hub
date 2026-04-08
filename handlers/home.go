package handlers

import (
	"fmt"
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
	tenantID := resolveViewTenantID(sess)

	// 没有可用租户时展示空首页，不报 500
	if tenantID == 0 {
		RenderTemplate(w, r, "index.html", PageData{
			Title:           "Skills Hub - OpenClaw 技能中心",
			MetaDescription: "Skills Hub 是 OpenClaw 智能体技能市场，探索、发现并安装全球开发者构建的优质 AI Skills。",
			MetaKeywords:    "OpenClaw,Skills,AI,智能体,技能市场,插件,自动化",
			CanonicalURL:    canonicalURL("/"),
			Skills:          []models.Skill{},
			CurrentPage:     "home",
			Pagination:      NewPaginationData(1, defaultPerPage, 0),
			Info:            "暂无数据，请先创建租户或同步技能。",
		})
		return
	}

	// 未登录访客给一层短缓存，利于 SEO 爬虫
	if sess == nil {
		setSearchCacheHeaders(w)
	}

	pageError := ""
	page, perPage := parsePaginationParams(r)

	// 首页热门查询走 Redis 缓存
	cacheTag := fmt.Sprintf("home:%d:p%d", tenantID, page)
	skills, totalSkills, hit := db.GetCachedSkills(tenantID, cacheTag)
	currentPage := page
	if !hit {
		var err error
		skills, totalSkills, currentPage, err = db.GetFilteredSkillsPage(tenantID, "", "", "", page, perPage)
		if err != nil {
			slog.Error("home skills load failed", "error", err)
			skills = []models.Skill{}
			totalSkills = 0
			currentPage = 1
			pageError = "技能数据加载失败，当前页面展示可能不完整"
		} else {
			db.SetCachedSkills(cacheTag, skills, totalSkills)
		}
	}

	categories, err := db.GetCategories(tenantID)
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

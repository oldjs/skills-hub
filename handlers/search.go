package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"skills-hub/db"
	"skills-hub/models"
)

func SearchHandler(w http.ResponseWriter, r *http.Request) {
	sess := GetCurrentSession(r)
	tenantID := resolveViewTenantID(sess)
	if tenantID == 0 {
		RenderServerError(w, r)
		return
	}
	isAdmin := sess != nil && (sess.IsPlatformAdmin || sess.IsSubAdmin)
	if !enforceRateLimit(w, "search-ip:"+GetClientIP(r), 30, isAdmin) {
		return
	}

	query := r.URL.Query().Get("q")
	category := r.URL.Query().Get("category")
	sortBy := r.URL.Query().Get("sort")
	author := strings.TrimSpace(r.URL.Query().Get("author"))
	source := strings.TrimSpace(r.URL.Query().Get("source"))
	dateRange := strings.TrimSpace(r.URL.Query().Get("date"))
	minRatingStr := strings.TrimSpace(r.URL.Query().Get("min_rating"))
	page, perPage := parsePaginationParams(r)

	// 解析最低评分
	var minRating float64
	if minRatingStr != "" {
		if v, err := strconv.ParseFloat(minRatingStr, 64); err == nil && v > 0 {
			minRating = v
		}
	}

	searchParams := db.AdvancedSearchParams{
		Query:     query,
		Category:  category,
		MinRating: minRating,
		DateRange: dateRange,
		Author:    author,
		Source:    source,
	}

	title := "搜索 Skills - Skills Hub"
	if query != "" && category != "" {
		title = "搜索: " + query + " / " + category + " - Skills Hub"
	} else if query != "" {
		title = "搜索: " + query + " - Skills Hub"
	} else if category != "" {
		title = category + " - Skills Hub"
	}

	if strings.Contains(r.Header.Get("Accept"), "application/json") || r.URL.Query().Get("format") == "json" {
		w.Header().Set("Content-Type", "application/json")
		skills, totalSkills, currentPage, err := db.GetFilteredSkillsPageAdvanced(tenantID, searchParams, sortBy, page, perPage)
		if err != nil {
			slog.Error("search json failed", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"skills": []models.Skill{}, "total": 0, "error": "搜索失败",
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"skills": skills, "query": query, "category": category,
			"sort": sortBy, "page": currentPage, "per_page": perPage, "total": totalSkills,
		})
		return
	}

	pageError := ""
	skills, totalSkills, currentPage, err := db.GetFilteredSkillsPageAdvanced(tenantID, searchParams, sortBy, page, perPage)

	if err != nil {
		slog.Error("search page failed", "error", err)
		skills = []models.Skill{}
		totalSkills = 0
		currentPage = 1
		pageError = "搜索服务暂时不可用，已为你展示空结果"
	}

	categories, err := db.GetCategories(tenantID)
	if err != nil {
		slog.Error("search categories failed", "error", err)
		categories = []string{}
		if pageError == "" {
			pageError = "筛选分类加载失败，请稍后刷新重试"
		}
	}

	// 未登录用户的搜索结果允许短缓存
	if sess == nil {
		setSearchCacheHeaders(w)
	}

	metaDesc := "在 Skills Hub 中搜索 OpenClaw 技能"
	if query != "" {
		metaDesc = "搜索结果: " + query + " - Skills Hub"
	}

	data := PageData{
		Title:           title,
		MetaDescription: metaDesc,
		MetaKeywords:    "OpenClaw,Skills,搜索,技能," + query,
		CanonicalURL:    canonicalURL("/search"),
		Skills:          skills,
		Categories:      categories,
		Query:           query,
		Category:        category,
		SortBy:          sortBy,
		AuthorFilter:    author,
		SourceFilter:    source,
		DateFilter:      dateRange,
		MinRating:       minRatingStr,
		CurrentPage:     "search",
		Pagination:      NewPaginationData(currentPage, perPage, totalSkills),
		TotalSkills:     totalSkills,
		CategoryCount:   len(categories),
		Error:           pageError,
	}

	RenderTemplate(w, r, "search.html", data)
}

func SearchAPIHandler(w http.ResponseWriter, r *http.Request) {
	sess := GetCurrentSession(r)
	if sess == nil || sess.CurrentTenantID == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if !enforceRateLimit(w, "search-ip:"+GetClientIP(r), 30, sess.IsPlatformAdmin || sess.IsSubAdmin) {
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"skills": []models.Skill{},
		})
		return
	}

	skills, err := db.SearchSkills(sess.CurrentTenantID, query)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"skills": []models.Skill{},
		})
		return
	}
	if len(skills) > 8 {
		skills = skills[:8]
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"skills": skills,
	})
}

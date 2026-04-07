package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"skills-hub/db"
	"skills-hub/models"
)

func SearchHandler(w http.ResponseWriter, r *http.Request) {
	sess := GetCurrentSession(r)
	if sess == nil || sess.CurrentTenantID == 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !enforceRateLimit(w, "search-ip:"+GetClientIP(r), 30, sess.IsPlatformAdmin || sess.IsSubAdmin) {
		return
	}

	query := r.URL.Query().Get("q")
	category := r.URL.Query().Get("category")
	sortBy := r.URL.Query().Get("sort") // score, rating, latest
	page, perPage := parsePaginationParams(r)
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

		skills, totalSkills, currentPage, err := db.GetFilteredSkillsPage(sess.CurrentTenantID, query, category, sortBy, page, perPage)

		if err != nil {
			log.Printf("search json failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"skills":   []models.Skill{},
				"query":    query,
				"category": category,
				"sort":     sortBy,
				"page":     1,
				"per_page": perPage,
				"total":    0,
				"error":    "搜索失败，请稍后重试",
			})
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"skills":   skills,
			"query":    query,
			"category": category,
			"sort":     sortBy,
			"page":     currentPage,
			"per_page": perPage,
			"total":    totalSkills,
		})
		return
	}

	pageError := ""
	skills, totalSkills, currentPage, err := db.GetFilteredSkillsPage(sess.CurrentTenantID, query, category, sortBy, page, perPage)

	if err != nil {
		log.Printf("search page failed: %v", err)
		skills = []models.Skill{}
		totalSkills = 0
		currentPage = 1
		pageError = "搜索服务暂时不可用，已为你展示空结果"
	}

	categories, err := db.GetCategories(sess.CurrentTenantID)
	if err != nil {
		log.Printf("search categories failed: %v", err)
		categories = []string{}
		if pageError == "" {
			pageError = "筛选分类加载失败，请稍后刷新重试"
		}
	}

	data := PageData{
		Title:         title,
		Skills:        skills,
		Categories:    categories,
		Query:         query,
		Category:      category,
		SortBy:        sortBy,
		CurrentPage:   "search",
		Pagination:    NewPaginationData(currentPage, perPage, totalSkills),
		TotalSkills:   totalSkills,
		CategoryCount: len(categories),
		Error:         pageError,
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

package handlers

import (
	"log"
	"net/http"

	"skills-hub/db"
	"skills-hub/models"
)

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/index.html" {
		http.NotFound(w, r)
		return
	}

	sess := GetCurrentSession(r)
	if sess == nil || sess.CurrentTenantID == 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	pageError := ""

	skills, err := db.GetAllSkills(sess.CurrentTenantID)
	if err != nil {
		log.Printf("home skills load failed: %v", err)
		skills = []models.Skill{}
		pageError = "技能数据加载失败，当前页面展示可能不完整"
	}

	featuredSkills := skills
	if len(featuredSkills) > 12 {
		featuredSkills = featuredSkills[:12]
	}

	categories, err := db.GetCategories(sess.CurrentTenantID)
	if err != nil {
		log.Printf("home categories load failed: %v", err)
		categories = []string{}
		if pageError == "" {
			pageError = "分类数据加载失败，当前页面展示可能不完整"
		}
	}

	data := PageData{
		Title:         "Skills Hub - OpenClaw 技能中心",
		Skills:        featuredSkills,
		Categories:    categories,
		CurrentPage:   "home",
		TotalSkills:   len(skills),
		CategoryCount: len(categories),
		Error:         pageError,
	}

	RenderTemplate(w, r, "index.html", data)
}

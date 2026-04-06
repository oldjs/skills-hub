package handlers

import (
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

	skills, err := db.GetAllSkills(sess.CurrentTenantID)
	if err != nil {
		skills = []models.Skill{}
	}

	featuredSkills := skills
	if len(featuredSkills) > 12 {
		featuredSkills = featuredSkills[:12]
	}

	categories, err := db.GetCategories(sess.CurrentTenantID)
	if err != nil {
		categories = []string{}
	}

	data := PageData{
		Title:         "Skills Hub - OpenClaw 技能中心",
		Skills:        featuredSkills,
		Categories:    categories,
		CurrentPage:   "home",
		TotalSkills:   len(skills),
		CategoryCount: len(categories),
	}

	RenderTemplate(w, r, "index.html", data)
}

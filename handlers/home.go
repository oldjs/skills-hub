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

	skills, err := db.GetAllSkills()
	if err != nil {
		skills = []models.Skill{}
	}

	categories, err := db.GetCategories()
	if err != nil {
		categories = []string{}
	}

	data := PageData{
		Title:       "Skills Hub - OpenClaw 技能中心",
		Skills:      skills,
		Categories:  categories,
		CurrentPage: "home",
	}

	RenderTemplate(w, "index.html", data)
}

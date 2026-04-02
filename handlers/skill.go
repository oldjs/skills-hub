package handlers

import (
	"net/http"

	"skills-hub/db"
)

func SkillHandler(w http.ResponseWriter, r *http.Request) {
	slug := r.URL.Query().Get("slug")
	if slug == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	skill, err := db.GetSkillBySlug(slug)
	if err != nil || skill == nil {
		http.NotFound(w, r)
		return
	}

	categories, _ := db.GetCategories()

	data := PageData{
		Title:       skill.DisplayName + " - Skills Hub",
		Skill:       skill,
		Categories:  categories,
		CurrentPage: "skill",
	}

	RenderTemplate(w, "skill.html", data)
}

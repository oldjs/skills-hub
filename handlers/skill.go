package handlers

import (
	"net/http"

	"skills-hub/db"
)

func SkillHandler(w http.ResponseWriter, r *http.Request) {
	sess := GetCurrentSession(r)
	if sess == nil || sess.CurrentTenantID == 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	slug := r.URL.Query().Get("slug")
	if slug == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	skill, err := db.GetSkillBySlug(sess.CurrentTenantID, slug)
	if err != nil || skill == nil {
		http.NotFound(w, r)
		return
	}

	categories, _ := db.GetCategories(sess.CurrentTenantID)

	data := PageData{
		Title:       skill.DisplayName + " - Skills Hub",
		Skill:       skill,
		Categories:  categories,
		CurrentPage: "skill",
	}

	RenderTemplate(w, r, "skill.html", data)
}

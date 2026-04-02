package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"skills-hub/db"
	"skills-hub/models"
)

func SearchHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	category := r.URL.Query().Get("category")
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

		skills, err := db.GetFilteredSkills(query, category)

		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"skills": []models.Skill{},
				"error":  err.Error(),
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"skills":   skills,
			"query":    query,
			"category": category,
		})
		return
	}

	skills, err := db.GetFilteredSkills(query, category)

	if err != nil {
		skills = []models.Skill{}
	}

	categories, _ := db.GetCategories()

	data := PageData{
		Title:         title,
		Skills:        skills,
		Categories:    categories,
		Query:         query,
		Category:      category,
		CurrentPage:   "search",
		TotalSkills:   len(skills),
		CategoryCount: len(categories),
	}

	RenderTemplate(w, "search.html", data)
}

func SearchAPIHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"skills": []models.Skill{},
		})
		return
	}

	skills, err := db.SearchSkills(query)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"skills": []models.Skill{},
		})
		return
	}
	if len(skills) > 8 {
		skills = skills[:8]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"skills": skills,
	})
}

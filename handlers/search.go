package handlers

import (
	"encoding/json"
	"net/http"

	"skills-hub/db"
)

func SearchHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	category := r.URL.Query().Get("category")

	if r.Header.Get("Accept") == "application/json" || r.URL.Query().Get("format") == "json" {
		w.Header().Set("Content-Type", "application/json")
		
		var skills interface{}
		var err error

		if category != "" {
			skills, err = db.GetSkillsByCategory(category)
		} else if query != "" {
			skills, err = db.SearchSkills(query)
		} else {
			skills, err = db.GetAllSkills()
		}

		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"skills": []interface{}{},
				"error":  err.Error(),
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"skills": skills,
		})
		return
	}

	var skills interface{}
	var err error

	if category != "" {
		skills, err = db.GetSkillsByCategory(category)
	} else if query != "" {
		skills, err = db.SearchSkills(query)
	} else {
		skills, err = db.GetAllSkills()
	}

	if err != nil {
		skills = []interface{}{}
	}

	categories, _ := db.GetCategories()

	data := PageData{
		Title:       "搜索: " + query + " - Skills Hub",
		Skills:      skills,
		Categories:  categories,
		Query:       query,
		CurrentPage: "search",
	}

	RenderTemplate(w, "search.html", data)
}

func SearchAPIHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	
	skills, err := db.SearchSkills(query)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []interface{}{},
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"results": skills,
	})
}

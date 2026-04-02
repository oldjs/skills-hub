package handlers

import (
	"bytes"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"skills-hub/models"
)

var templates map[string]*template.Template

func InitTemplates(templateDir string) {
	templates = make(map[string]*template.Template)

	funcMap := template.FuncMap{
		"split":      strings.Split,
		"categories": categoryList,
		"formatDate": formatDate,
	}

	basePath := filepath.Join(templateDir, "layout.html")

	pages := []string{"index.html", "search.html", "skill.html"}
	for _, page := range pages {
		files := []string{basePath, filepath.Join(templateDir, page)}
		t, err := template.New("layout.html").Funcs(funcMap).ParseFiles(files...)
		if err != nil {
			log.Fatalf("Failed to parse template %s: %v", page, err)
		}
		templates[page] = t
	}

	log.Println("Templates initialized")
}

func RenderTemplate(w http.ResponseWriter, name string, data interface{}) {
	t, ok := templates[name]
	if !ok {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", data); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, "Template render failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

type PageData struct {
	Title         string
	Skills        []models.Skill
	Categories    []string
	Query         string
	Category      string
	CurrentPage   string
	Skill         *models.Skill
	TotalSkills   int
	CategoryCount int
}

func categoryList(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02")
}

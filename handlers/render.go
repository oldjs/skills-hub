package handlers

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"
)

var templates map[string]*template.Template

func InitTemplates(templateDir string) {
	templates = make(map[string]*template.Template)

	funcMap := template.FuncMap{
		"split": strings.Split,
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
	
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		log.Printf("Template execution error: %v", err)
	}
}

type PageData struct {
	Title       string
	Skills      interface{}
	Categories  interface{}
	Query       string
	CurrentPage string
	Skill       interface{}
}

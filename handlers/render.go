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
	pages := []string{"index.html", "search.html", "skill.html", "login.html", "register.html", "admin_tenants.html", "admin_tenant_detail.html"}

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

func RenderTemplate(w http.ResponseWriter, r *http.Request, name string, data interface{}) {
	t, ok := templates[name]
	if !ok {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	csrfToken, err := getOrCreateCSRFToken(w, r)
	if err != nil {
		log.Printf("CSRF token error: %v", err)
		http.Error(w, "Template render failed", http.StatusInternalServerError)
		return
	}

	pageData := map[string]interface{}{
		"CSRFToken":  csrfToken,
		"IsLoggedIn": false,
	}

	sess := getSession(r)
	if sess != nil {
		ctx, err := buildRequestContext(sess.UserID, sess)
		if err != nil {
			log.Printf("build request context error: %v", err)
			http.Error(w, "Template render failed", http.StatusInternalServerError)
			return
		}
		pageData["IsLoggedIn"] = true
		pageData["Session"] = sess
		pageData["CurrentUser"] = ctx.User
		pageData["TenantOptions"] = ctx.TenantOptions
		pageData["CurrentTenant"] = ctx.CurrentTenant
		pageData["IsPlatformAdmin"] = ctx.User != nil && ctx.User.IsPlatformAdmin
	}

	mergeTemplateData(pageData, data)

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", pageData); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, "Template render failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

func mergeTemplateData(target map[string]interface{}, data interface{}) {
	switch value := data.(type) {
	case nil:
		return
	case map[string]interface{}:
		for key, item := range value {
			target[key] = item
		}
	case PageData:
		mergePageData(target, value)
	case *PageData:
		if value != nil {
			mergePageData(target, *value)
		}
	}
}

func mergePageData(target map[string]interface{}, data PageData) {
	target["Title"] = data.Title
	target["Skills"] = data.Skills
	target["Categories"] = data.Categories
	target["Query"] = data.Query
	target["Category"] = data.Category
	target["CurrentPage"] = data.CurrentPage
	target["Skill"] = data.Skill
	target["TotalSkills"] = data.TotalSkills
	target["CategoryCount"] = data.CategoryCount
	target["Error"] = data.Error
	target["Info"] = data.Info
	if data.Tenant != nil {
		target["Tenant"] = data.Tenant
	}
	if data.Tenants != nil {
		target["Tenants"] = data.Tenants
	}
	if data.Members != nil {
		target["Members"] = data.Members
	}
	if data.Invites != nil {
		target["Invites"] = data.Invites
	}
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
	Error         string
	Info          string
	Tenant        *models.Tenant
	Tenants       []models.Tenant
	Members       []models.TenantMember
	Invites       []models.TenantInvite
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

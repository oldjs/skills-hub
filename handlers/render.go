package handlers

import (
	"bytes"
	"fmt"
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
		"split":         strings.Split,
		"categories":    categoryList,
		"formatDate":    formatDate,
		"maskEmail":     maskEmail,
		"formatTime":    formatTime,
		"formatTimePtr": formatTimePtr,
		"firstChar":     firstChar,
		"seq":           seq,
		"parseInt":      parseIntFromStr,
		"roundRating":   roundRating,
	}
	basePath := filepath.Join(templateDir, "layout.html")
	adminShellPath := filepath.Join(templateDir, "admin_shell.html")
	pages := []string{"index.html", "search.html", "skill.html", "login.html", "register.html", "admin_dashboard.html", "admin_skills.html", "admin_skill_detail.html", "admin_comments.html", "admin_users.html", "admin_tenants.html", "admin_tenant_detail.html", "upload.html"}

	for _, page := range pages {
		files := []string{basePath, filepath.Join(templateDir, page)}
		if strings.HasPrefix(page, "admin_") {
			files = []string{basePath, adminShellPath, filepath.Join(templateDir, page)}
		}
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
		pageData["IsSubAdmin"] = ctx.User != nil && ctx.User.IsSubAdmin
		pageData["IsAdmin"] = ctx.User != nil && (ctx.User.IsPlatformAdmin || ctx.User.IsSubAdmin)
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
	target["SortBy"] = data.SortBy
	target["StatusFilter"] = data.StatusFilter
	target["CurrentPage"] = data.CurrentPage
	target["Skill"] = data.Skill
	target["Comments"] = data.Comments
	target["UserRating"] = data.UserRating
	target["TotalSkills"] = data.TotalSkills
	target["CategoryCount"] = data.CategoryCount
	target["Error"] = data.Error
	target["Info"] = data.Info
	target["AdminSection"] = data.AdminSection
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
	if data.AdminStats != nil {
		target["AdminStats"] = data.AdminStats
		target["PendingReviewCount"] = data.AdminStats.PendingSkills
	}
	if data.AdminSkills != nil {
		target["AdminSkills"] = data.AdminSkills
	}
	if data.AdminUsers != nil {
		target["AdminUsers"] = data.AdminUsers
	}
	if data.AdminComments != nil {
		target["AdminComments"] = data.AdminComments
	}
	if data.AdminLogs != nil {
		target["AdminLogs"] = data.AdminLogs
	}
	if data.AdminSkill != nil {
		target["AdminSkill"] = data.AdminSkill
	}
}

type PageData struct {
	Title         string
	Skills        []models.Skill
	Categories    []string
	Query         string
	Category      string
	SortBy        string // 排序方式：score, rating, latest
	StatusFilter  string
	CurrentPage   string
	Skill         *models.Skill
	Comments      []models.SkillComment // 评论列表
	UserRating    int                   // 当前用户的评分
	TotalSkills   int
	CategoryCount int
	Error         string
	Info          string
	AdminSection  string
	Tenant        *models.Tenant
	Tenants       []models.Tenant
	Members       []models.TenantMember
	Invites       []models.TenantInvite
	AdminStats    *models.AdminDashboardStats
	AdminSkills   []models.AdminSkill
	AdminUsers    []models.AdminUser
	AdminComments []models.AdminComment
	AdminLogs     []models.AdminActionLog
	AdminSkill    *models.AdminSkill
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

// 邮箱脱敏：user@example.com -> u***@example.com
func maskEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return "***"
	}
	name := parts[0]
	if len(name) <= 1 {
		return name + "***@" + parts[1]
	}
	return string(name[0]) + "***@" + parts[1]
}

// 格式化时间带时分
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04")
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return formatTime(*t)
}

// 生成 1~n 的序列，模板里用来渲染星星
func seq(n int) []int {
	s := make([]int, n)
	for i := range s {
		s[i] = i + 1
	}
	return s
}

// 字符串转 int，模板里用
func parseIntFromStr(s string) int {
	var n int
	fmt.Sscan(s, &n)
	return n
}

// 四舍五入评分，返回 int
func roundRating(f float64) int {
	return int(f + 0.5)
}

func firstChar(value string) string {
	for _, r := range strings.TrimSpace(value) {
		return string(r)
	}
	return "?"
}

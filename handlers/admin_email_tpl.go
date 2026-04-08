package handlers

import (
	"log/slog"
	"net/http"
	"strings"

	"skills-hub/db"
)

// GET /admin/email-templates — 邮件模板管理
func AdminEmailTemplatesHandler(w http.ResponseWriter, r *http.Request) {
	templates, err := db.ListEmailTemplates()
	if err != nil {
		slog.Error("list email templates failed", "error", err)
	}

	RenderTemplate(w, r, "admin_email_templates.html", map[string]interface{}{
		"Title":          "邮件模板管理 - Skills Hub",
		"AdminSection":   "email_templates",
		"CurrentPage":    "admin",
		"EmailTemplates": templates,
		"Info":           r.URL.Query().Get("info"),
		"Error":          r.URL.Query().Get("error"),
	})
}

// POST /admin/email-templates/update — 更新模板
func AdminEmailTemplateUpdateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}

	id := strings.TrimSpace(r.FormValue("template_id"))
	subject := strings.TrimSpace(r.FormValue("subject"))
	bodyHTML := r.FormValue("body_html")

	if id == "" || subject == "" || bodyHTML == "" {
		http.Redirect(w, r, "/admin/email-templates?error=所有字段都不能为空", http.StatusSeeOther)
		return
	}

	if err := db.UpdateEmailTemplate(id, subject, bodyHTML); err != nil {
		slog.Error("update email template failed", "error", err)
		http.Redirect(w, r, "/admin/email-templates?error=保存失败", http.StatusSeeOther)
		return
	}

	recordAdminAction(r, "email_template.update", "email_template", 0, id)
	http.Redirect(w, r, "/admin/email-templates?info=模板已更新", http.StatusSeeOther)
}

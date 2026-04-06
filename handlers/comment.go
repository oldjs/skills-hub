package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"skills-hub/db"
	"skills-hub/security"
)

// POST 提交评论
func CommentSkillHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sess := GetCurrentSession(r)
	if sess == nil || sess.CurrentTenantID == 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}

	// 校验图形验证码
	captchaInput := strings.TrimSpace(r.FormValue("captcha"))
	slug := r.FormValue("slug")
	if !validateCaptcha(r, captchaInput) {
		// 验证码错误，带错误信息跳回详情页
		http.Redirect(w, r, "/skill?slug="+url.QueryEscape(slug)+"&error=captcha", http.StatusSeeOther)
		return
	}

	skillID, err := strconv.ParseInt(r.FormValue("skill_id"), 10, 64)
	if err != nil {
		http.Error(w, "无效的 skill ID", http.StatusBadRequest)
		return
	}

	content := strings.TrimSpace(r.FormValue("content"))
	if content == "" {
		http.Error(w, "评论内容不能为空", http.StatusBadRequest)
		return
	}
	// 限制评论长度
	if len([]rune(content)) > 500 {
		http.Error(w, "评论内容不能超过 500 个字符", http.StatusBadRequest)
		return
	}

	if err := db.AddComment(sess.CurrentTenantID, skillID, sess.UserID, content); err != nil {
		http.Error(w, "评论失败", http.StatusInternalServerError)
		return
	}

	// 提交后跳回详情页
	http.Redirect(w, r, "/skill?slug="+url.QueryEscape(slug), http.StatusSeeOther)
}

// 评论编辑器的预览直接走后端渲染，前端和最终展示不会跑偏。
func MarkdownPreviewHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}

	content := strings.TrimSpace(r.FormValue("content"))
	if len([]rune(content)) > 500 {
		http.Error(w, "评论内容不能超过 500 个字符", http.StatusBadRequest)
		return
	}

	rendered, err := security.RenderCommentMarkdown(content)
	if err != nil {
		http.Error(w, "预览生成失败", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"html": string(rendered),
	})
}

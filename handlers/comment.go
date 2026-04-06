package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"skills-hub/db"
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

	slug := r.FormValue("slug")

	if err := db.AddComment(sess.CurrentTenantID, skillID, sess.UserID, content); err != nil {
		http.Error(w, "评论失败", http.StatusInternalServerError)
		return
	}

	// 提交后跳回详情页
	http.Redirect(w, r, "/skill?slug="+slug, http.StatusSeeOther)
}

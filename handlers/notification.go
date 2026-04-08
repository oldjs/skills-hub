package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"skills-hub/db"
)

// GET /api/notifications — 拉通知列表 + 未读数
func NotificationsAPIHandler(w http.ResponseWriter, r *http.Request) {
	sess := GetCurrentSession(r)
	if sess == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	notifications, _ := db.GetUserNotifications(sess.UserID, 20)
	unread := db.CountUnreadNotifications(sess.UserID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"notifications": notifications,
		"unread":        unread,
	})
}

// POST /api/notifications/read — 标记单条已读
func NotificationReadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sess := GetCurrentSession(r)
	if sess == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}

	nID, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "无效的 ID", http.StatusBadRequest)
		return
	}
	_ = db.MarkNotificationRead(nID, sess.UserID)

	// 表单提交跳回 account 页，AJAX 返回 JSON
	if !wantsJSON(r) {
		http.Redirect(w, r, "/account#notifications", http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// POST /api/notifications/read-all — 全部标记已读
func NotificationReadAllHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sess := GetCurrentSession(r)
	if sess == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}
	_ = db.MarkAllNotificationsRead(sess.UserID)

	if !wantsJSON(r) {
		http.Redirect(w, r, "/account#notifications", http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// 判断请求方是想要 JSON 还是 HTML
func wantsJSON(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "application/json")
}

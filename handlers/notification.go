package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

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
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

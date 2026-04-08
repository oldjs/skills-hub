package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"skills-hub/db"
)

// POST /api/bookmark — 切换收藏状态
func BookmarkToggleHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sess := GetCurrentSession(r)
	if sess == nil || sess.CurrentTenantID == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}
	if !enforceRateLimit(w, fmt.Sprintf("user-write:%d", sess.UserID), 10, sess.IsPlatformAdmin || sess.IsSubAdmin) {
		return
	}

	skillID, err := strconv.ParseInt(r.FormValue("skill_id"), 10, 64)
	if err != nil || skillID <= 0 {
		http.Error(w, "无效的 skill ID", http.StatusBadRequest)
		return
	}

	bookmarked, err := db.ToggleBookmark(sess.UserID, skillID, sess.CurrentTenantID)
	if err != nil {
		log.Printf("bookmark toggle failed: %v", err)
		http.Error(w, "操作失败", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"bookmarked": bookmarked,
	})
}

// GET /account/bookmarks — 收藏列表页
func AccountBookmarksHandler(w http.ResponseWriter, r *http.Request) {
	sess := GetCurrentSession(r)
	if sess == nil || sess.CurrentTenantID == 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	bookmarks, err := db.GetUserBookmarks(sess.UserID, sess.CurrentTenantID, 50)
	if err != nil {
		log.Printf("bookmarks load failed: %v", err)
		bookmarks = nil
	}

	data := PageData{
		Title:       "我的收藏 - Skills Hub",
		CurrentPage: "bookmarks",
		Skills:      bookmarks,
	}
	RenderTemplate(w, r, "bookmarks.html", data)
}

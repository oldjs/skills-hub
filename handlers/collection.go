package handlers

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"skills-hub/db"
)

// GET /collections — 我的合集列表
func CollectionListHandler(w http.ResponseWriter, r *http.Request) {
	sess := GetCurrentSession(r)
	if sess == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	collections, err := db.ListUserCollections(sess.UserID)
	if err != nil {
		slog.Error("list collections failed", "error", err)
	}

	RenderTemplate(w, r, "collections.html", map[string]interface{}{
		"Title":       "我的合集 - Skills Hub",
		"CurrentPage": "collections",
		"Collections": collections,
	})
}

// POST /collections/create — 创建合集
func CollectionCreateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}
	sess := GetCurrentSession(r)
	if sess == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	desc := strings.TrimSpace(r.FormValue("description"))
	if name == "" {
		http.Redirect(w, r, "/collections?error=名称不能为空", http.StatusSeeOther)
		return
	}

	_, err := db.CreateCollection(sess.UserID, name, desc, true)
	if err != nil {
		slog.Error("create collection failed", "error", err)
		http.Redirect(w, r, "/collections?error=创建失败", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/collections?info=合集已创建", http.StatusSeeOther)
}

// POST /collections/add-skill — 向合集添加技能
func CollectionAddSkillHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}
	sess := GetCurrentSession(r)
	if sess == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	collectionID, _ := strconv.ParseInt(r.FormValue("collection_id"), 10, 64)
	skillID, _ := strconv.ParseInt(r.FormValue("skill_id"), 10, 64)
	if collectionID == 0 || skillID == 0 {
		http.Error(w, "参数错误", http.StatusBadRequest)
		return
	}

	// 验证合集归属
	c, err := db.GetCollection(collectionID)
	if err != nil || c == nil || c.UserID != sess.UserID {
		http.Error(w, "合集不存在", http.StatusNotFound)
		return
	}

	_ = db.AddToCollection(collectionID, skillID)
	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

// POST /collections/delete — 删除合集
func CollectionDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}
	sess := GetCurrentSession(r)
	if sess == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	collectionID, _ := strconv.ParseInt(r.FormValue("collection_id"), 10, 64)
	_ = db.DeleteCollection(collectionID, sess.UserID)
	http.Redirect(w, r, "/collections?info=合集已删除", http.StatusSeeOther)
}

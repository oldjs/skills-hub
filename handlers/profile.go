package handlers

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"skills-hub/db"
	"skills-hub/models"
)

// GET /user?id=123 公开 Profile 页
func ProfileHandler(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.URL.Query().Get("id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil || userID <= 0 {
		RenderNotFound(w, r)
		return
	}

	profile, err := db.GetUserPublicProfile(userID)
	if err != nil || profile == nil {
		RenderNotFound(w, r)
		return
	}

	// 根据 tab 参数加载对应数据
	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "ratings"
	}

	var ratings []models.UserRatingItem
	var comments []models.UserCommentItem

	switch tab {
	case "comments":
		comments, err = db.GetUserComments(userID, 20)
		if err != nil {
			slog.Error("profile comments load failed", "error", err)
		}
	default:
		tab = "ratings"
		ratings, err = db.GetUserRatings(userID, 20)
		if err != nil {
			slog.Error("profile ratings load failed", "error", err)
		}
	}

	profileDesc := profile.DisplayName + " 在 Skills Hub 的个人主页"
	if profile.Bio != "" {
		profileDesc = truncateText(profile.Bio, 160)
	}

	data := PageData{
		Title:           profile.DisplayName + " - Skills Hub",
		MetaDescription: profileDesc,
		CanonicalURL:    canonicalURL("/user?id=" + userIDStr),
		CurrentPage:     "profile",
		ProfileUser:    profile,
		ProfileTab:     tab,
		ProfileRatings: ratings,
		ProfileComments: comments,
	}

	RenderTemplate(w, r, "profile.html", data)
}

// POST /account/profile 编辑个人资料
func AccountProfileUpdateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sess := GetCurrentSession(r)
	if sess == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}

	displayName := strings.TrimSpace(r.FormValue("display_name"))
	bio := strings.TrimSpace(r.FormValue("bio"))

	// 基本校验
	if displayName == "" {
		http.Redirect(w, r, "/account?error=显示名称不能为空", http.StatusSeeOther)
		return
	}
	if len([]rune(displayName)) > 50 {
		displayName = string([]rune(displayName)[:50])
	}
	if len([]rune(bio)) > 200 {
		bio = string([]rune(bio)[:200])
	}

	if err := db.UpdateUserProfile(sess.UserID, displayName, bio); err != nil {
		slog.Error("update profile failed", "error", err)
		http.Redirect(w, r, "/account?error=保存失败", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/account?info=个人资料已更新", http.StatusSeeOther)
}

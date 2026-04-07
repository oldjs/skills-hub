package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"skills-hub/db"
)

func AdminUsersHandler(w http.ResponseWriter, r *http.Request) {
	users, err := db.ListAdminUsers()
	if err != nil {
		http.Error(w, "用户列表加载失败", http.StatusInternalServerError)
		return
	}

	renderAdminPage(w, r, "admin_users.html", PageData{
		Title:        "用户管理 - Skills Hub",
		AdminSection: "users",
		AdminUsers:   users,
		Info:         r.URL.Query().Get("info"),
		Error:        r.URL.Query().Get("error"),
	})
}

func AdminUserUpdateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}

	userID, err := parseInt64(r.FormValue("user_id"))
	if err != nil {
		http.Redirect(w, r, "/admin/users?error=参数错误", http.StatusSeeOther)
		return
	}

	status := strings.TrimSpace(r.FormValue("status"))
	if status != "active" && status != "disabled" {
		http.Redirect(w, r, "/admin/users?error=状态不正确", http.StatusSeeOther)
		return
	}

	user, err := db.GetUserByID(userID)
	if err != nil || user == nil {
		http.Redirect(w, r, "/admin/users?error=用户不存在", http.StatusSeeOther)
		return
	}

	if status == "disabled" && user.IsPlatformAdmin {
		count, err := db.CountActivePlatformAdmins()
		if err != nil {
			http.Redirect(w, r, "/admin/users?error=管理员数量校验失败", http.StatusSeeOther)
			return
		}
		if count <= 1 {
			http.Redirect(w, r, "/admin/users?error=至少保留一个可用的平台管理员", http.StatusSeeOther)
			return
		}
	}

	if err := db.UpdateUserStatusByAdmin(userID, status); err != nil {
		http.Redirect(w, r, "/admin/users?error=用户状态更新失败", http.StatusSeeOther)
		return
	}

	recordAdminAction(r, "user.status", "user", userID, fmt.Sprintf("将用户 %s 设为 %s", user.Email, status))
	http.Redirect(w, r, "/admin/users?info=用户状态已更新", http.StatusSeeOther)
}

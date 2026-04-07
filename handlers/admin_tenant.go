package handlers

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"skills-hub/db"
)

func AdminTenantsHandler(w http.ResponseWriter, r *http.Request) {
	tenants, err := db.ListTenants()
	if err != nil {
		http.Error(w, "租户加载失败", http.StatusInternalServerError)
		return
	}

	renderAdminPage(w, r, "admin_tenants.html", PageData{
		Title:        "租户管理 - Skills Hub",
		AdminSection: "tenants",
		Tenants:      tenants,
		Info:         r.URL.Query().Get("info"),
		Error:        r.URL.Query().Get("error"),
	})
}

func AdminTenantDetailHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, err := parseInt64(r.URL.Query().Get("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	tenant, err := db.GetTenantByID(tenantID)
	if err != nil || tenant == nil {
		http.NotFound(w, r)
		return
	}
	members, err := db.ListTenantMembers(tenantID)
	if err != nil {
		http.Error(w, "成员加载失败", http.StatusInternalServerError)
		return
	}
	invites, err := db.ListTenantInvites(tenantID)
	if err != nil {
		http.Error(w, "邀请加载失败", http.StatusInternalServerError)
		return
	}

	renderAdminPage(w, r, "admin_tenant_detail.html", PageData{
		Title:        tenant.Name + " - 租户管理",
		AdminSection: "tenants",
		Tenant:       tenant,
		Members:      members,
		Invites:      invites,
		Info:         r.URL.Query().Get("info"),
		Error:        r.URL.Query().Get("error"),
	})
}

func AdminTenantCreateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	slug := strings.TrimSpace(r.FormValue("slug"))
	description := strings.TrimSpace(r.FormValue("description"))
	ownerEmail := normalizeEmail(r.FormValue("owner_email"))
	if name == "" || slug == "" {
		http.Redirect(w, r, "/admin/tenants?error=请填写租户名称和标识", http.StatusSeeOther)
		return
	}

	tenant, err := db.CreateTenant(slug, name, description, true)
	if err != nil {
		http.Redirect(w, r, "/admin/tenants?error=创建租户失败", http.StatusSeeOther)
		return
	}

	if ownerEmail != "" {
		user, err := db.GetUserByEmail(ownerEmail)
		if err == nil && user != nil {
			if _, err := db.AddTenantMember(tenant.ID, user.ID, "owner"); err != nil {
				http.Redirect(w, r, "/admin/tenants?error=租户已创建，但负责人添加失败", http.StatusSeeOther)
				return
			}
		} else {
			if err := db.CreateTenantInvite(tenant.ID, ownerEmail, "owner", time.Now().Add(7*24*time.Hour)); err != nil {
				http.Redirect(w, r, "/admin/tenants?error=租户已创建，但负责人邀请失败", http.StatusSeeOther)
				return
			}
		}
	}

	recordAdminAction(r, "tenant.create", "tenant", tenant.ID, "创建了新租户")
	http.Redirect(w, r, "/admin/tenants?info=租户已创建", http.StatusSeeOther)
}

func AdminTenantUpdateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}
	tenantID, err := parseInt64(r.FormValue("tenant_id"))
	if err != nil {
		http.Redirect(w, r, "/admin/tenants?error=参数错误", http.StatusSeeOther)
		return
	}
	autoSync := r.FormValue("auto_sync_enabled") == "1"
	if err := db.UpdateTenant(tenantID, strings.TrimSpace(r.FormValue("name")), strings.TrimSpace(r.FormValue("slug")), strings.TrimSpace(r.FormValue("description")), strings.TrimSpace(r.FormValue("status")), autoSync); err != nil {
		http.Redirect(w, r, "/admin/tenant?id="+r.FormValue("tenant_id")+"&error=保存失败", http.StatusSeeOther)
		return
	}
	recordAdminAction(r, "tenant.update", "tenant", tenantID, "更新了租户设置")
	http.Redirect(w, r, "/admin/tenant?id="+r.FormValue("tenant_id")+"&info=租户信息已更新", http.StatusSeeOther)
}

func AdminTenantInviteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}
	tenantID, err := parseInt64(r.FormValue("tenant_id"))
	if err != nil {
		http.Redirect(w, r, "/admin/tenants?error=参数错误", http.StatusSeeOther)
		return
	}
	email := normalizeEmail(r.FormValue("email"))
	role := strings.TrimSpace(r.FormValue("role"))
	if !validateEmail(email) {
		http.Redirect(w, r, "/admin/tenant?id="+r.FormValue("tenant_id")+"&error=邮箱格式不正确", http.StatusSeeOther)
		return
	}
	if role == "" {
		role = "member"
	}

	user, err := db.GetUserByEmail(email)
	if err == nil && user != nil {
		if _, err := db.AddTenantMember(tenantID, user.ID, role); err == nil {
			recordAdminAction(r, "tenant.member.add", "tenant", tenantID, "添加了租户成员 "+email)
			http.Redirect(w, r, "/admin/tenant?id="+r.FormValue("tenant_id")+"&info=成员已添加", http.StatusSeeOther)
			return
		}
	}
	if err := db.CreateTenantInvite(tenantID, email, role, time.Now().Add(7*24*time.Hour)); err != nil {
		http.Redirect(w, r, "/admin/tenant?id="+r.FormValue("tenant_id")+"&error=邀请创建失败", http.StatusSeeOther)
		return
	}
	recordAdminAction(r, "tenant.invite.create", "tenant", tenantID, "创建了邀请 "+email)
	http.Redirect(w, r, "/admin/tenant?id="+r.FormValue("tenant_id")+"&info=邀请已发送", http.StatusSeeOther)
}

func AdminTenantInviteRevokeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}
	inviteID, err := parseInt64(r.FormValue("invite_id"))
	if err == nil {
		_ = db.RevokeTenantInvite(inviteID)
		recordAdminAction(r, "tenant.invite.revoke", "invite", inviteID, "撤销了一条租户邀请")
	}
	http.Redirect(w, r, "/admin/tenant?id="+r.FormValue("tenant_id")+"&info=邀请已撤销", http.StatusSeeOther)
}

func AdminTenantMemberUpdateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}
	tenantID, err1 := parseInt64(r.FormValue("tenant_id"))
	userID, err2 := parseInt64(r.FormValue("user_id"))
	if err1 != nil || err2 != nil {
		http.Redirect(w, r, "/admin/tenants?error=参数错误", http.StatusSeeOther)
		return
	}
	if err := db.UpdateTenantMember(tenantID, userID, strings.TrimSpace(r.FormValue("role")), strings.TrimSpace(r.FormValue("status"))); err != nil {
		http.Redirect(w, r, "/admin/tenant?id="+r.FormValue("tenant_id")+"&error=成员更新失败", http.StatusSeeOther)
		return
	}
	recordAdminAction(r, "tenant.member.update", "tenant", tenantID, "更新了租户成员角色或状态")
	http.Redirect(w, r, "/admin/tenant?id="+r.FormValue("tenant_id")+"&info=成员已更新", http.StatusSeeOther)
}

func AdminTenantMemberRemoveHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}
	tenantID, err1 := parseInt64(r.FormValue("tenant_id"))
	userID, err2 := parseInt64(r.FormValue("user_id"))
	if err1 != nil || err2 != nil {
		http.Redirect(w, r, "/admin/tenants?error=参数错误", http.StatusSeeOther)
		return
	}
	if err := db.RemoveTenantMember(tenantID, userID); err != nil {
		http.Redirect(w, r, "/admin/tenant?id="+r.FormValue("tenant_id")+"&error="+urlQueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	recordAdminAction(r, "tenant.member.remove", "tenant", tenantID, "移除了一个租户成员")
	http.Redirect(w, r, "/admin/tenant?id="+r.FormValue("tenant_id")+"&info=成员已移除", http.StatusSeeOther)
}

func AdminTenantSyncHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}
	tenantID, err := parseInt64(r.FormValue("tenant_id"))
	if err != nil {
		http.Redirect(w, r, "/admin/tenants?error=参数错误", http.StatusSeeOther)
		return
	}
	if !db.StartTenantSync(tenantID) {
		http.Redirect(w, r, "/admin/tenant?id="+r.FormValue("tenant_id")+"&error=同步已经在进行中", http.StatusSeeOther)
		return
	}
	go func() {
		defer db.FinishTenantSync(tenantID)
		_ = db.SyncFromClawHub(tenantID)
	}()
	recordAdminAction(r, "tenant.sync.start", "tenant", tenantID, "手动触发了租户同步")
	http.Redirect(w, r, "/admin/tenant?id="+r.FormValue("tenant_id")+"&info=已开始后台同步", http.StatusSeeOther)
}

func urlQueryEscape(value string) string {
	return url.QueryEscape(value)
}

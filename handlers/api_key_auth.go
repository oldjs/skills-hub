package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"skills-hub/db"
	"skills-hub/models"
)

type apiAuthContext struct {
	User   *models.User
	APIKey *models.APIKey
}

func requireAPIKeyAuth(w http.ResponseWriter, r *http.Request) (*apiAuthContext, bool) {
	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
		writeAPIUnauthorized(w)
		return nil, false
	}
	keyValue := strings.TrimSpace(authorization[len("Bearer "):])
	if keyValue == "" {
		writeAPIUnauthorized(w)
		return nil, false
	}

	user, apiKey, err := db.FindUserByAPIKey(keyValue)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "API Key 校验失败")
		return nil, false
	}
	if user == nil || apiKey == nil || user.Status != "active" {
		writeAPIUnauthorized(w)
		return nil, false
	}
	if err := db.TouchAPIKeyUsage(apiKey.ID); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "API Key 状态更新失败")
		return nil, false
	}

	isAdmin := user.IsPlatformAdmin || user.IsSubAdmin
	if !enforceRateLimit(w, fmt.Sprintf("api-key:%d", apiKey.ID), 60, isAdmin) {
		return nil, false
	}
	if !enforceRateLimit(w, fmt.Sprintf("api-ip:%s", GetClientIP(r)), 300, isAdmin) {
		return nil, false
	}

	return &apiAuthContext{User: user, APIKey: apiKey}, true
}

func resolveAPITenantScopeForUser(user *models.User, r *http.Request) (*int64, error) {
	tenantID, err := parseTenantIDQuery(r)
	if err != nil {
		return nil, fmt.Errorf("tenant_id 参数不正确")
	}
	if tenantID == nil || *tenantID == 0 {
		return nil, nil
	}
	tenant, err := db.GetUserTenant(user.ID, *tenantID)
	if err != nil {
		return nil, fmt.Errorf("租户校验失败")
	}
	if tenant == nil || tenant.TenantStatus != "active" || tenant.MembershipStatus != "active" {
		return nil, fmt.Errorf("无权访问该租户数据")
	}
	return tenantID, nil
}

func resolveRequiredAPITenantForUser(user *models.User, r *http.Request) (*models.UserTenant, error) {
	tenantID, err := parseTenantIDQuery(r)
	if err != nil {
		return nil, fmt.Errorf("tenant_id 参数不正确")
	}
	if tenantID != nil && *tenantID > 0 {
		tenant, err := db.GetUserTenant(user.ID, *tenantID)
		if err != nil {
			return nil, fmt.Errorf("租户校验失败")
		}
		if tenant == nil || tenant.TenantStatus != "active" || tenant.MembershipStatus != "active" {
			return nil, fmt.Errorf("无权访问该租户数据")
		}
		return tenant, nil
	}
	tenant, err := db.PickActiveTenant(user.ID, user.LastTenantID)
	if err != nil {
		return nil, err
	}
	return tenant, nil
}

func writeAPIUnauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="skills-hub"`)
	writeAPIError(w, http.StatusUnauthorized, "需要有效的 API Key")
}

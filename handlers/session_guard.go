package handlers

import (
	"log"
	"net/http"

	"skills-hub/db"
	"skills-hub/models"
)

func sessionTokenFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}

	cookie, err := r.Cookie("session")
	if err != nil {
		return ""
	}
	return cookie.Value
}

func destroySession(w http.ResponseWriter, r *http.Request) {
	if token := sessionTokenFromRequest(r); token != "" {
		deleteSession(token)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "csrf_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   isCookieSecure(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	clearSessionCookie(w)
}

func persistSessionSnapshot(w http.ResponseWriter, r *http.Request, sess sessionData) error {
	token := sessionTokenFromRequest(r)
	if token == "" {
		return http.ErrNoCookie
	}

	if err := setSession(token, sess); err != nil {
		return err
	}
	setSessionCookie(w, token)
	return nil
}

func loadActiveSession(w http.ResponseWriter, r *http.Request) (*sessionData, error) {
	sess := getSession(r)
	if sess == nil {
		return nil, nil
	}

	user, err := db.GetUserByID(sess.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil || user.Status != "active" {
		// 用户被禁用或删掉后，旧 session 直接作废。
		destroySession(w, r)
		return nil, nil
	}

	var (
		tenant *models.UserTenant
	)
	if sess.CurrentTenantID > 0 {
		tenant, err = db.GetUserTenant(user.ID, sess.CurrentTenantID)
		if err != nil {
			return nil, err
		}
	}
	if tenant == nil || tenant.TenantStatus != "active" || tenant.MembershipStatus != "active" {
		tenant, err = ensureUserTenant(user)
		if err != nil {
			return nil, err
		}
	}
	if tenant == nil || tenant.TenantStatus != "active" || tenant.MembershipStatus != "active" {
		// 当前账号已经没有可用租户了，保留 session 只会继续出错。
		destroySession(w, r)
		return nil, nil
	}

	if user.LastTenantID == nil || *user.LastTenantID != tenant.TenantID {
		if err := db.UpdateUserLastTenant(user.ID, tenant.TenantID); err != nil {
			return nil, err
		}
	}

	refreshed := buildSession(user, tenant)
	if sessionChanged(sess, &refreshed) {
		if err := persistSessionSnapshot(w, r, refreshed); err != nil {
			return nil, err
		}
	}

	return &refreshed, nil
}

func requireTenantMembership(w http.ResponseWriter, r *http.Request, tenantID int64) (*sessionData, bool, error) {
	sess, err := loadActiveSession(w, r)
	if err != nil || sess == nil {
		return nil, false, err
	}

	tenant, err := db.GetUserTenant(sess.UserID, tenantID)
	if err != nil {
		return nil, false, err
	}
	if tenant == nil || tenant.TenantStatus != "active" || tenant.MembershipStatus != "active" {
		return sess, false, nil
	}

	return sess, true, nil
}

func sessionChanged(left, right *sessionData) bool {
	if left == nil || right == nil {
		return left != right
	}

	return left.UserID != right.UserID ||
		left.Email != right.Email ||
		left.DisplayName != right.DisplayName ||
		left.IsPlatformAdmin != right.IsPlatformAdmin ||
		left.IsSubAdmin != right.IsSubAdmin ||
		left.CurrentTenantID != right.CurrentTenantID ||
		left.TenantName != right.TenantName ||
		left.TenantSlug != right.TenantSlug ||
		left.TenantRole != right.TenantRole
}

func logSessionRefreshError(err error) {
	if err != nil {
		log.Printf("session refresh failed: %v", err)
	}
}

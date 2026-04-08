package handlers

import (
	"skills-hub/db"
)

// 未登录用户使用的默认租户 ID（取第一个 active 租户）
// 登录用户直接用 session 里的 CurrentTenantID
func resolveViewTenantID(sess *sessionData) int64 {
	if sess != nil && sess.CurrentTenantID > 0 {
		return sess.CurrentTenantID
	}
	return db.GetDefaultTenantID()
}

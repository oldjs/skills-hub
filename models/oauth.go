package models

import "time"

// 第三方 OAuth 登录关联记录
type OAuthConnection struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"userId"`
	Provider       string    `json:"provider"`       // github / google
	ProviderUserID string    `json:"providerUserId"` // 第三方平台的用户 ID
	Email          string    `json:"email"`
	DisplayName    string    `json:"displayName"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// OIDC Provider 的客户端应用
type OAuthClient struct {
	ID           string    `json:"id"`       // client_id
	Name         string    `json:"name"`
	RedirectURIs []string  `json:"redirectUris"`
	CreatedAt    time.Time `json:"createdAt"`
}

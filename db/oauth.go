package db

import (
	"database/sql"
	"time"

	"skills-hub/models"
)

// 按 provider + provider_user_id 找关联
func GetOAuthConnection(provider, providerUserID string) (*models.OAuthConnection, error) {
	row := GetDB().QueryRow(`
		SELECT id, user_id, provider, provider_user_id, email, display_name, created_at, updated_at
		FROM oauth_connections
		WHERE provider = ? AND provider_user_id = ?
	`, provider, providerUserID)

	var c models.OAuthConnection
	err := row.Scan(&c.ID, &c.UserID, &c.Provider, &c.ProviderUserID, &c.Email, &c.DisplayName, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// 创建 OAuth 关联（把第三方账号绑定到本地 user_id）
func CreateOAuthConnection(userID int64, provider, providerUserID, email, displayName, accessToken, refreshToken string) error {
	now := time.Now()
	_, err := GetDB().Exec(`
		INSERT INTO oauth_connections (user_id, provider, provider_user_id, email, display_name, access_token, refresh_token, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(provider, provider_user_id) DO UPDATE SET
			user_id = excluded.user_id,
			email = excluded.email,
			display_name = excluded.display_name,
			access_token = excluded.access_token,
			refresh_token = excluded.refresh_token,
			updated_at = excluded.updated_at
	`, userID, provider, providerUserID, email, displayName, accessToken, refreshToken, now, now)
	return err
}

// 拉某用户的所有 OAuth 关联
func GetUserOAuthConnections(userID int64) ([]models.OAuthConnection, error) {
	rows, err := GetDB().Query(`
		SELECT id, user_id, provider, provider_user_id, email, display_name, created_at, updated_at
		FROM oauth_connections WHERE user_id = ?
		ORDER BY created_at ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conns []models.OAuthConnection
	for rows.Next() {
		var c models.OAuthConnection
		if err := rows.Scan(&c.ID, &c.UserID, &c.Provider, &c.ProviderUserID, &c.Email, &c.DisplayName, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		conns = append(conns, c)
	}
	return conns, rows.Err()
}

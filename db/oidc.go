package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"

	"skills-hub/models"
)

// 按 client_id 查 OIDC 客户端
func GetOAuthClient(clientID string) (*models.OAuthClient, error) {
	row := GetDB().QueryRow(`
		SELECT id, name, redirect_uris, created_at
		FROM oauth_clients WHERE id = ?
	`, clientID)

	var c models.OAuthClient
	var redirectURIsJSON string
	err := row.Scan(&c.ID, &c.Name, &redirectURIsJSON, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(redirectURIsJSON), &c.RedirectURIs)
	return &c, nil
}

// 验证 client_secret
func VerifyOAuthClientSecret(clientID, secret string) (*models.OAuthClient, error) {
	client, err := GetOAuthClient(clientID)
	if err != nil || client == nil {
		return nil, err
	}

	hash := sha256.Sum256([]byte(secret))
	expectedHash := hex.EncodeToString(hash[:])

	var storedHash string
	err = GetDB().QueryRow(`SELECT secret_hash FROM oauth_clients WHERE id = ?`, clientID).Scan(&storedHash)
	if err != nil {
		return nil, err
	}
	if storedHash != expectedHash {
		return nil, nil
	}

	return client, nil
}

// 创建 OIDC 客户端，返回明文 secret（只展示一次）
func CreateOAuthClient(id, name, secret string, redirectURIs []string) error {
	hash := sha256.Sum256([]byte(secret))
	hashHex := hex.EncodeToString(hash[:])

	urisJSON, _ := json.Marshal(redirectURIs)
	_, err := GetDB().Exec(`
		INSERT INTO oauth_clients (id, secret_hash, name, redirect_uris)
		VALUES (?, ?, ?, ?)
	`, id, hashHex, name, string(urisJSON))
	return err
}

// 删除 OIDC 客户端
func DeleteOAuthClient(clientID string) error {
	_, err := GetDB().Exec(`DELETE FROM oauth_clients WHERE id = ?`, clientID)
	return err
}

// 列出所有 OIDC 客户端
func ListOAuthClients() ([]models.OAuthClient, error) {
	rows, err := GetDB().Query(`SELECT id, name, redirect_uris, created_at FROM oauth_clients ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clients []models.OAuthClient
	for rows.Next() {
		var c models.OAuthClient
		var urisJSON string
		if err := rows.Scan(&c.ID, &c.Name, &urisJSON, &c.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(urisJSON), &c.RedirectURIs)
		clients = append(clients, c)
	}
	return clients, rows.Err()
}

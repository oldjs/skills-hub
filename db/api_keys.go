package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"time"

	"skills-hub/models"
	"skills-hub/security"
)

func HashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func CreateAPIKey(userID int64, keyHash, keyPrefix, name string) (*models.APIKey, error) {
	keyHash = security.EscapePlainText(keyHash)
	keyPrefix = security.EscapePlainText(keyPrefix)
	name = security.EscapePlainText(name)

	result, err := GetDB().Exec(`
		INSERT INTO api_keys (user_id, key_hash, key_prefix, name, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, userID, keyHash, keyPrefix, name, time.Now())
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return GetAPIKeyByID(id)
}

func ListUserAPIKeys(userID int64) ([]models.APIKey, error) {
	rows, err := GetDB().Query(`
		SELECT id, user_id, key_prefix, name, created_at, last_used_at, revoked_at
		FROM api_keys
		WHERE user_id = ?
		ORDER BY created_at DESC, id DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []models.APIKey
	for rows.Next() {
		item, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, item)
	}
	return keys, rows.Err()
}

func GetAPIKeyByID(id int64) (*models.APIKey, error) {
	row := GetDB().QueryRow(`
		SELECT id, user_id, key_prefix, name, created_at, last_used_at, revoked_at
		FROM api_keys WHERE id = ?
	`, id)
	item, err := scanAPIKey(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func FindUserByAPIKey(raw string) (*models.User, *models.APIKey, error) {
	hash := HashAPIKey(raw)
	row := GetDB().QueryRow(`
		SELECT k.id, k.user_id, k.key_prefix, k.name, k.created_at, k.last_used_at, k.revoked_at,
		       u.id, u.email, u.display_name, u.status, u.is_platform_admin, u.is_sub_admin,
		       u.last_tenant_id, u.last_login_at, u.created_at, u.updated_at
		FROM api_keys k
		JOIN users u ON u.id = k.user_id
		WHERE k.key_hash = ? AND k.revoked_at IS NULL
	`, hash)

	var (
		key              models.APIKey
		user             models.User
		keyLastUsedAt    sql.NullTime
		keyRevokedAt     sql.NullTime
		userLastTenantID sql.NullInt64
		userLastLoginAt  sql.NullTime
		isPlatformAdmin  int
		isSubAdmin       int
	)
	if err := row.Scan(&key.ID, &key.UserID, &key.KeyPrefix, &key.Name, &key.CreatedAt, &keyLastUsedAt, &keyRevokedAt, &user.ID, &user.Email, &user.DisplayName, &user.Status, &isPlatformAdmin, &isSubAdmin, &userLastTenantID, &userLastLoginAt, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	if keyLastUsedAt.Valid {
		key.LastUsedAt = &keyLastUsedAt.Time
	}
	if keyRevokedAt.Valid {
		key.RevokedAt = &keyRevokedAt.Time
	}
	key.KeyPrefix = security.DecodeStoredText(key.KeyPrefix)
	key.Name = security.DecodeStoredText(key.Name)
	user.IsPlatformAdmin = isPlatformAdmin == 1
	user.IsSubAdmin = isSubAdmin == 1
	if userLastTenantID.Valid {
		user.LastTenantID = &userLastTenantID.Int64
	}
	if userLastLoginAt.Valid {
		user.LastLoginAt = &userLastLoginAt.Time
	}
	decodeUserForDisplay(&user)
	return &user, &key, nil
}

func TouchAPIKeyUsage(id int64) error {
	_, err := GetDB().Exec(`UPDATE api_keys SET last_used_at = ? WHERE id = ?`, time.Now(), id)
	return err
}

func RevokeAPIKey(id, userID int64) error {
	_, err := GetDB().Exec(`UPDATE api_keys SET revoked_at = ? WHERE id = ? AND user_id = ? AND revoked_at IS NULL`, time.Now(), id, userID)
	return err
}

func scanAPIKey(scanner interface {
	Scan(dest ...interface{}) error
}) (models.APIKey, error) {
	var (
		item       models.APIKey
		lastUsedAt sql.NullTime
		revokedAt  sql.NullTime
	)
	if err := scanner.Scan(&item.ID, &item.UserID, &item.KeyPrefix, &item.Name, &item.CreatedAt, &lastUsedAt, &revokedAt); err != nil {
		return models.APIKey{}, err
	}
	item.KeyPrefix = security.DecodeStoredText(item.KeyPrefix)
	item.Name = security.DecodeStoredText(item.Name)
	if lastUsedAt.Valid {
		item.LastUsedAt = &lastUsedAt.Time
	}
	if revokedAt.Valid {
		item.RevokedAt = &revokedAt.Time
	}
	return item, nil
}

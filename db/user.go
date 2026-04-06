package db

import (
	"database/sql"
	"time"

	"skills-hub/models"
)

func GetUserByEmail(email string) (*models.User, error) {
	row := GetDB().QueryRow(`
		SELECT id, email, display_name, status, is_platform_admin, last_tenant_id, last_login_at, created_at, updated_at
		FROM users WHERE email = ?
	`, email)

	var user models.User
	var isPlatformAdmin int
	var lastTenantID sql.NullInt64
	var lastLoginAt sql.NullTime
	if err := row.Scan(&user.ID, &user.Email, &user.DisplayName, &user.Status, &isPlatformAdmin, &lastTenantID, &lastLoginAt, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	user.IsPlatformAdmin = isPlatformAdmin == 1
	if lastTenantID.Valid {
		user.LastTenantID = &lastTenantID.Int64
	}
	if lastLoginAt.Valid {
		user.LastLoginAt = &lastLoginAt.Time
	}

	return &user, nil
}

func CreateUser(email, displayName string, isPlatformAdmin bool) (*models.User, error) {
	result, err := GetDB().Exec(`
		INSERT INTO users (email, display_name, is_platform_admin)
		VALUES (?, ?, ?)
	`, email, displayName, boolToInt(isPlatformAdmin))
	if err != nil {
		return nil, err
	}

	userID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return GetUserByID(userID)
}

func GetUserByID(userID int64) (*models.User, error) {
	row := GetDB().QueryRow(`
		SELECT id, email, display_name, status, is_platform_admin, last_tenant_id, last_login_at, created_at, updated_at
		FROM users WHERE id = ?
	`, userID)

	var user models.User
	var isPlatformAdmin int
	var lastTenantID sql.NullInt64
	var lastLoginAt sql.NullTime
	if err := row.Scan(&user.ID, &user.Email, &user.DisplayName, &user.Status, &isPlatformAdmin, &lastTenantID, &lastLoginAt, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	user.IsPlatformAdmin = isPlatformAdmin == 1
	if lastTenantID.Valid {
		user.LastTenantID = &lastTenantID.Int64
	}
	if lastLoginAt.Valid {
		user.LastLoginAt = &lastLoginAt.Time
	}

	return &user, nil
}

func UpdateUserLogin(userID, tenantID int64) error {
	_, err := GetDB().Exec(`
		UPDATE users
		SET last_tenant_id = ?, last_login_at = ?, updated_at = ?
		WHERE id = ?
	`, tenantID, time.Now(), time.Now(), userID)
	return err
}

func UpdateUserLastTenant(userID, tenantID int64) error {
	_, err := GetDB().Exec(`
		UPDATE users
		SET last_tenant_id = ?, updated_at = ?
		WHERE id = ?
	`, tenantID, time.Now(), userID)
	return err
}

func SetUserPlatformAdmin(userID int64, isPlatformAdmin bool) error {
	_, err := GetDB().Exec(`
		UPDATE users
		SET is_platform_admin = ?, updated_at = ?
		WHERE id = ?
	`, boolToInt(isPlatformAdmin), time.Now(), userID)
	return err
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

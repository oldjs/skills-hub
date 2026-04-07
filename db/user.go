package db

import (
	"database/sql"
	"time"

	"skills-hub/models"
	"skills-hub/security"
)

func GetUserByEmail(email string) (*models.User, error) {
	email = security.EscapePlainText(email)

	row := GetDB().QueryRow(`
		SELECT id, email, display_name, status, is_platform_admin, is_sub_admin, last_tenant_id, last_login_at, created_at, updated_at
		FROM users WHERE email = ?
	`, email)

	var user models.User
	var isPlatformAdmin int
	var isSubAdmin int
	var lastTenantID sql.NullInt64
	var lastLoginAt sql.NullTime
	if err := row.Scan(&user.ID, &user.Email, &user.DisplayName, &user.Status, &isPlatformAdmin, &isSubAdmin, &lastTenantID, &lastLoginAt, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	user.IsPlatformAdmin = isPlatformAdmin == 1
	user.IsSubAdmin = isSubAdmin == 1
	if lastTenantID.Valid {
		user.LastTenantID = &lastTenantID.Int64
	}
	if lastLoginAt.Valid {
		user.LastLoginAt = &lastLoginAt.Time
	}
	decodeUserForDisplay(&user)

	return &user, nil
}

func CreateUser(email, displayName string, isPlatformAdmin bool) (*models.User, error) {
	// 注册信息后面会反复回显，这里先收干净。
	email = security.EscapePlainText(email)
	displayName = security.EscapePlainText(displayName)

	tx, err := GetDB().Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	count, err := CountUsersTx(tx)
	if err != nil {
		return nil, err
	}
	// 第一位注册用户直接给平台管理员，后面再叠加环境变量提权规则。
	shouldBeAdmin := isPlatformAdmin || count == 0

	result, err := tx.Exec(`
		INSERT INTO users (email, display_name, is_platform_admin, is_sub_admin)
		VALUES (?, ?, ?, 0)
	`, email, displayName, boolToInt(shouldBeAdmin))
	if err != nil {
		return nil, err
	}

	userID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return GetUserByID(userID)
}

func GetUserByID(userID int64) (*models.User, error) {
	row := GetDB().QueryRow(`
		SELECT id, email, display_name, status, is_platform_admin, is_sub_admin, last_tenant_id, last_login_at, created_at, updated_at
		FROM users WHERE id = ?
	`, userID)

	var user models.User
	var isPlatformAdmin int
	var isSubAdmin int
	var lastTenantID sql.NullInt64
	var lastLoginAt sql.NullTime
	if err := row.Scan(&user.ID, &user.Email, &user.DisplayName, &user.Status, &isPlatformAdmin, &isSubAdmin, &lastTenantID, &lastLoginAt, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	user.IsPlatformAdmin = isPlatformAdmin == 1
	user.IsSubAdmin = isSubAdmin == 1
	if lastTenantID.Valid {
		user.LastTenantID = &lastTenantID.Int64
	}
	if lastLoginAt.Valid {
		user.LastLoginAt = &lastLoginAt.Time
	}
	decodeUserForDisplay(&user)

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
		SET is_platform_admin = ?, is_sub_admin = CASE WHEN ? = 1 THEN 0 ELSE is_sub_admin END, updated_at = ?
		WHERE id = ?
	`, boolToInt(isPlatformAdmin), boolToInt(isPlatformAdmin), time.Now(), userID)
	return err
}

func SetUserSubAdmin(userID int64, isSubAdmin bool) error {
	_, err := GetDB().Exec(`
		UPDATE users
		SET is_sub_admin = CASE WHEN is_platform_admin = 1 THEN 0 ELSE ? END, updated_at = ?
		WHERE id = ?
	`, boolToInt(isSubAdmin), time.Now(), userID)
	return err
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

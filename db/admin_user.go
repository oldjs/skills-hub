package db

import (
	"database/sql"
	"time"

	"skills-hub/models"
	"skills-hub/security"
)

func ListAdminComments(limit int) ([]models.AdminComment, error) {
	if limit <= 0 {
		limit = 200
	}

	rows, err := GetDB().Query(`
		SELECT c.id, c.tenant_id, COALESCE(t.name, ''), c.skill_id, s.slug, s.display_name,
		       c.user_id, COALESCE(u.email, ''), COALESCE(u.display_name, ''), c.content, c.created_at
		FROM skill_comments c
		JOIN skills s ON s.id = c.skill_id
		LEFT JOIN tenants t ON t.id = c.tenant_id
		LEFT JOIN users u ON u.id = c.user_id
		ORDER BY c.created_at DESC, c.id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []models.AdminComment
	for rows.Next() {
		var item models.AdminComment
		if err := rows.Scan(&item.ID, &item.TenantID, &item.TenantName, &item.SkillID, &item.SkillSlug, &item.SkillName, &item.UserID, &item.UserEmail, &item.UserDisplayName, &item.Content, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.TenantName = security.DecodeStoredText(item.TenantName)
		item.SkillName = security.DecodeStoredText(item.SkillName)
		item.UserEmail = security.DecodeStoredText(item.UserEmail)
		item.UserDisplayName = security.DecodeStoredText(item.UserDisplayName)
		item.Content = security.DecodeStoredText(item.Content)
		comments = append(comments, item)
	}
	return comments, rows.Err()
}

func DeleteCommentByID(commentID int64) error {
	_, err := GetDB().Exec(`DELETE FROM skill_comments WHERE id = ?`, commentID)
	return err
}

func GetAdminCommentByID(commentID int64) (*models.AdminComment, error) {
	row := GetDB().QueryRow(`
		SELECT c.id, c.tenant_id, COALESCE(t.name, ''), c.skill_id, s.slug, s.display_name,
		       c.user_id, COALESCE(u.email, ''), COALESCE(u.display_name, ''), c.content, c.created_at
		FROM skill_comments c
		JOIN skills s ON s.id = c.skill_id
		LEFT JOIN tenants t ON t.id = c.tenant_id
		LEFT JOIN users u ON u.id = c.user_id
		WHERE c.id = ?
	`, commentID)

	var item models.AdminComment
	if err := row.Scan(&item.ID, &item.TenantID, &item.TenantName, &item.SkillID, &item.SkillSlug, &item.SkillName, &item.UserID, &item.UserEmail, &item.UserDisplayName, &item.Content, &item.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	item.TenantName = security.DecodeStoredText(item.TenantName)
	item.SkillName = security.DecodeStoredText(item.SkillName)
	item.UserEmail = security.DecodeStoredText(item.UserEmail)
	item.UserDisplayName = security.DecodeStoredText(item.UserDisplayName)
	item.Content = security.DecodeStoredText(item.Content)
	return &item, nil
}

func ListAdminUsers() ([]models.AdminUser, error) {
	rows, err := GetDB().Query(`
		SELECT u.id, u.email, u.display_name, u.status, u.is_platform_admin, u.created_at, u.last_login_at,
		       COUNT(tm.id)
		FROM users u
		LEFT JOIN tenant_members tm ON tm.user_id = u.id
		GROUP BY u.id
		ORDER BY u.created_at ASC, u.id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.AdminUser
	for rows.Next() {
		var (
			item            models.AdminUser
			isPlatformAdmin int
			lastLoginAt     sql.NullTime
		)
		if err := rows.Scan(&item.ID, &item.Email, &item.DisplayName, &item.Status, &isPlatformAdmin, &item.CreatedAt, &lastLoginAt, &item.TenantCount); err != nil {
			return nil, err
		}
		item.Email = security.DecodeStoredText(item.Email)
		item.DisplayName = security.DecodeStoredText(item.DisplayName)
		item.Status = security.DecodeStoredText(item.Status)
		item.IsPlatformAdmin = isPlatformAdmin == 1
		if lastLoginAt.Valid {
			item.LastLoginAt = &lastLoginAt.Time
		}
		users = append(users, item)
	}
	return users, rows.Err()
}

func UpdateUserStatusByAdmin(userID int64, status string) error {
	status = security.EscapePlainText(status)
	_, err := GetDB().Exec(`UPDATE users SET status = ?, updated_at = ? WHERE id = ?`, status, time.Now(), userID)
	return err
}

func CountActivePlatformAdmins() (int, error) {
	var count int
	err := GetDB().QueryRow(`SELECT COUNT(*) FROM users WHERE is_platform_admin = 1 AND status = 'active'`).Scan(&count)
	return count, err
}

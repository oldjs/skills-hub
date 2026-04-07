package db

import (
	"database/sql"
	"fmt"
	"time"

	"skills-hub/models"
	"skills-hub/security"
)

func GetAdminDashboardStats() (*models.AdminDashboardStats, error) {
	stats := &models.AdminDashboardStats{}

	if err := GetDB().QueryRow(`SELECT COUNT(*) FROM skills`).Scan(&stats.TotalSkills); err != nil {
		return nil, err
	}
	if err := GetDB().QueryRow(`SELECT COUNT(*) FROM users`).Scan(&stats.TotalUsers); err != nil {
		return nil, err
	}
	if err := GetDB().QueryRow(`SELECT COUNT(*) FROM skill_comments`).Scan(&stats.TotalComments); err != nil {
		return nil, err
	}
	if err := GetDB().QueryRow(`SELECT COUNT(*) FROM skills WHERE source = 'upload' AND review_status = 'pending'`).Scan(&stats.PendingSkills); err != nil {
		return nil, err
	}

	return stats, nil
}

func ListAdminActionLogs(limit int) ([]models.AdminActionLog, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := GetDB().Query(`
		SELECT l.id, l.admin_user_id, COALESCE(u.display_name, ''), COALESCE(u.email, ''),
		       l.action, l.target_type, l.target_id, l.details, l.created_at
		FROM admin_action_logs l
		LEFT JOIN users u ON u.id = l.admin_user_id
		ORDER BY l.created_at DESC, l.id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.AdminActionLog
	for rows.Next() {
		var item models.AdminActionLog
		if err := rows.Scan(&item.ID, &item.AdminUserID, &item.AdminName, &item.AdminEmail, &item.Action, &item.TargetType, &item.TargetID, &item.Details, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.AdminName = security.DecodeStoredText(item.AdminName)
		item.AdminEmail = security.DecodeStoredText(item.AdminEmail)
		item.Action = security.DecodeStoredText(item.Action)
		item.TargetType = security.DecodeStoredText(item.TargetType)
		item.Details = security.DecodeStoredText(item.Details)
		logs = append(logs, item)
	}

	return logs, rows.Err()
}

func LogAdminAction(adminUserID int64, action, targetType string, targetID int64, details string) error {
	action = security.EscapePlainText(action)
	targetType = security.EscapePlainText(targetType)
	details = security.EscapePlainText(details)

	_, err := GetDB().Exec(`
		INSERT INTO admin_action_logs (admin_user_id, action, target_type, target_id, details, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, adminUserID, action, targetType, targetID, details, time.Now())
	return err
}

func buildActionDetail(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = security.EscapePlainText(part)
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	return fmt.Sprint(filtered)
}

func CountUsersTx(tx *sql.Tx) (int, error) {
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

package db

import (
	"database/sql"
	"errors"

	"skills-hub/models"
	"skills-hub/security"
)

// 添加评论
func AddComment(tenantID, skillID, userID int64, content string) error {
	// 评论按 Markdown 源入库，但先把原始 HTML 转义掉。
	content = security.EscapeMarkdownSource(content)

	result, err := GetDB().Exec(`
		INSERT INTO skill_comments (tenant_id, skill_id, user_id, content)
		SELECT ?, s.id, ?, ?
		FROM skills s
		WHERE s.tenant_id = ? AND s.id = ? AND (s.source != 'upload' OR s.review_status = 'approved')
	`, tenantID, userID, content, tenantID, skillID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrSkillNotFound
	}
	return nil
}

// 拿某个 skill 的所有评论，按时间倒序，带用户信息
func GetSkillComments(tenantID, skillID int64) ([]models.SkillComment, error) {
	rows, err := GetDB().Query(`
		SELECT c.id, c.tenant_id, c.skill_id, c.user_id, c.content, c.created_at,
			   u.email, u.display_name
		FROM skill_comments c
		JOIN users u ON u.id = c.user_id
		WHERE c.tenant_id = ? AND c.skill_id = ?
		ORDER BY c.created_at DESC
	`, tenantID, skillID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []models.SkillComment
	for rows.Next() {
		var c models.SkillComment
		if err := rows.Scan(&c.ID, &c.TenantID, &c.SkillID, &c.UserID, &c.Content, &c.CreatedAt, &c.Email, &c.DisplayName); err != nil {
			return nil, err
		}
		decodeCommentForDisplay(&c)
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

func IsSkillNotFound(err error) bool {
	return errors.Is(err, ErrSkillNotFound) || errors.Is(err, sql.ErrNoRows)
}

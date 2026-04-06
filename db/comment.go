package db

import (
	"skills-hub/models"
)

// 添加评论
func AddComment(tenantID, skillID, userID int64, content string) error {
	_, err := GetDB().Exec(`
		INSERT INTO skill_comments (tenant_id, skill_id, user_id, content)
		VALUES (?, ?, ?, ?)
	`, tenantID, skillID, userID, content)
	return err
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
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

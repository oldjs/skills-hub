package db

import (
	"database/sql"
	"errors"

	"skills-hub/models"
	"skills-hub/security"
)

// 添加评论，支持回复（parentID 非空时为回复）
func AddComment(tenantID, skillID, userID int64, content string, parentID *int64) error {
	// 评论按 Markdown 源入库，但先把原始 HTML 转义掉。
	content = security.EscapeMarkdownSource(content)

	// 如果是回复，校验父评论存在且是顶层评论
	if parentID != nil {
		var parentParentID sql.NullInt64
		err := GetDB().QueryRow(`
			SELECT parent_id FROM skill_comments
			WHERE id = ? AND tenant_id = ? AND skill_id = ?
		`, *parentID, tenantID, skillID).Scan(&parentParentID)
		if err != nil {
			if err == sql.ErrNoRows {
				return ErrSkillNotFound
			}
			return err
		}
		// 不允许回复的回复（只支持一层嵌套）
		if parentParentID.Valid {
			return errors.New("不支持多层嵌套回复")
		}
	}

	result, err := GetDB().Exec(`
		INSERT INTO skill_comments (tenant_id, skill_id, user_id, content, parent_id)
		SELECT ?, s.id, ?, ?, ?
		FROM skills s
		WHERE s.tenant_id = ? AND s.id = ? AND (s.source != 'upload' OR s.review_status = 'approved')
	`, tenantID, userID, content, parentID, tenantID, skillID)
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

// 拿某个 skill 的所有评论，组装成树形结构返回
func GetSkillComments(tenantID, skillID int64) ([]models.SkillComment, error) {
	rows, err := GetDB().Query(`
		SELECT c.id, c.tenant_id, c.skill_id, c.user_id, c.content, c.parent_id, c.created_at,
		       u.email, u.display_name
		FROM skill_comments c
		JOIN users u ON u.id = c.user_id
		WHERE c.tenant_id = ? AND c.skill_id = ?
		ORDER BY c.created_at ASC
	`, tenantID, skillID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var all []models.SkillComment
	for rows.Next() {
		var c models.SkillComment
		var parentID sql.NullInt64
		if err := rows.Scan(&c.ID, &c.TenantID, &c.SkillID, &c.UserID, &c.Content, &parentID, &c.CreatedAt, &c.Email, &c.DisplayName); err != nil {
			return nil, err
		}
		if parentID.Valid {
			pid := parentID.Int64
			c.ParentID = &pid
		}
		decodeCommentForDisplay(&c)
		all = append(all, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 组装树形：顶层评论（按时间倒序）+ 子评论挂到对应顶层下
	topLevel := make([]models.SkillComment, 0)
	replyMap := make(map[int64][]models.SkillComment)

	for _, c := range all {
		if c.ParentID == nil {
			topLevel = append(topLevel, c)
		} else {
			replyMap[*c.ParentID] = append(replyMap[*c.ParentID], c)
		}
	}

	// 倒序展示顶层评论（最新的在前），子评论保持正序
	for i, j := 0, len(topLevel)-1; i < j; i, j = i+1, j-1 {
		topLevel[i], topLevel[j] = topLevel[j], topLevel[i]
	}

	// 把子评论挂到对应顶层评论下
	for i := range topLevel {
		if replies, ok := replyMap[topLevel[i].ID]; ok {
			topLevel[i].Replies = replies
		}
	}

	return topLevel, nil
}

func IsSkillNotFound(err error) bool {
	return errors.Is(err, ErrSkillNotFound) || errors.Is(err, sql.ErrNoRows)
}

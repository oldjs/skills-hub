package db

import (
	"skills-hub/models"
	"skills-hub/security"
	"time"
)

// 拉用户公开 Profile 信息（统计数字一起算）
func GetUserPublicProfile(userID int64) (*models.UserProfile, error) {
	var p models.UserProfile
	err := GetDB().QueryRow(`
		SELECT id, display_name, COALESCE(bio, ''), created_at
		FROM users WHERE id = ? AND status = 'active'
	`, userID).Scan(&p.ID, &p.DisplayName, &p.Bio, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	p.DisplayName = security.DecodeStoredText(p.DisplayName)
	p.Bio = security.DecodeStoredText(p.Bio)

	// 统计上传技能数
	_ = GetDB().QueryRow(`SELECT COUNT(*) FROM skills WHERE author = ? AND source = 'upload' AND review_status = 'approved'`,
		security.EscapePlainText(p.DisplayName)).Scan(&p.SkillCount)
	// 统计评分数
	_ = GetDB().QueryRow(`SELECT COUNT(*) FROM skill_ratings WHERE user_id = ?`, userID).Scan(&p.RatingCount)
	// 统计评论数
	_ = GetDB().QueryRow(`SELECT COUNT(*) FROM skill_comments WHERE user_id = ?`, userID).Scan(&p.CommentCount)

	return &p, nil
}

// 拉用户的评分记录，最新在前
func GetUserRatings(userID int64, limit int) ([]models.UserRatingItem, error) {
	rows, err := GetDB().Query(`
		SELECT s.slug, s.display_name, r.score, r.created_at
		FROM skill_ratings r
		JOIN skills s ON s.id = r.skill_id
		WHERE r.user_id = ?
		ORDER BY r.created_at DESC
		LIMIT ?
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.UserRatingItem
	for rows.Next() {
		var item models.UserRatingItem
		if err := rows.Scan(&item.SkillSlug, &item.SkillName, &item.Score, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.SkillName = security.DecodeStoredText(item.SkillName)
		items = append(items, item)
	}
	return items, rows.Err()
}

// 拉用户的评论记录，最新在前
func GetUserComments(userID int64, limit int) ([]models.UserCommentItem, error) {
	rows, err := GetDB().Query(`
		SELECT s.slug, s.display_name, c.content, c.created_at
		FROM skill_comments c
		JOIN skills s ON s.id = c.skill_id
		WHERE c.user_id = ?
		ORDER BY c.created_at DESC
		LIMIT ?
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.UserCommentItem
	for rows.Next() {
		var item models.UserCommentItem
		if err := rows.Scan(&item.SkillSlug, &item.SkillName, &item.Content, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.SkillName = security.DecodeStoredText(item.SkillName)
		item.Content = security.DecodeStoredText(item.Content)
		// 评论内容截断预览
		if len([]rune(item.Content)) > 100 {
			item.Content = string([]rune(item.Content)[:100]) + "..."
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// 更新用户 Profile（display_name + bio）
func UpdateUserProfile(userID int64, displayName, bio string) error {
	displayName = security.EscapePlainText(displayName)
	bio = security.EscapePlainText(bio)
	_, err := GetDB().Exec(`
		UPDATE users SET display_name = ?, bio = ?, updated_at = ?
		WHERE id = ?
	`, displayName, bio, time.Now(), userID)
	return err
}

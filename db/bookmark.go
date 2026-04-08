package db

import (
	"database/sql"
	"time"

	"skills-hub/models"
)

// 切换收藏状态（有就删，没有就加），返回操作后的状态
func ToggleBookmark(userID, skillID, tenantID int64) (bookmarked bool, err error) {
	// 先查有没有
	var id int64
	err = GetDB().QueryRow(`
		SELECT id FROM skill_bookmarks
		WHERE user_id = ? AND skill_id = ? AND tenant_id = ?
	`, userID, skillID, tenantID).Scan(&id)

	if err == sql.ErrNoRows {
		// 没有，添加收藏
		_, err = GetDB().Exec(`
			INSERT INTO skill_bookmarks (user_id, skill_id, tenant_id) VALUES (?, ?, ?)
		`, userID, skillID, tenantID)
		return true, err
	}
	if err != nil {
		return false, err
	}

	// 有，取消收藏
	_, err = GetDB().Exec(`DELETE FROM skill_bookmarks WHERE id = ?`, id)
	return false, err
}

// 查某个 skill 是否被当前用户收藏
func IsBookmarked(userID, skillID, tenantID int64) bool {
	var count int
	err := GetDB().QueryRow(`
		SELECT COUNT(*) FROM skill_bookmarks
		WHERE user_id = ? AND skill_id = ? AND tenant_id = ?
	`, userID, skillID, tenantID).Scan(&count)
	return err == nil && count > 0
}

// 拉用户的收藏列表，带技能基本信息
func GetUserBookmarks(userID, tenantID int64, limit int) ([]models.Skill, error) {
	rows, err := GetDB().Query(`
		SELECT s.id, s.tenant_id, s.slug, s.display_name, s.summary, s.content, s.score,
		       s.source_updated_at, s.version, s.categories, s.source,
		       COALESCE(AVG(r.score), 0) as avg_rating, COUNT(r.id) as rating_count
		FROM skill_bookmarks b
		JOIN skills s ON s.id = b.skill_id
		LEFT JOIN skill_ratings r ON r.skill_id = s.id AND r.tenant_id = s.tenant_id
		WHERE b.user_id = ? AND b.tenant_id = ?
		  AND (s.source != 'upload' OR s.review_status = 'approved')
		GROUP BY s.id
		ORDER BY b.created_at DESC
		LIMIT ?
	`, userID, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSkillsWithRating(rows)
}

// 复用 querySkillsWithRating 的扫描逻辑
func scanSkillsWithRating(rows *sql.Rows) ([]models.Skill, error) {
	var skills []models.Skill
	for rows.Next() {
		var skill models.Skill
		var updatedAt int64
		if err := rows.Scan(&skill.ID, &skill.TenantID, &skill.Slug, &skill.DisplayName, &skill.Summary, &skill.Content, &skill.Score, &updatedAt, &skill.Version, &skill.Categories, &skill.Source, &skill.AvgRating, &skill.RatingCount); err != nil {
			return nil, err
		}
		skill.UpdatedAt = time.Unix(updatedAt, 0)
		decodeSkillForDisplay(&skill)
		skills = append(skills, skill)
	}
	return skills, rows.Err()
}

package db

import (
	"skills-hub/models"
)

// LeaderboardEntry 排行榜条目
type LeaderboardEntry struct {
	Skill       models.Skill
	WeeklyScore float64 // 近 7 天的综合分
}

// 热门技能排行榜：综合评分、下载量、近期评论活跃度
func GetSkillLeaderboard(tenantID int64, limit int) ([]models.Skill, error) {
	// 按综合热度排序：用户评分 * 权重 + 近 7 天评论数 + 下载量归一化
	statement := `
		SELECT s.id, s.tenant_id, s.slug, s.display_name, s.summary, s.content, s.score,
		       s.source_updated_at, s.version, s.categories, s.source,
		       COALESCE(AVG(r.score), 0) as avg_rating, COUNT(r.id) as rating_count
		FROM skills s
		LEFT JOIN skill_ratings r ON r.skill_id = s.id AND r.tenant_id = s.tenant_id
		WHERE s.tenant_id = ? AND (s.source != 'upload' OR s.review_status = 'approved')
		GROUP BY s.id
		HAVING avg_rating > 0 OR s.download_count > 0
		ORDER BY
			avg_rating * 2 + COALESCE(s.download_count, 0) * 0.01 + s.score DESC,
			rating_count DESC,
			s.display_name ASC
		LIMIT ?
	`
	return querySkillsWithRating(statement, tenantID, limit)
}

// 近期最活跃（7 天内收到最多评论的技能）
func GetRecentlyActiveSkills(tenantID int64, limit int) ([]models.Skill, error) {
	statement := `
		SELECT s.id, s.tenant_id, s.slug, s.display_name, s.summary, s.content, s.score,
		       s.source_updated_at, s.version, s.categories, s.source,
		       COALESCE(AVG(r.score), 0) as avg_rating, COUNT(DISTINCT r.id) as rating_count
		FROM skills s
		LEFT JOIN skill_ratings r ON r.skill_id = s.id AND r.tenant_id = s.tenant_id
		LEFT JOIN skill_comments c ON c.skill_id = s.id AND c.tenant_id = s.tenant_id
		     AND c.created_at >= datetime('now', '-7 days')
		WHERE s.tenant_id = ? AND (s.source != 'upload' OR s.review_status = 'approved')
		GROUP BY s.id
		HAVING COUNT(DISTINCT c.id) > 0
		ORDER BY COUNT(DISTINCT c.id) DESC, avg_rating DESC
		LIMIT ?
	`
	return querySkillsWithRating(statement, tenantID, limit)
}

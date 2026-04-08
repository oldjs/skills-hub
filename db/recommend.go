package db

import (
	"strings"

	"skills-hub/models"
	"skills-hub/security"
)

// 按分类匹配推荐相关技能，排除自身，按评分排序
func GetRelatedSkills(tenantID, excludeSkillID int64, categories string, limit int) ([]models.Skill, error) {
	// 拆出所有分类做 LIKE 匹配
	cats := splitCategories(categories)
	if len(cats) == 0 {
		return nil, nil
	}

	// 基础查询，和 buildFilteredSkillsQuery 保持一致的 SELECT 结构
	statement := `
		SELECT s.id, s.tenant_id, s.slug, s.display_name, s.summary, s.content, s.score,
		       s.source_updated_at, s.version, s.categories, s.source,
		       COALESCE(AVG(r.score), 0) as avg_rating, COUNT(r.id) as rating_count
		FROM skills s
		LEFT JOIN skill_ratings r ON r.skill_id = s.id AND r.tenant_id = s.tenant_id
		WHERE s.tenant_id = ?
		  AND s.id != ?
		  AND (s.source != 'upload' OR s.review_status = 'approved')
		  AND (`

	args := []interface{}{tenantID, excludeSkillID}

	// 拼分类 OR 条件
	var orClauses []string
	for _, cat := range cats {
		orClauses = append(orClauses, "s.categories LIKE ?")
		args = append(args, "%"+security.EscapePlainText(cat)+"%")
	}
	statement += strings.Join(orClauses, " OR ")
	statement += `)
		GROUP BY s.id
		ORDER BY avg_rating DESC, s.score DESC, s.display_name ASC
		LIMIT ?`
	args = append(args, limit)

	return querySkillsWithRating(statement, args...)
}

// 把逗号分隔的 categories 拆开去空
func splitCategories(categories string) []string {
	var result []string
	for _, cat := range strings.Split(categories, ",") {
		cat = strings.TrimSpace(cat)
		if cat != "" {
			result = append(result, cat)
		}
	}
	return result
}

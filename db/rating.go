package db

import (
	"database/sql"

	"skills-hub/models"
)

// 添加或更新评分，每用户每skill只能评一次
func AddRating(tenantID, skillID, userID int64, score int) error {
	result, err := GetDB().Exec(`
		INSERT INTO skill_ratings (tenant_id, skill_id, user_id, score)
		SELECT ?, s.id, ?, ?
		FROM skills s
		WHERE s.tenant_id = ? AND s.id = ?
		ON CONFLICT(tenant_id, skill_id, user_id) DO UPDATE SET
			score = excluded.score
	`, tenantID, userID, score, tenantID, skillID)
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

// 查当前用户对某个 skill 的评分
func GetUserRating(tenantID, skillID, userID int64) (int, error) {
	var score int
	err := GetDB().QueryRow(`
		SELECT score FROM skill_ratings
		WHERE tenant_id = ? AND skill_id = ? AND user_id = ?
	`, tenantID, skillID, userID).Scan(&score)

	if err == sql.ErrNoRows {
		return 0, nil
	}
	return score, err
}

// 拿某个 skill 的平均评分和评分人数
func GetSkillRatingStats(tenantID, skillID int64) (float64, int, error) {
	var avg sql.NullFloat64
	var count int
	err := GetDB().QueryRow(`
		SELECT AVG(score), COUNT(*) FROM skill_ratings
		WHERE tenant_id = ? AND skill_id = ?
	`, tenantID, skillID).Scan(&avg, &count)

	if err != nil {
		return 0, 0, err
	}
	if !avg.Valid {
		return 0, 0, nil
	}
	return avg.Float64, count, nil
}

// 批量拿多个 skill 的评分统计，搜索列表用
func GetSkillsRatingStats(tenantID int64, skillIDs []int64) (map[int64]models.Skill, error) {
	if len(skillIDs) == 0 {
		return map[int64]models.Skill{}, nil
	}

	// 拼 IN 查询的占位符
	placeholders := make([]string, len(skillIDs))
	args := make([]interface{}, 0, len(skillIDs)+1)
	args = append(args, tenantID)
	for i, id := range skillIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}

	query := `
		SELECT skill_id, AVG(score), COUNT(*)
		FROM skill_ratings
		WHERE tenant_id = ? AND skill_id IN (` + joinStrings(placeholders, ",") + `)
		GROUP BY skill_id
	`

	rows, err := GetDB().Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64]models.Skill)
	for rows.Next() {
		var skillID int64
		var avg float64
		var count int
		if err := rows.Scan(&skillID, &avg, &count); err != nil {
			return nil, err
		}
		result[skillID] = models.Skill{AvgRating: avg, RatingCount: count}
	}

	return result, rows.Err()
}

// 简单的 join 工具函数
func joinStrings(parts []string, sep string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += sep
		}
		result += part
	}
	return result
}

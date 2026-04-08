package db

import "time"

// sitemap 用的轻量结构，只需要 slug 和 更新时间
type SitemapSkill struct {
	Slug      string
	UpdatedAt time.Time
}

// 拉所有已审核通过的技能 slug，给 sitemap 用
func GetAllApprovedSkillSlugs() ([]SitemapSkill, error) {
	rows, err := GetDB().Query(`
		SELECT DISTINCT slug, MAX(updated_at) as updated_at
		FROM skills
		WHERE source != 'upload' OR review_status = 'approved'
		GROUP BY slug
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var skills []SitemapSkill
	for rows.Next() {
		var s SitemapSkill
		var updatedStr string
		if err := rows.Scan(&s.Slug, &updatedStr); err != nil {
			return nil, err
		}
		s.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedStr)
		if s.UpdatedAt.IsZero() {
			s.UpdatedAt = time.Now()
		}
		skills = append(skills, s)
	}
	return skills, rows.Err()
}

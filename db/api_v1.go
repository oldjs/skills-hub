package db

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"skills-hub/security"
)

type APISkillRecord struct {
	ID            int64
	TenantID      int64
	Slug          string
	Name          string
	Description   string
	Version       string
	Author        string
	Source        string
	Content       string
	Categories    string
	DownloadCount int
	CreatedAt     time.Time
	RatingAvg     float64
	RatingCount   int
}

type PlatformStats struct {
	TotalSkills   int `json:"total_skills"`
	TotalUsers    int `json:"total_users"`
	TotalTenants  int `json:"total_tenants"`
	TotalComments int `json:"total_comments"`
	TotalRatings  int `json:"total_ratings"`
}

func SearchSkillsForAPI(query, sortBy string, tenantID *int64) ([]APISkillRecord, error) {
	rows, err := querySkillRowsForAPI(query, tenantID, "")
	if err != nil {
		return nil, err
	}

	skills := mergeAPISkillRows(rows)
	sortAPISkills(skills, sortBy)
	return skills, nil
}

func GetSkillBySlugForAPI(slug string, tenantID *int64) (*APISkillRecord, error) {
	if tenantID != nil && *tenantID > 0 {
		rows, err := queryUploadedSkillRowsBySlug(*tenantID, slug)
		if err != nil {
			return nil, err
		}
		if len(rows) > 0 {
			merged := mergeAPISkillRows(rows)
			if len(merged) > 0 {
				return &merged[0], nil
			}
		}
	}

	rows, err := queryPublicSkillRowsBySlug(slug)
	if err != nil {
		return nil, err
	}
	merged := mergeAPISkillRows(rows)
	if len(merged) == 0 {
		return nil, nil
	}
	return &merged[0], nil
}

func ListCategoryCountsForAPI(tenantID *int64) (map[string]int, error) {
	skills, err := SearchSkillsForAPI("", "rating", tenantID)
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int)
	for _, skill := range skills {
		for _, category := range strings.Split(skill.Categories, ",") {
			category = strings.TrimSpace(category)
			if category == "" {
				continue
			}
			counts[category]++
		}
	}
	return counts, nil
}

func GetPlatformStatsForAPI() (*PlatformStats, error) {
	stats := &PlatformStats{}

	if err := GetDB().QueryRow(`SELECT COUNT(DISTINCT slug) FROM skills WHERE source != 'upload'`).Scan(&stats.TotalSkills); err != nil {
		return nil, err
	}

	var uploadedCount int
	if err := GetDB().QueryRow(`SELECT COUNT(*) FROM skills WHERE source = 'upload'`).Scan(&uploadedCount); err != nil {
		return nil, err
	}
	stats.TotalSkills += uploadedCount

	if err := GetDB().QueryRow(`SELECT COUNT(*) FROM users`).Scan(&stats.TotalUsers); err != nil {
		return nil, err
	}
	if err := GetDB().QueryRow(`SELECT COUNT(*) FROM tenants`).Scan(&stats.TotalTenants); err != nil {
		return nil, err
	}
	if err := GetDB().QueryRow(`SELECT COUNT(*) FROM skill_comments`).Scan(&stats.TotalComments); err != nil {
		return nil, err
	}
	if err := GetDB().QueryRow(`SELECT COUNT(*) FROM skill_ratings`).Scan(&stats.TotalRatings); err != nil {
		return nil, err
	}

	return stats, nil
}

func IncrementSkillDownloadCount(skillID int64) error {
	_, err := GetDB().Exec(`UPDATE skills SET download_count = download_count + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, skillID)
	return err
}

type apiSkillRow struct {
	APISkillRecord
	RatingSum float64
}

func querySkillRowsForAPI(query string, tenantID *int64, slug string) ([]apiSkillRow, error) {
	statement := `
		SELECT s.id, s.tenant_id, s.slug, s.display_name, s.summary, s.version,
		       COALESCE(s.author, ''), s.source, COALESCE(s.content, ''), COALESCE(s.categories, ''),
		       COALESCE(s.download_count, 0), s.created_at, COALESCE(SUM(r.score), 0), COUNT(r.id)
		FROM skills s
		LEFT JOIN skill_ratings r ON r.skill_id = s.id AND r.tenant_id = s.tenant_id
		WHERE (s.source != 'upload'`
	args := make([]interface{}, 0, 8)
	if tenantID != nil && *tenantID > 0 {
		statement += ` OR (s.source = 'upload' AND s.tenant_id = ?)`
		args = append(args, *tenantID)
	}
	statement += `)`

	query = security.EscapePlainText(query)
	if query != "" {
		pattern := "%" + query + "%"
		statement += ` AND (s.slug LIKE ? OR s.display_name LIKE ? OR s.summary LIKE ? OR s.categories LIKE ? OR s.version LIKE ? OR COALESCE(s.author, '') LIKE ?)`
		args = append(args, pattern, pattern, pattern, pattern, pattern, pattern)
	}
	if slug != "" {
		statement += ` AND s.slug = ?`
		args = append(args, slug)
	}
	statement += ` GROUP BY s.id, s.tenant_id, s.slug, s.display_name, s.summary, s.version, s.author, s.source, s.content, s.categories, s.download_count, s.created_at`

	rows, err := GetDB().Query(statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []apiSkillRow
	for rows.Next() {
		var row apiSkillRow
		if err := rows.Scan(
			&row.ID,
			&row.TenantID,
			&row.Slug,
			&row.Name,
			&row.Description,
			&row.Version,
			&row.Author,
			&row.Source,
			&row.Content,
			&row.Categories,
			&row.DownloadCount,
			&row.CreatedAt,
			&row.RatingSum,
			&row.RatingCount,
		); err != nil {
			return nil, err
		}
		row.Name = security.DecodeStoredText(row.Name)
		row.Description = security.DecodeStoredText(row.Description)
		row.Version = security.DecodeStoredText(row.Version)
		row.Author = security.DecodeStoredText(row.Author)
		row.Source = security.DecodeStoredText(row.Source)
		row.Categories = security.DecodeStoredText(row.Categories)
		result = append(result, row)
	}

	return result, rows.Err()
}

func queryUploadedSkillRowsBySlug(tenantID int64, slug string) ([]apiSkillRow, error) {
	statement := `
		SELECT s.id, s.tenant_id, s.slug, s.display_name, s.summary, s.version,
		       COALESCE(s.author, ''), s.source, COALESCE(s.content, ''), COALESCE(s.categories, ''),
		       COALESCE(s.download_count, 0), s.created_at, COALESCE(SUM(r.score), 0), COUNT(r.id)
		FROM skills s
		LEFT JOIN skill_ratings r ON r.skill_id = s.id AND r.tenant_id = s.tenant_id
		WHERE s.source = 'upload' AND s.tenant_id = ? AND s.slug = ?
		GROUP BY s.id, s.tenant_id, s.slug, s.display_name, s.summary, s.version, s.author, s.source, s.content, s.categories, s.download_count, s.created_at
	`

	rows, err := GetDB().Query(statement, tenantID, slug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []apiSkillRow
	for rows.Next() {
		var row apiSkillRow
		if err := rows.Scan(
			&row.ID,
			&row.TenantID,
			&row.Slug,
			&row.Name,
			&row.Description,
			&row.Version,
			&row.Author,
			&row.Source,
			&row.Content,
			&row.Categories,
			&row.DownloadCount,
			&row.CreatedAt,
			&row.RatingSum,
			&row.RatingCount,
		); err != nil {
			return nil, err
		}
		row.Name = security.DecodeStoredText(row.Name)
		row.Description = security.DecodeStoredText(row.Description)
		row.Version = security.DecodeStoredText(row.Version)
		row.Author = security.DecodeStoredText(row.Author)
		row.Source = security.DecodeStoredText(row.Source)
		row.Categories = security.DecodeStoredText(row.Categories)
		result = append(result, row)
	}

	return result, rows.Err()
}

func queryPublicSkillRowsBySlug(slug string) ([]apiSkillRow, error) {
	return querySkillRowsForAPI("", nil, slug)
}

func mergeAPISkillRows(rows []apiSkillRow) []APISkillRecord {
	acc := make(map[string]*apiSkillRow)
	order := make([]string, 0, len(rows))

	for _, row := range rows {
		key := row.Source + ":" + row.Slug
		if row.Source == "upload" {
			key = row.Source + ":" + row.Slug + ":" + int64Key(row.TenantID)
		}

		existing := acc[key]
		if existing == nil {
			copy := row
			acc[key] = &copy
			order = append(order, key)
			continue
		}

		existing.RatingSum += row.RatingSum
		existing.RatingCount += row.RatingCount
		existing.DownloadCount += row.DownloadCount
		if row.CreatedAt.After(existing.CreatedAt) {
			existing.CreatedAt = row.CreatedAt
		}
		if existing.Content == "" && row.Content != "" {
			existing.Content = row.Content
		}
		if existing.Description == "" && row.Description != "" {
			existing.Description = row.Description
		}
		if existing.Version == "" && row.Version != "" {
			existing.Version = row.Version
		}
		if existing.Author == "" && row.Author != "" {
			existing.Author = row.Author
		}
		if existing.Categories == "" && row.Categories != "" {
			existing.Categories = row.Categories
		}
	}

	result := make([]APISkillRecord, 0, len(order))
	for _, key := range order {
		row := acc[key]
		if row == nil {
			continue
		}
		if row.RatingCount > 0 {
			row.RatingAvg = row.RatingSum / float64(row.RatingCount)
		}
		if row.Author == "" && row.Source != "upload" {
			row.Author = "ClawHub"
		}
		result = append(result, row.APISkillRecord)
	}
	return result
}

func sortAPISkills(skills []APISkillRecord, sortBy string) {
	sort.Slice(skills, func(i, j int) bool {
		left := skills[i]
		right := skills[j]

		switch sortBy {
		case "newest":
			if left.CreatedAt.Equal(right.CreatedAt) {
				return left.Name < right.Name
			}
			return left.CreatedAt.After(right.CreatedAt)
		default:
			if left.RatingAvg == right.RatingAvg {
				if left.RatingCount == right.RatingCount {
					return left.Name < right.Name
				}
				return left.RatingCount > right.RatingCount
			}
			return left.RatingAvg > right.RatingAvg
		}
	})
}

func int64Key(value int64) string {
	return strconv.FormatInt(value, 10)
}

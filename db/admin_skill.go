package db

import (
	"database/sql"
	"time"

	"skills-hub/models"
	"skills-hub/security"
)

func ListAdminSkills(status string) ([]models.AdminSkill, error) {
	query := `
		SELECT s.id, s.tenant_id, COALESCE(t.name, ''), s.slug, s.display_name, s.summary, COALESCE(s.content, ''),
		       s.version, s.categories, COALESCE(s.author, ''), s.source, COALESCE(s.review_status, 'approved'),
		       COALESCE(s.review_note, ''), COALESCE(reviewer.display_name, ''), s.reviewed_at, s.created_at, s.updated_at,
		       COALESCE(AVG(r.score), 0), COUNT(r.id)
		FROM skills s
		LEFT JOIN tenants t ON t.id = s.tenant_id
		LEFT JOIN users reviewer ON reviewer.id = s.reviewed_by
		LEFT JOIN skill_ratings r ON r.skill_id = s.id AND r.tenant_id = s.tenant_id
		WHERE s.source = 'upload'
	`
	args := make([]interface{}, 0, 1)
	if status != "" && status != "all" {
		query += ` AND COALESCE(s.review_status, 'approved') = ?`
		args = append(args, status)
	}
	query += ` GROUP BY s.id ORDER BY CASE COALESCE(s.review_status, 'approved') WHEN 'pending' THEN 0 WHEN 'rejected' THEN 1 ELSE 2 END, s.updated_at DESC`

	rows, err := GetDB().Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var skills []models.AdminSkill
	for rows.Next() {
		item, err := scanAdminSkill(rows)
		if err != nil {
			return nil, err
		}
		skills = append(skills, item)
	}
	return skills, rows.Err()
}

func GetAdminSkillByID(skillID int64) (*models.AdminSkill, error) {
	row := GetDB().QueryRow(`
		SELECT s.id, s.tenant_id, COALESCE(t.name, ''), s.slug, s.display_name, s.summary, COALESCE(s.content, ''),
		       s.version, s.categories, COALESCE(s.author, ''), s.source, COALESCE(s.review_status, 'approved'),
		       COALESCE(s.review_note, ''), COALESCE(reviewer.display_name, ''), s.reviewed_at, s.created_at, s.updated_at,
		       COALESCE(AVG(r.score), 0), COUNT(r.id)
		FROM skills s
		LEFT JOIN tenants t ON t.id = s.tenant_id
		LEFT JOIN users reviewer ON reviewer.id = s.reviewed_by
		LEFT JOIN skill_ratings r ON r.skill_id = s.id AND r.tenant_id = s.tenant_id
		WHERE s.id = ? AND s.source = 'upload'
		GROUP BY s.id
	`, skillID)

	item, err := scanAdminSkill(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func UpdateAdminSkill(skillID int64, displayName, summary, version, categories, author, content string) error {
	displayName = security.EscapePlainText(displayName)
	summary = security.EscapePlainText(summary)
	version = security.EscapePlainText(version)
	categories = security.EscapePlainText(categories)
	author = security.EscapePlainText(author)
	content = security.EscapeMarkdownSource(content)

	_, err := GetDB().Exec(`
		UPDATE skills
		SET display_name = ?, summary = ?, version = ?, categories = ?, author = ?, content = ?, updated_at = ?
		WHERE id = ?
	`, displayName, summary, version, categories, author, content, time.Now(), skillID)
	return err
}

func ReviewAdminSkill(skillID, reviewerID int64, status, note string) error {
	status = security.EscapePlainText(status)
	note = security.EscapePlainText(note)

	_, err := GetDB().Exec(`
		UPDATE skills
		SET review_status = ?, review_note = ?, reviewed_by = ?, reviewed_at = ?, updated_at = ?
		WHERE id = ?
	`, status, note, reviewerID, time.Now(), time.Now(), skillID)
	return err
}

func GetPendingSkillCount() (int, error) {
	var count int
	err := GetDB().QueryRow(`SELECT COUNT(*) FROM skills WHERE source = 'upload' AND review_status = 'pending'`).Scan(&count)
	return count, err
}

func scanAdminSkill(scanner interface {
	Scan(dest ...interface{}) error
}) (models.AdminSkill, error) {
	var (
		item       models.AdminSkill
		reviewedAt sql.NullTime
	)
	if err := scanner.Scan(&item.ID, &item.TenantID, &item.TenantName, &item.Slug, &item.DisplayName, &item.Summary, &item.Content, &item.Version, &item.Categories, &item.Author, &item.Source, &item.ReviewStatus, &item.ReviewNote, &item.ReviewerName, &reviewedAt, &item.CreatedAt, &item.UpdatedAt, &item.AvgRating, &item.RatingCount); err != nil {
		return models.AdminSkill{}, err
	}
	item.TenantName = security.DecodeStoredText(item.TenantName)
	item.DisplayName = security.DecodeStoredText(item.DisplayName)
	item.Summary = security.DecodeStoredText(item.Summary)
	item.Content = security.DecodeStoredText(item.Content)
	item.Version = security.DecodeStoredText(item.Version)
	item.Categories = security.DecodeStoredText(item.Categories)
	item.Author = security.DecodeStoredText(item.Author)
	item.Source = security.DecodeStoredText(item.Source)
	item.ReviewStatus = security.DecodeStoredText(item.ReviewStatus)
	item.ReviewNote = security.DecodeStoredText(item.ReviewNote)
	item.ReviewerName = security.DecodeStoredText(item.ReviewerName)
	if reviewedAt.Valid {
		item.ReviewedAt = &reviewedAt.Time
	}
	return item, nil
}

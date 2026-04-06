package db

import (
	"fmt"
	"time"

	"skills-hub/models"
	"skills-hub/security"
)

// 保存用户上传的 skill，来源标记为 upload
func SaveUploadedSkill(tenantID int64, slug, displayName, summary, content, version, categories string) (*models.Skill, error) {
	// 上传包里的文本同样不可信，入库前先转义。
	displayName = security.EscapePlainText(displayName)
	summary = security.EscapePlainText(summary)
	content = security.EscapeMarkdownSource(content)
	version = security.EscapePlainText(version)
	categories = security.EscapePlainText(categories)

	// 先看有没有同 slug 的
	existing, err := GetSkillBySlug(tenantID, slug)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("slug '%s' 已存在，请换一个名字", slug)
	}

	now := time.Now()
	result, err := GetDB().Exec(`
		INSERT INTO skills (tenant_id, slug, display_name, summary, content, score, source_updated_at, version, categories, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?, 'upload', ?, ?)
	`, tenantID, slug, displayName, summary, content, now.Unix(), version, categories, now, now)
	if err != nil {
		return nil, err
	}

	skillID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &models.Skill{
		ID:          skillID,
		TenantID:    tenantID,
		Slug:        slug,
		DisplayName: security.DecodeStoredText(displayName),
		Summary:     security.DecodeStoredText(summary),
		Content:     content,
		Version:     security.DecodeStoredText(version),
		Categories:  security.DecodeStoredText(categories),
		Source:      "upload",
		UpdatedAt:   now,
	}, nil
}

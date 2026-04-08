package db

import (
	"log"
	"time"

	"skills-hub/security"
)

// SkillVersion 技能的一个历史版本
type SkillVersion struct {
	ID        int64     `json:"id"`
	SkillID   int64     `json:"skillId"`
	TenantID  int64     `json:"tenantId"`
	Version   string    `json:"version"`
	Summary   string    `json:"summary"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

// 保存当前 skill 状态为一个版本快照（在更新前调用）
func SaveSkillVersion(skillID, tenantID int64) {
	var version, summary, content string
	err := GetDB().QueryRow(`
		SELECT COALESCE(version, ''), COALESCE(summary, ''), COALESCE(content, '')
		FROM skills WHERE id = ? AND tenant_id = ?
	`, skillID, tenantID).Scan(&version, &summary, &content)
	if err != nil {
		log.Printf("save skill version: read failed for skill %d: %v", skillID, err)
		return
	}
	// 没内容就不存版本
	if content == "" && summary == "" {
		return
	}

	_, err = GetDB().Exec(`
		INSERT INTO skill_versions (skill_id, tenant_id, version, summary, content)
		VALUES (?, ?, ?, ?, ?)
	`, skillID, tenantID, version, summary, content)
	if err != nil {
		log.Printf("save skill version failed for skill %d: %v", skillID, err)
	}
}

// 拉某 skill 的版本历史
func GetSkillVersions(skillID int64) ([]SkillVersion, error) {
	rows, err := GetDB().Query(`
		SELECT id, skill_id, tenant_id, version, summary, content, created_at
		FROM skill_versions WHERE skill_id = ?
		ORDER BY created_at DESC LIMIT 20
	`, skillID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []SkillVersion
	for rows.Next() {
		var v SkillVersion
		if err := rows.Scan(&v.ID, &v.SkillID, &v.TenantID, &v.Version, &v.Summary, &v.Content, &v.CreatedAt); err != nil {
			return nil, err
		}
		v.Version = security.DecodeStoredText(v.Version)
		v.Summary = security.DecodeStoredText(v.Summary)
		v.Content = security.DecodeStoredText(v.Content)
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

// 回滚 skill 到某个历史版本
func RollbackSkillToVersion(skillID, versionID int64) error {
	var version, summary, content string
	var tenantID int64
	err := GetDB().QueryRow(`
		SELECT tenant_id, version, summary, content
		FROM skill_versions WHERE id = ? AND skill_id = ?
	`, versionID, skillID).Scan(&tenantID, &version, &summary, &content)
	if err != nil {
		return err
	}

	// 先存当前版本为快照
	SaveSkillVersion(skillID, tenantID)

	// 回滚
	_, err = GetDB().Exec(`
		UPDATE skills SET version = ?, summary = ?, content = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, version, summary, content, skillID)
	if err != nil {
		return err
	}

	// 同步 FTS
	SyncSkillToFTS(skillID)
	return nil
}

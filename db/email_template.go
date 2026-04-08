package db

import (
	"database/sql"
	"time"
)

type EmailTemplate struct {
	ID        string    `json:"id"`
	Subject   string    `json:"subject"`
	BodyHTML  string    `json:"bodyHtml"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// 按 ID 拿邮件模板
func GetEmailTemplate(id string) (*EmailTemplate, error) {
	var t EmailTemplate
	err := GetDB().QueryRow(`SELECT id, subject, body_html, updated_at FROM email_templates WHERE id = ?`, id).
		Scan(&t.ID, &t.Subject, &t.BodyHTML, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &t, err
}

// 更新邮件模板
func UpdateEmailTemplate(id, subject, bodyHTML string) error {
	_, err := GetDB().Exec(`UPDATE email_templates SET subject = ?, body_html = ?, updated_at = ? WHERE id = ?`,
		subject, bodyHTML, time.Now(), id)
	return err
}

// 列出所有模板
func ListEmailTemplates() ([]EmailTemplate, error) {
	rows, err := GetDB().Query(`SELECT id, subject, body_html, updated_at FROM email_templates ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []EmailTemplate
	for rows.Next() {
		var t EmailTemplate
		if err := rows.Scan(&t.ID, &t.Subject, &t.BodyHTML, &t.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

package handlers

import (
	"fmt"
	"log"

	"skills-hub/db"
)

// 技能审核结果通知上传者
func notifySkillReviewResult(skillID int64, status, reviewNote string) {
	// 查 skill 信息找到租户，再找 owner
	skill, err := db.GetAdminSkillByID(skillID)
	if err != nil || skill == nil {
		return
	}

	// 找到租户的 owner（上传者大概率是 owner 或 admin）
	members, err := db.ListTenantMembers(skill.TenantID)
	if err != nil {
		return
	}

	title := "你的技能「" + skill.DisplayName + "」已通过审核"
	nType := "review_approved"
	if status == "rejected" {
		title = "你的技能「" + skill.DisplayName + "」未通过审核"
		nType = "review_rejected"
	}

	content := ""
	if reviewNote != "" {
		content = "审核备注: " + reviewNote
	}
	link := "/skill?slug=" + skill.Slug

	// 通知租户所有成员（实际上应该只通知上传者，但目前没有 uploaded_by 字段，通知 owner 即可）
	for _, m := range members {
		if m.Role == "owner" || m.Role == "admin" {
			if err := db.CreateNotification(m.UserID, nType, title, content, link); err != nil {
				log.Printf("notify skill review failed for user %d: %v", m.UserID, err)
			}
		}
	}
}

// 评论回复通知父评论作者
func notifyCommentReply(tenantID, skillID int64, parentID *int64, replierUserID int64) {
	if parentID == nil {
		return
	}

	// 查父评论的作者
	var parentUserID int64
	var skillSlug, skillName string
	err := db.GetDB().QueryRow(`
		SELECT c.user_id, s.slug, s.display_name
		FROM skill_comments c
		JOIN skills s ON s.id = c.skill_id
		WHERE c.id = ? AND c.tenant_id = ?
	`, *parentID, tenantID).Scan(&parentUserID, &skillSlug, &skillName)
	if err != nil {
		return
	}

	// 不通知自己回复自己
	if parentUserID == replierUserID {
		return
	}

	// 查回复者名字
	replier, _ := db.GetUserByID(replierUserID)
	replierName := "有人"
	if replier != nil && replier.DisplayName != "" {
		replierName = replier.DisplayName
	}

	title := fmt.Sprintf("%s 回复了你在「%s」的评论", replierName, skillName)
	link := "/skill?slug=" + skillSlug
	if err := db.CreateNotification(parentUserID, "comment_reply", title, "", link); err != nil {
		log.Printf("notify comment reply failed: %v", err)
	}
}

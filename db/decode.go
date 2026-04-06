package db

import (
	"skills-hub/models"
	"skills-hub/security"
)

func decodeSkillForDisplay(skill *models.Skill) {
	if skill == nil {
		return
	}

	// 这些字段模板里按普通文本展示，读出来时还原一下最稳。
	skill.DisplayName = security.DecodeStoredText(skill.DisplayName)
	skill.Summary = security.DecodeStoredText(skill.Summary)
	skill.Version = security.DecodeStoredText(skill.Version)
	skill.Categories = security.DecodeStoredText(skill.Categories)
	skill.Source = security.DecodeStoredText(skill.Source)
}

func decodeCommentForDisplay(comment *models.SkillComment) {
	if comment == nil {
		return
	}

	comment.Email = security.DecodeStoredText(comment.Email)
	comment.DisplayName = security.DecodeStoredText(comment.DisplayName)
}

func decodeUserForDisplay(user *models.User) {
	if user == nil {
		return
	}

	user.Email = security.DecodeStoredText(user.Email)
	user.DisplayName = security.DecodeStoredText(user.DisplayName)
	user.Status = security.DecodeStoredText(user.Status)
}

func decodeTenantForDisplay(tenant *models.Tenant) {
	if tenant == nil {
		return
	}

	tenant.Slug = security.DecodeStoredText(tenant.Slug)
	tenant.Name = security.DecodeStoredText(tenant.Name)
	tenant.Description = security.DecodeStoredText(tenant.Description)
	tenant.Status = security.DecodeStoredText(tenant.Status)
}

func decodeUserTenantForDisplay(tenant *models.UserTenant) {
	if tenant == nil {
		return
	}

	tenant.TenantSlug = security.DecodeStoredText(tenant.TenantSlug)
	tenant.TenantName = security.DecodeStoredText(tenant.TenantName)
	tenant.TenantStatus = security.DecodeStoredText(tenant.TenantStatus)
	tenant.TenantRole = security.DecodeStoredText(tenant.TenantRole)
	tenant.MembershipStatus = security.DecodeStoredText(tenant.MembershipStatus)
}

func decodeTenantMemberForDisplay(member *models.TenantMember) {
	if member == nil {
		return
	}

	member.Role = security.DecodeStoredText(member.Role)
	member.Status = security.DecodeStoredText(member.Status)
	member.Email = security.DecodeStoredText(member.Email)
	member.DisplayName = security.DecodeStoredText(member.DisplayName)
}

func decodeTenantInviteForDisplay(invite *models.TenantInvite) {
	if invite == nil {
		return
	}

	invite.Email = security.DecodeStoredText(invite.Email)
	invite.Role = security.DecodeStoredText(invite.Role)
	invite.Status = security.DecodeStoredText(invite.Status)
}

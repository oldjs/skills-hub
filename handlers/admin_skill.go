package handlers

import (
	"net/http"
	"strings"

	"skills-hub/db"
)

func AdminSkillsHandler(w http.ResponseWriter, r *http.Request) {
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status == "" {
		status = "pending"
	}

	skills, err := db.ListAdminSkills(status)
	if err != nil {
		http.Error(w, "Skills 列表加载失败", http.StatusInternalServerError)
		return
	}

	renderAdminPage(w, r, "admin_skills.html", PageData{
		Title:        "Skills 审核 - Skills Hub",
		AdminSection: "skills",
		AdminSkills:  skills,
		StatusFilter: status,
		Info:         r.URL.Query().Get("info"),
		Error:        r.URL.Query().Get("error"),
	})
}

func AdminSkillDetailHandler(w http.ResponseWriter, r *http.Request) {
	skillID, err := parseInt64(r.URL.Query().Get("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	skill, err := db.GetAdminSkillByID(skillID)
	if err != nil {
		http.Error(w, "Skill 详情加载失败", http.StatusInternalServerError)
		return
	}
	if skill == nil {
		http.NotFound(w, r)
		return
	}

	renderAdminPage(w, r, "admin_skill_detail.html", PageData{
		Title:        skill.DisplayName + " - Skill 审核",
		AdminSection: "skills",
		AdminSkill:   skill,
		Info:         r.URL.Query().Get("info"),
		Error:        r.URL.Query().Get("error"),
	})
}

func AdminSkillUpdateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}

	skillID, err := parseInt64(r.FormValue("skill_id"))
	if err != nil {
		http.Redirect(w, r, "/admin/skills?error=参数错误", http.StatusSeeOther)
		return
	}

	if strings.TrimSpace(r.FormValue("display_name")) == "" {
		http.Redirect(w, r, "/admin/skill?id="+r.FormValue("skill_id")+"&error=显示名称不能为空", http.StatusSeeOther)
		return
	}

	err = db.UpdateAdminSkill(
		skillID,
		strings.TrimSpace(r.FormValue("display_name")),
		strings.TrimSpace(r.FormValue("summary")),
		strings.TrimSpace(r.FormValue("version")),
		strings.TrimSpace(r.FormValue("categories")),
		strings.TrimSpace(r.FormValue("author")),
		r.FormValue("content"),
	)
	if err != nil {
		http.Redirect(w, r, "/admin/skill?id="+r.FormValue("skill_id")+"&error=Skill 保存失败", http.StatusSeeOther)
		return
	}

	recordAdminAction(r, "skill.update", "skill", skillID, "更新了 Skill 内容和元数据")
	http.Redirect(w, r, "/admin/skill?id="+r.FormValue("skill_id")+"&info=Skill 已更新", http.StatusSeeOther)
}

func AdminSkillReviewHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}

	skillID, err := parseInt64(r.FormValue("skill_id"))
	if err != nil {
		http.Redirect(w, r, "/admin/skills?error=参数错误", http.StatusSeeOther)
		return
	}

	status := strings.TrimSpace(r.FormValue("review_status"))
	if status != "approved" && status != "rejected" {
		http.Redirect(w, r, "/admin/skill?id="+r.FormValue("skill_id")+"&error=审核状态不正确", http.StatusSeeOther)
		return
	}

	if err := db.ReviewAdminSkill(skillID, adminActorID(r), status, strings.TrimSpace(r.FormValue("review_note"))); err != nil {
		http.Redirect(w, r, "/admin/skill?id="+r.FormValue("skill_id")+"&error=审核操作失败", http.StatusSeeOther)
		return
	}

	action := "skill.approve"
	message := "Skill 已通过审核"
	if status == "rejected" {
		action = "skill.reject"
		message = "Skill 已拒绝"
	}
	recordAdminAction(r, action, "skill", skillID, strings.TrimSpace(r.FormValue("review_note")))

	// 通知上传者审核结果
	notifySkillReviewResult(skillID, status, strings.TrimSpace(r.FormValue("review_note")))
	http.Redirect(w, r, "/admin/skill?id="+r.FormValue("skill_id")+"&info="+urlQueryEscape(message), http.StatusSeeOther)
}

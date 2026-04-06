package handlers

import (
	"log"
	"net/http"

	"skills-hub/db"
	"skills-hub/models"
	"skills-hub/security"
)

func SkillHandler(w http.ResponseWriter, r *http.Request) {
	sess := GetCurrentSession(r)
	if sess == nil || sess.CurrentTenantID == 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	slug := r.URL.Query().Get("slug")
	if slug == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	skill, err := db.GetSkillBySlug(sess.CurrentTenantID, slug)
	if err != nil || skill == nil {
		http.NotFound(w, r)
		return
	}

	// 拉评分统计
	avg, count, _ := db.GetSkillRatingStats(sess.CurrentTenantID, skill.ID)
	skill.AvgRating = avg
	skill.RatingCount = count

	// 当前用户的评分
	userRating, _ := db.GetUserRating(sess.CurrentTenantID, skill.ID, sess.UserID)

	// 拉评论列表
	comments, _ := db.GetSkillComments(sess.CurrentTenantID, skill.ID)
	if comments == nil {
		comments = []models.SkillComment{}
	}
	for i := range comments {
		// 评论展示统一走后端 Markdown 渲染，模板里不再拼原始内容。
		rendered, err := security.RenderCommentMarkdown(comments[i].Content)
		if err != nil {
			log.Printf("comment markdown render failed: %v", err)
			continue
		}
		comments[i].ContentHTML = rendered
	}
	if skill.Content != "" {
		// SKILL.md 详情页也改成后端渲染，顺手把旧数据一起兜底清洗。
		rendered, err := security.RenderSkillMarkdown(skill.Content)
		if err != nil {
			log.Printf("skill markdown render failed: %v", err)
		} else {
			skill.ContentHTML = rendered
		}
	}

	categories, _ := db.GetCategories(sess.CurrentTenantID)

	// 检查是否有来自评论验证码失败的错误
	errParam := r.URL.Query().Get("error")
	errMsg := ""
	if errParam == "captcha" {
		errMsg = "图形验证码错误，请重新输入后提交评论"
	}

	data := PageData{
		Title:       skill.DisplayName + " - Skills Hub",
		Skill:       skill,
		Categories:  categories,
		CurrentPage: "skill",
		Comments:    comments,
		UserRating:  userRating,
		Error:       errMsg,
	}

	RenderTemplate(w, r, "skill.html", data)
}

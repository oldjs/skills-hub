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

	pageInfo := ""

	// 拉评分统计
	avg, count, err := db.GetSkillRatingStats(sess.CurrentTenantID, skill.ID)
	if err != nil {
		log.Printf("skill rating stats failed: %v", err)
		pageInfo = "部分互动数据加载失败，已展示基础信息"
	}
	skill.AvgRating = avg
	skill.RatingCount = count

	// 当前用户的评分
	userRating, err := db.GetUserRating(sess.CurrentTenantID, skill.ID, sess.UserID)
	if err != nil {
		log.Printf("user rating load failed: %v", err)
		pageInfo = "部分互动数据加载失败，已展示基础信息"
	}

	// 拉评论列表
	comments, err := db.GetSkillComments(sess.CurrentTenantID, skill.ID)
	if err != nil {
		log.Printf("skill comments load failed: %v", err)
		comments = []models.SkillComment{}
		pageInfo = "评论暂时加载失败，请稍后刷新重试"
	}
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

	categories, err := db.GetCategories(sess.CurrentTenantID)
	if err != nil {
		log.Printf("skill categories load failed: %v", err)
		categories = []string{}
		if pageInfo == "" {
			pageInfo = "部分页面信息加载失败，已展示核心内容"
		}
	}

	// 拉相关技能推荐
	relatedSkills, err := db.GetRelatedSkills(sess.CurrentTenantID, skill.ID, skill.Categories, 6)
	if err != nil {
		log.Printf("related skills load failed: %v", err)
		relatedSkills = []models.Skill{}
	}

	// 检查是否有来自评论验证码失败的错误
	errParam := r.URL.Query().Get("error")
	errMsg := ""
	if errParam == "captcha" {
		errMsg = "图形验证码错误，请重新输入后提交评论"
	}

	data := PageData{
		Title:         skill.DisplayName + " - Skills Hub",
		Skill:         skill,
		Categories:    categories,
		CurrentPage:   "skill",
		Comments:      comments,
		UserRating:    userRating,
		RelatedSkills: relatedSkills,
		Error:         errMsg,
		Info:          pageInfo,
	}

	RenderTemplate(w, r, "skill.html", data)
}

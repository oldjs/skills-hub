package handlers

import (
	"net/http"

	"skills-hub/db"
	"skills-hub/models"
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

	categories, _ := db.GetCategories(sess.CurrentTenantID)

	data := PageData{
		Title:       skill.DisplayName + " - Skills Hub",
		Skill:       skill,
		Categories:  categories,
		CurrentPage: "skill",
		Comments:    comments,
		UserRating:  userRating,
	}

	RenderTemplate(w, r, "skill.html", data)
}

package handlers

import (
	"log/slog"
	"net/http"

	"skills-hub/db"
	"skills-hub/models"
	"skills-hub/security"
)

func SkillHandler(w http.ResponseWriter, r *http.Request) {
	sess := GetCurrentSession(r)
	tenantID := resolveViewTenantID(sess)
	if tenantID == 0 {
		RenderServerError(w, r)
		return
	}

	slug := r.URL.Query().Get("slug")
	if slug == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	skill, err := db.GetSkillBySlug(tenantID, slug)
	if err != nil || skill == nil {
		RenderNotFound(w, r)
		return
	}

	pageInfo := ""

	// 拉评分统计
	avg, count, err := db.GetSkillRatingStats(tenantID, skill.ID)
	if err != nil {
		slog.Error("skill rating stats failed", "error", err)
		pageInfo = "部分互动数据加载失败，已展示基础信息"
	}
	skill.AvgRating = avg
	skill.RatingCount = count

	// 当前用户的评分（未登录则为 0）
	var userRating int
	if sess != nil {
		userRating, err = db.GetUserRating(tenantID, skill.ID, sess.UserID)
		if err != nil {
			slog.Error("user rating load failed", "error", err)
			pageInfo = "部分互动数据加载失败，已展示基础信息"
		}
	}

	// 拉评论列表（支持排序参数）
	commentSort := r.URL.Query().Get("comment_sort")
	comments, err := db.GetSkillComments(tenantID, skill.ID, commentSort)
	if err != nil {
		slog.Error("skill comments load failed", "error", err)
		comments = []models.SkillComment{}
		pageInfo = "评论暂时加载失败，请稍后刷新重试"
	}
	if comments == nil {
		comments = []models.SkillComment{}
	}
	for i := range comments {
		// 评论展示统一走后端 Markdown 渲染
		rendered, err := security.RenderCommentMarkdown(comments[i].Content)
		if err != nil {
			slog.Error("comment markdown render failed", "error", err)
			continue
		}
		comments[i].ContentHTML = rendered
		// 子评论也要渲染
		for j := range comments[i].Replies {
			rr, err := security.RenderCommentMarkdown(comments[i].Replies[j].Content)
			if err != nil {
				slog.Error("reply markdown render failed", "error", err)
				continue
			}
			comments[i].Replies[j].ContentHTML = rr
		}
	}
	if skill.Content != "" {
		// SKILL.md 详情页也改成后端渲染，顺手把旧数据一起兜底清洗。
		rendered, err := security.RenderSkillMarkdown(skill.Content)
		if err != nil {
			slog.Error("skill markdown render failed", "error", err)
		} else {
			skill.ContentHTML = rendered
		}
	}

	categories, err := db.GetCategories(tenantID)
	if err != nil {
		slog.Error("skill categories load failed", "error", err)
		categories = []string{}
		if pageInfo == "" {
			pageInfo = "部分页面信息加载失败，已展示核心内容"
		}
	}

	// 查收藏状态（未登录为 false）
	var isBookmarked bool
	if sess != nil {
		isBookmarked = db.IsBookmarked(sess.UserID, skill.ID, tenantID)
	}

	// 拉相关技能推荐
	relatedSkills, err := db.GetRelatedSkills(tenantID, skill.ID, skill.Categories, 6)
	if err != nil {
		slog.Error("related skills load failed", "error", err)
		relatedSkills = []models.Skill{}
	}

	// 检查是否有来自评论验证码失败的错误
	errParam := r.URL.Query().Get("error")
	errMsg := ""
	if errParam == "captcha" {
		errMsg = "图形验证码错误，请重新输入后提交评论"
	}

	// SEO: 技能详情页最重要，给足 meta 数据
	metaDesc := truncateText(skill.Summary, 160)
	if metaDesc == "" {
		metaDesc = skill.DisplayName + " - OpenClaw 智能体技能"
	}
	metaKw := skill.DisplayName + "," + skill.Categories + ",OpenClaw,Skills"

	// 技能详情页缓存（未登录用户可用 CDN 缓存）
	if sess == nil {
		cacheContent := skill.Slug + skill.Version + skill.Summary
		if setCacheHeaders(w, r, cacheContent, skill.UpdatedAt) {
			return
		}
	}

	data := PageData{
		Title:           skill.DisplayName + " - Skills Hub",
		MetaDescription: metaDesc,
		MetaKeywords:    metaKw,
		CanonicalURL:    canonicalURL("/skill?slug=" + skill.Slug),
		Skill:           skill,
		Categories:      categories,
		CurrentPage:     "skill",
		Comments:        comments,
		UserRating:      userRating,
		IsBookmarked:    isBookmarked,
		RelatedSkills:   relatedSkills,
		Error:           errMsg,
		Info:            pageInfo,
	}

	RenderTemplate(w, r, "skill.html", data)
}

package models

import "time"

type User struct {
	ID              int64      `json:"id"`
	Email           string     `json:"email"`
	DisplayName     string     `json:"displayName"`
	Bio             string     `json:"bio"`
	Status          string     `json:"status"`
	IsPlatformAdmin bool       `json:"isPlatformAdmin"`
	IsSubAdmin      bool       `json:"isSubAdmin"`
	LastTenantID    *int64     `json:"lastTenantId,omitempty"`
	LastLoginAt     *time.Time `json:"lastLoginAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

// 公开 Profile 数据
type UserProfile struct {
	ID           int64     `json:"id"`
	DisplayName  string    `json:"displayName"`
	Bio          string    `json:"bio"`
	CreatedAt    time.Time `json:"createdAt"`
	SkillCount   int       `json:"skillCount"`
	RatingCount  int       `json:"ratingCount"`
	CommentCount int       `json:"commentCount"`
}

// Profile 页的评分记录
type UserRatingItem struct {
	SkillSlug string    `json:"skillSlug"`
	SkillName string    `json:"skillName"`
	Score     int       `json:"score"`
	CreatedAt time.Time `json:"createdAt"`
}

// Profile 页的评论记录
type UserCommentItem struct {
	SkillSlug string    `json:"skillSlug"`
	SkillName string    `json:"skillName"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

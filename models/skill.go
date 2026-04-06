package models

import "time"

type Skill struct {
	ID          int64     `json:"id"`
	TenantID    int64     `json:"tenantId"`
	Slug        string    `json:"slug"`
	DisplayName string    `json:"displayName"`
	Summary     string    `json:"summary"`
	Content     string    `json:"content"`     // SKILL.md 的完整内容
	Score       float64   `json:"score"`       // 原始评分（来源数据）
	AvgRating   float64   `json:"avgRating"`   // 用户评分均值
	RatingCount int       `json:"ratingCount"` // 评分人数
	UpdatedAt   time.Time `json:"updatedAt"`
	Version     string    `json:"version"`
	Categories  string    `json:"categories"`
	Source      string    `json:"source"`
	ClawHubURL  string    `json:"clawhubUrl"`
}

// 用户评分
type SkillRating struct {
	ID        int64     `json:"id"`
	TenantID  int64     `json:"tenantId"`
	SkillID   int64     `json:"skillId"`
	UserID    int64     `json:"userId"`
	Score     int       `json:"score"`
	CreatedAt time.Time `json:"createdAt"`
}

// 用户评论
type SkillComment struct {
	ID          int64     `json:"id"`
	TenantID    int64     `json:"tenantId"`
	SkillID     int64     `json:"skillId"`
	UserID      int64     `json:"userId"`
	Content     string    `json:"content"`
	CreatedAt   time.Time `json:"createdAt"`
	Email       string    `json:"email"`       // join 查出来的
	DisplayName string    `json:"displayName"` // join 查出来的
}

type SearchResult struct {
	Results []Skill `json:"results"`
}

type APIResponse struct {
	Results []struct {
		Slug        string  `json:"slug"`
		DisplayName string  `json:"displayName"`
		Summary     string  `json:"summary"`
		Score       float64 `json:"score"`
		UpdatedAt   int64   `json:"updatedAt"`
		Version     string  `json:"version"`
	} `json:"results"`
}

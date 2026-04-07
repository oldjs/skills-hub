package models

import "time"

type AdminDashboardStats struct {
	TotalSkills   int
	TotalUsers    int
	TotalComments int
	PendingSkills int
}

type AdminActionLog struct {
	ID          int64
	AdminUserID int64
	AdminName   string
	AdminEmail  string
	Action      string
	TargetType  string
	TargetID    int64
	Details     string
	CreatedAt   time.Time
}

type AdminSkill struct {
	ID           int64
	TenantID     int64
	TenantName   string
	Slug         string
	DisplayName  string
	Summary      string
	Content      string
	Version      string
	Categories   string
	Author       string
	Source       string
	ReviewStatus string
	ReviewNote   string
	ReviewerName string
	ReviewedAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
	AvgRating    float64
	RatingCount  int
}

type AdminComment struct {
	ID              int64
	TenantID        int64
	TenantName      string
	SkillID         int64
	SkillSlug       string
	SkillName       string
	UserID          int64
	UserEmail       string
	UserDisplayName string
	Content         string
	CreatedAt       time.Time
}

type AdminUser struct {
	ID              int64
	Email           string
	DisplayName     string
	Status          string
	IsPlatformAdmin bool
	TenantCount     int
	CreatedAt       time.Time
	LastLoginAt     *time.Time
}

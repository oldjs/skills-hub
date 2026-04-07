package models

import "time"

type User struct {
	ID              int64      `json:"id"`
	Email           string     `json:"email"`
	DisplayName     string     `json:"displayName"`
	Status          string     `json:"status"`
	IsPlatformAdmin bool       `json:"isPlatformAdmin"`
	IsSubAdmin      bool       `json:"isSubAdmin"`
	LastTenantID    *int64     `json:"lastTenantId,omitempty"`
	LastLoginAt     *time.Time `json:"lastLoginAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

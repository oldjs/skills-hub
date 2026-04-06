package models

import "time"

type Tenant struct {
	ID              int64     `json:"id"`
	Slug            string    `json:"slug"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	Status          string    `json:"status"`
	AutoSyncEnabled bool      `json:"autoSyncEnabled"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type TenantMember struct {
	ID          int64     `json:"id"`
	TenantID    int64     `json:"tenantId"`
	UserID      int64     `json:"userId"`
	Role        string    `json:"role"`
	Status      string    `json:"status"`
	JoinedAt    time.Time `json:"joinedAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
}

type TenantInvite struct {
	ID         int64      `json:"id"`
	TenantID   int64      `json:"tenantId"`
	Email      string     `json:"email"`
	Role       string     `json:"role"`
	Status     string     `json:"status"`
	ExpiresAt  time.Time  `json:"expiresAt"`
	AcceptedAt *time.Time `json:"acceptedAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

type UserTenant struct {
	TenantID         int64  `json:"tenantId"`
	TenantSlug       string `json:"tenantSlug"`
	TenantName       string `json:"tenantName"`
	TenantStatus     string `json:"tenantStatus"`
	TenantRole       string `json:"tenantRole"`
	MembershipStatus string `json:"membershipStatus"`
}

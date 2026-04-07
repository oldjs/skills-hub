package models

import "time"

type APIKey struct {
	ID         int64
	UserID     int64
	KeyPrefix  string
	Name       string
	CreatedAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}

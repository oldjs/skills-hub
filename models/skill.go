package models

import "time"

type Skill struct {
	ID          int64     `json:"id"`
	Slug        string    `json:"slug"`
	DisplayName string    `json:"displayName"`
	Summary     string    `json:"summary"`
	Score       float64   `json:"score"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Version     string    `json:"version"`
	Categories  string    `json:"categories"`
	ClawHubURL  string    `json:"clawhubUrl"`
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

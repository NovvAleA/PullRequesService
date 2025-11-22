package models

import "time"

type User struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	TeamName string `json:"team_name"` // Добавлено из спецификации
	IsActive bool   `json:"is_active"`
}

type Team struct {
	TeamName string `json:"team_name"`
	Members  []User `json:"members"`
}

type TeamMember struct { // Добавлено из спецификации
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
}

type SetActiveRequest struct {
	UserID string `json:"user_id"`
	Active bool   `json:"is_active"`
}

type PullRequest struct {
	PullRequestID   string    `json:"pull_request_id"`
	PullRequestName string    `json:"pull_request_name"`
	AuthorID        string    `json:"author_id"`
	Status          string    `json:"status"` // OPEN|MERGED
	Reviewers       []string  `json:"assigned_reviewers"`
	CreatedAt       time.Time `json:"createdAt,omitempty"` // Добавлено из спецификации
	MergedAt        *string   `json:"mergedAt,omitempty"`  // Может быть null
}

type PullRequestShort struct { // Добавлено из спецификации
	PullRequestID   string `json:"pull_request_id"`
	PullRequestName string `json:"pull_request_name"`
	AuthorID        string `json:"author_id"`
	Status          string `json:"status"` // OPEN|MERGED
}

type CreatePRRequest struct {
	PullRequestID   string `json:"pull_request_id"`
	PullRequestName string `json:"pull_request_name"`
	AuthorID        string `json:"author_id"`
}

type ReassignRequest struct {
	PullRequestID string `json:"pull_request_id"`
	OldUserID     string `json:"old_user_id"`
}

type ErrorResponse struct { // Добавлено из спецификации
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

package models

type User struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
}

type Team struct {
	TeamName string `json:"team_name"`
	Members  []User `json:"members"`
}

type SetActiveRequest struct {
	UserID string `json:"user_id"`
	Active bool   `json:"is_active"`
}

type PullRequest struct {
	PullRequestID   string   `json:"pull_request_id"`
	PullRequestName string   `json:"pull_request_name"`
	AuthorID        string   `json:"author_id"`
	Status          string   `json:"status"` // OPEN|MERGED
	Reviewers       []string `json:"assigned_reviewers"`
	MergedAt        *string  `json:"mergedAt,omitempty"` // Может быть null
}

type CreatePRRequest struct {
	PullRequestID   string `json:"pull_request_id"`
	PullRequestName string `json:"pull_request_name"`
	AuthorID        string `json:"author_id"`
}

type ReassignRequest struct {
	OldReviewerID string `json:"old_reviewer_id"`
}

package api

import (
	"testing"
	"time"

	"PR_service/internal/models"

	"github.com/stretchr/testify/assert"
)

// Тестируем функции из пакета api

func TestValidateRequiredFields(t *testing.T) {
	tests := []struct {
		name     string
		fields   map[string]string
		expected string
	}{
		{
			name: "All fields present",
			fields: map[string]string{
				"field1": "value1",
				"field2": "value2",
			},
			expected: "",
		},
		{
			name: "One field missing",
			fields: map[string]string{
				"field1": "value1",
				"field2": "",
			},
			expected: "field2 is required",
		},
		{
			name: "Multiple fields missing - returns first",
			fields: map[string]string{
				"field1": "",
				"field2": "",
				"field3": "value3",
			},
			expected: "field1 is required",
		},
		{
			name:     "Empty fields map",
			fields:   map[string]string{},
			expected: "",
		},
		{
			name: "All fields empty",
			fields: map[string]string{
				"field1": "",
				"field2": "",
			},
			expected: "field1 is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateRequiredFields(tt.fields)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreatePRRequestValidation(t *testing.T) {
	tests := []struct {
		name        string
		pr          models.CreatePRRequest
		shouldError bool
		errorField  string
	}{
		{
			name: "Valid request",
			pr: models.CreatePRRequest{
				PullRequestID:   "pr1",
				PullRequestName: "Test PR",
				AuthorID:        "user1",
			},
			shouldError: false,
		},
		{
			name: "Missing pull_request_id",
			pr: models.CreatePRRequest{
				PullRequestID:   "",
				PullRequestName: "Test PR",
				AuthorID:        "user1",
			},
			shouldError: true,
			errorField:  "pull_request_id",
		},
		{
			name: "Missing pull_request_name",
			pr: models.CreatePRRequest{
				PullRequestID:   "pr1",
				PullRequestName: "",
				AuthorID:        "user1",
			},
			shouldError: true,
			errorField:  "pull_request_name",
		},
		{
			name: "Missing author_id",
			pr: models.CreatePRRequest{
				PullRequestID:   "pr1",
				PullRequestName: "Test PR",
				AuthorID:        "",
			},
			shouldError: true,
			errorField:  "author_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := map[string]string{
				"pull_request_id":   tt.pr.PullRequestID,
				"pull_request_name": tt.pr.PullRequestName,
				"author_id":         tt.pr.AuthorID,
			}

			result := validateRequiredFields(fields)

			if tt.shouldError {
				assert.Contains(t, result, tt.errorField)
				assert.Contains(t, result, "is required")
			} else {
				assert.Empty(t, result)
			}
		})
	}
}

func TestTeamValidation(t *testing.T) {
	tests := []struct {
		name        string
		team        models.Team
		shouldError bool
	}{
		{
			name: "Valid team with members",
			team: models.Team{
				TeamName: "backend",
				Members: []models.User{
					{UserID: "user1", Username: "john", TeamName: "backend", IsActive: true},
					{UserID: "user2", Username: "jane", TeamName: "backend", IsActive: true},
				},
			},
			shouldError: false,
		},
		{
			name: "Valid team without members",
			team: models.Team{
				TeamName: "empty-team",
				Members:  []models.User{},
			},
			shouldError: false,
		},
		{
			name: "Invalid team - empty team name",
			team: models.Team{
				TeamName: "",
				Members: []models.User{
					{UserID: "user1", Username: "john", TeamName: "", IsActive: true},
				},
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := map[string]string{
				"team_name": tt.team.TeamName,
			}

			result := validateRequiredFields(fields)

			if tt.shouldError {
				assert.Contains(t, result, "team_name")
				assert.Contains(t, result, "is required")
			} else {
				assert.Empty(t, result)
			}
		})
	}
}

func TestUserValidation(t *testing.T) {
	tests := []struct {
		name        string
		user        models.User
		shouldError bool
	}{
		{
			name: "Valid user with team name",
			user: models.User{
				UserID:   "user1",
				Username: "john_doe",
				TeamName: "backend",
				IsActive: true,
			},
			shouldError: false,
		},
		{
			name: "Valid user without team name",
			user: models.User{
				UserID:   "user2",
				Username: "jane_doe",
				TeamName: "",
				IsActive: true,
			},
			shouldError: false, // team_name не обязателен на уровне валидации полей
		},
		{
			name: "Invalid user - empty user_id",
			user: models.User{
				UserID:   "",
				Username: "john_doe",
				TeamName: "backend",
				IsActive: true,
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Тестируем только обязательные поля для SetIsActive
			fields := map[string]string{
				"user_id": tt.user.UserID,
			}

			result := validateRequiredFields(fields)

			if tt.shouldError {
				assert.Contains(t, result, "user_id")
				assert.Contains(t, result, "is required")
			} else {
				assert.Empty(t, result)
			}
		})
	}
}

func TestPullRequestValidation(t *testing.T) {
	tests := []struct {
		name        string
		pr          models.PullRequest
		shouldError bool
	}{
		{
			name: "Valid OPEN pull request",
			pr: models.PullRequest{
				PullRequestID:   "pr1",
				PullRequestName: "Feature implementation",
				AuthorID:        "user1",
				Status:          "OPEN",
				Reviewers:       []string{"user2", "user3"},
			},
			shouldError: false,
		},
		{
			name: "Valid MERGED pull request",
			pr: models.PullRequest{
				PullRequestID:   "pr2",
				PullRequestName: "Bug fix",
				AuthorID:        "user1",
				Status:          "MERGED",
				Reviewers:       []string{"user2"},
			},
			shouldError: false,
		},
		{
			name: "Invalid pull request - empty ID",
			pr: models.PullRequest{
				PullRequestID:   "",
				PullRequestName: "Feature implementation",
				AuthorID:        "user1",
				Status:          "OPEN",
				Reviewers:       []string{"user2"},
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Тестируем только обязательные поля для CreatePR
			fields := map[string]string{
				"pull_request_id":   tt.pr.PullRequestID,
				"pull_request_name": tt.pr.PullRequestName,
				"author_id":         tt.pr.AuthorID,
			}

			result := validateRequiredFields(fields)

			if tt.shouldError {
				assert.Contains(t, result, "pull_request_id")
				assert.Contains(t, result, "is required")
			} else {
				assert.Empty(t, result)
			}
		})
	}
}

func TestErrorResponseCreation(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		message      string
		expectedCode string
		expectedMsg  string
	}{
		{
			name:         "NOT_FOUND error",
			code:         "NOT_FOUND",
			message:      "Resource not found",
			expectedCode: "NOT_FOUND",
			expectedMsg:  "Resource not found",
		},
		{
			name:         "PR_EXISTS error",
			code:         "PR_EXISTS",
			message:      "PR already exists",
			expectedCode: "PR_EXISTS",
			expectedMsg:  "PR already exists",
		},
		{
			name:         "Empty error",
			code:         "",
			message:      "",
			expectedCode: "",
			expectedMsg:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorResp := createErrorResponse(tt.code, tt.message)

			assert.Equal(t, tt.expectedCode, errorResp.Error.Code)
			assert.Equal(t, tt.expectedMsg, errorResp.Error.Message)
		})
	}
}

func TestResponseCreationHelpers(t *testing.T) {
	t.Run("createTeamResponse", func(t *testing.T) {
		team := models.Team{
			TeamName: "test-team",
			Members: []models.User{
				{UserID: "user1", Username: "User One", TeamName: "test-team", IsActive: true},
			},
		}

		response := createTeamResponse(team)

		assert.Contains(t, response, "team")
		assert.Equal(t, team, response["team"])
	})

	t.Run("createUserResponse", func(t *testing.T) {
		user := models.User{
			UserID:   "user1",
			Username: "John Doe",
			TeamName: "backend",
			IsActive: true,
		}

		response := createUserResponse(user)

		assert.Contains(t, response, "user")
		assert.Equal(t, user, response["user"])
	})

	t.Run("createPRResponse", func(t *testing.T) {
		pr := models.PullRequest{
			PullRequestID:   "pr1",
			PullRequestName: "Test PR",
			AuthorID:        "user1",
			Status:          "OPEN",
			Reviewers:       []string{"user2", "user3"},
		}

		response := createPRResponse(pr)

		assert.Contains(t, response, "pr")
		assert.Equal(t, pr, response["pr"])
	})

	t.Run("createPRShortResponse", func(t *testing.T) {
		prs := []models.PullRequestShort{
			{
				PullRequestID:   "pr1",
				PullRequestName: "PR One",
				AuthorID:        "user1",
				Status:          "OPEN",
			},
			{
				PullRequestID:   "pr2",
				PullRequestName: "PR Two",
				AuthorID:        "user2",
				Status:          "MERGED",
			},
		}
		userID := "reviewer1"

		response := createPRShortResponse(prs, userID)

		assert.Contains(t, response, "user_id")
		assert.Contains(t, response, "pull_requests")
		assert.Equal(t, userID, response["user_id"])
		assert.Equal(t, prs, response["pull_requests"])
	})

	t.Run("createReassignResponse", func(t *testing.T) {
		pr := models.PullRequest{
			PullRequestID:   "pr1",
			PullRequestName: "Test PR",
			AuthorID:        "user1",
			Status:          "OPEN",
			Reviewers:       []string{"user3"},
		}
		replacedBy := "user3"

		response := createReassignResponse(pr, replacedBy)

		assert.Contains(t, response, "pr")
		assert.Contains(t, response, "replaced_by")
		assert.Equal(t, pr, response["pr"])
		assert.Equal(t, replacedBy, response["replaced_by"])
	})
}

func TestModelInitialization(t *testing.T) {
	t.Run("User model with all fields", func(t *testing.T) {
		user := models.User{
			UserID:   "test-user",
			Username: "Test User",
			TeamName: "test-team",
			IsActive: true,
		}

		assert.Equal(t, "test-user", user.UserID)
		assert.Equal(t, "Test User", user.Username)
		assert.Equal(t, "test-team", user.TeamName)
		assert.True(t, user.IsActive)
	})

	t.Run("PullRequest model with dates", func(t *testing.T) {
		now := time.Now()
		mergedAt := "2023-01-01T12:00:00Z"

		pr := models.PullRequest{
			PullRequestID:   "test-pr",
			PullRequestName: "Test PR",
			AuthorID:        "user1",
			Status:          "MERGED",
			Reviewers:       []string{"user2"},
			CreatedAt:       now,
			MergedAt:        &mergedAt,
		}

		assert.Equal(t, "test-pr", pr.PullRequestID)
		assert.Equal(t, "Test PR", pr.PullRequestName)
		assert.Equal(t, "user1", pr.AuthorID)
		assert.Equal(t, "MERGED", pr.Status)
		assert.Len(t, pr.Reviewers, 1)
		assert.Equal(t, now, pr.CreatedAt)
		assert.Equal(t, &mergedAt, pr.MergedAt)
	})

	t.Run("PullRequestShort model", func(t *testing.T) {
		prShort := models.PullRequestShort{
			PullRequestID:   "short-pr",
			PullRequestName: "Short PR",
			AuthorID:        "user1",
			Status:          "OPEN",
		}

		assert.Equal(t, "short-pr", prShort.PullRequestID)
		assert.Equal(t, "Short PR", prShort.PullRequestName)
		assert.Equal(t, "user1", prShort.AuthorID)
		assert.Equal(t, "OPEN", prShort.Status)
	})

	t.Run("ErrorResponse model", func(t *testing.T) {
		errorResp := models.ErrorResponse{}
		errorResp.Error.Code = "TEST_ERROR"
		errorResp.Error.Message = "Test error message"

		assert.Equal(t, "TEST_ERROR", errorResp.Error.Code)
		assert.Equal(t, "Test error message", errorResp.Error.Message)
	})
}

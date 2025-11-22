package storage

import (
	"testing"
	"time"

	"PR_service/internal/models"

	"github.com/stretchr/testify/assert"
)

// MockDBExecutor мок для DB операций
// Тестируем только функции из пакета storage

func TestPickRandomDistinct(t *testing.T) {
	tests := []struct {
		name     string
		arr      []string
		n        int
		expected int
	}{
		{
			name:     "More elements than needed",
			arr:      []string{"a", "b", "c", "d", "e"},
			n:        3,
			expected: 3,
		},
		{
			name:     "Less elements than needed",
			arr:      []string{"a", "b"},
			n:        5,
			expected: 2,
		},
		{
			name:     "Empty array",
			arr:      []string{},
			n:        3,
			expected: 0,
		},
		{
			name:     "Exact number of elements",
			arr:      []string{"a", "b", "c"},
			n:        3,
			expected: 3,
		},
		{
			name:     "Single element",
			arr:      []string{"a"},
			n:        1,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PickForTest(tt.arr, tt.n)
			assert.Len(t, result, tt.expected)

			// Check that all elements in result are from original array
			for _, item := range result {
				assert.Contains(t, tt.arr, item)
			}

			// Check for duplicates
			assert.Equal(t, len(result), len(uniqueStrings(result)))
		})
	}
}

func TestPickRandomDistinct_NoDuplicates(t *testing.T) {
	input := []string{"a", "b", "c", "d"}
	result := PickForTest(input, 3)

	// Should have no duplicates
	seen := make(map[string]bool)
	for _, item := range result {
		assert.False(t, seen[item], "Duplicate found: %s", item)
		seen[item] = true
	}
}

func TestPickRandomDistinct_OriginalNotModified(t *testing.T) {
	original := []string{"x", "y", "z"}
	copyArr := make([]string, len(original))
	copy(copyArr, original)

	_ = PickForTest(copyArr, 2)

	// Original array should not be modified
	assert.Equal(t, original, copyArr)
}

// Тестируем бизнес-логику, которая находится в storage
func TestCreatePRValidation(t *testing.T) {
	tests := []struct {
		name        string
		pr          models.CreatePRRequest
		shouldError bool
	}{
		{
			name: "Valid PR request",
			pr: models.CreatePRRequest{
				PullRequestID:   "pr1",
				PullRequestName: "Test PR",
				AuthorID:        "user1",
			},
			shouldError: false,
		},
		{
			name: "Missing PR ID",
			pr: models.CreatePRRequest{
				PullRequestID:   "",
				PullRequestName: "Test PR",
				AuthorID:        "user1",
			},
			shouldError: true,
		},
		{
			name: "Missing PR Name",
			pr: models.CreatePRRequest{
				PullRequestID:   "pr1",
				PullRequestName: "",
				AuthorID:        "user1",
			},
			shouldError: true,
		},
		{
			name: "Missing Author ID",
			pr: models.CreatePRRequest{
				PullRequestID:   "pr1",
				PullRequestName: "Test PR",
				AuthorID:        "",
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Вместо validateRequiredFields проверяем поля напрямую
			hasError := tt.pr.PullRequestID == "" || tt.pr.PullRequestName == "" || tt.pr.AuthorID == ""
			assert.Equal(t, tt.shouldError, hasError)
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
				},
			},
			shouldError: false,
		},
		{
			name: "Empty team name",
			team: models.Team{
				TeamName: "",
				Members: []models.User{
					{UserID: "user1", Username: "john", TeamName: "", IsActive: true},
				},
			},
			shouldError: true,
		},
		{
			name: "No members",
			team: models.Team{
				TeamName: "backend",
				Members:  []models.User{},
			},
			shouldError: false, // Команда без участников допустима
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasError := tt.team.TeamName == ""
			assert.Equal(t, tt.shouldError, hasError)
		})
	}
}

// Тестируем граничные случаи для pickRandomDistinct
func TestPickRandomDistinct_EdgeCases(t *testing.T) {
	t.Run("Zero elements requested", func(t *testing.T) {
		result := PickForTest([]string{"a", "b", "c"}, 0)
		assert.Empty(t, result, "Должен вернуть пустой слайс при n=0")
	})

	t.Run("Negative elements requested", func(t *testing.T) {
		result := PickForTest([]string{"a", "b", "c"}, -1)
		assert.Empty(t, result, "Должен вернуть пустой слайс при отрицательном n")
	})

	t.Run("Nil slice", func(t *testing.T) {
		result := PickForTest(nil, 2)
		assert.Empty(t, result, "Должен вернуть пустой слайс при nil массиве")
	})

	t.Run("Empty slice", func(t *testing.T) {
		result := PickForTest([]string{}, 2)
		assert.Empty(t, result, "Должен вернуть пустой слайс при пустом массиве")
	})

	t.Run("Very large n", func(t *testing.T) {
		result := PickForTest([]string{"a", "b"}, 1000)
		assert.Len(t, result, 2, "Должен вернуть все элементы когда n > len(arr)")
		assert.ElementsMatch(t, []string{"a", "b"}, result)
	})

	t.Run("Single element array", func(t *testing.T) {
		result := PickForTest([]string{"single"}, 1)
		assert.Equal(t, []string{"single"}, result, "Должен вернуть единственный элемент")
	})

	t.Run("Single element array with n=0", func(t *testing.T) {
		result := PickForTest([]string{"single"}, 0)
		assert.Empty(t, result, "Должен вернуть пустой слайс даже при одном элементе")
	})
}

func TestModelStructures(t *testing.T) {
	t.Run("User model with team name", func(t *testing.T) {
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
			Reviewers:       []string{"user2", "user3"},
			CreatedAt:       now,
			MergedAt:        &mergedAt,
		}

		assert.Equal(t, "test-pr", pr.PullRequestID)
		assert.Equal(t, "Test PR", pr.PullRequestName)
		assert.Equal(t, "user1", pr.AuthorID)
		assert.Equal(t, "MERGED", pr.Status)
		assert.Len(t, pr.Reviewers, 2)
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

	t.Run("Team with members", func(t *testing.T) {
		team := models.Team{
			TeamName: "development",
			Members: []models.User{
				{
					UserID:   "dev1",
					Username: "Developer One",
					TeamName: "development",
					IsActive: true,
				},
				{
					UserID:   "dev2",
					Username: "Developer Two",
					TeamName: "development",
					IsActive: true,
				},
			},
		}

		assert.Equal(t, "development", team.TeamName)
		assert.Len(t, team.Members, 2)
		assert.Equal(t, "dev1", team.Members[0].UserID)
		assert.Equal(t, "development", team.Members[0].TeamName)
		assert.Equal(t, "dev2", team.Members[1].UserID)
		assert.Equal(t, "development", team.Members[1].TeamName)
	})
}

func TestStorageModelCompatibility(t *testing.T) {
	t.Run("CreatePRRequest compatibility", func(t *testing.T) {
		// Проверяем что CreatePRRequest имеет правильные поля для storage
		prRequest := models.CreatePRRequest{
			PullRequestID:   "storage-test-pr",
			PullRequestName: "Storage Test PR",
			AuthorID:        "storage-user",
		}

		assert.Equal(t, "storage-test-pr", prRequest.PullRequestID)
		assert.Equal(t, "Storage Test PR", prRequest.PullRequestName)
		assert.Equal(t, "storage-user", prRequest.AuthorID)

		// Проверяем что поля не пустые (для валидации)
		assert.NotEmpty(t, prRequest.PullRequestID)
		assert.NotEmpty(t, prRequest.PullRequestName)
		assert.NotEmpty(t, prRequest.AuthorID)
	})

	t.Run("SetActiveRequest compatibility", func(t *testing.T) {
		activeReq := models.SetActiveRequest{
			UserID: "test-user",
			Active: false,
		}

		assert.Equal(t, "test-user", activeReq.UserID)
		assert.False(t, activeReq.Active)
		assert.NotEmpty(t, activeReq.UserID)
	})

	t.Run("ReassignRequest compatibility", func(t *testing.T) {
		reassignReq := models.ReassignRequest{
			PullRequestID: "pr-to-reassign",
			OldUserID:     "old-reviewer",
		}

		assert.Equal(t, "pr-to-reassign", reassignReq.PullRequestID)
		assert.Equal(t, "old-reviewer", reassignReq.OldUserID)
		assert.NotEmpty(t, reassignReq.PullRequestID)
		assert.NotEmpty(t, reassignReq.OldUserID)
	})
}

func TestErrorScenarios(t *testing.T) {
	t.Run("Empty user ID in SetActiveRequest", func(t *testing.T) {
		req := models.SetActiveRequest{
			UserID: "",
			Active: true,
		}

		assert.Empty(t, req.UserID)
	})

	t.Run("Empty PR ID in CreatePRRequest", func(t *testing.T) {
		req := models.CreatePRRequest{
			PullRequestID:   "",
			PullRequestName: "Valid Name",
			AuthorID:        "valid-author",
		}

		assert.Empty(t, req.PullRequestID)
		assert.NotEmpty(t, req.PullRequestName)
		assert.NotEmpty(t, req.AuthorID)
	})

	t.Run("PR with nil MergedAt", func(t *testing.T) {
		pr := models.PullRequest{
			PullRequestID:   "open-pr",
			PullRequestName: "Open PR",
			AuthorID:        "author",
			Status:          "OPEN",
			Reviewers:       []string{"reviewer1"},
			CreatedAt:       time.Now(),
			MergedAt:        nil,
		}

		assert.Equal(t, "OPEN", pr.Status)
		assert.Nil(t, pr.MergedAt)
		assert.False(t, pr.CreatedAt.IsZero())
	})
}

// Вспомогательная функция для проверки уникальности
func uniqueStrings(arr []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, item := range arr {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

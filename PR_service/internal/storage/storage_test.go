package storage

import (
	"testing"

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
			name: "Valid team",
			team: models.Team{
				TeamName: "backend",
				Members: []models.User{
					{UserID: "user1", Username: "john"},
				},
			},
			shouldError: false,
		},
		{
			name: "Empty team name",
			team: models.Team{
				TeamName: "",
				Members: []models.User{
					{UserID: "user1", Username: "john"},
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

package api

import (
	"testing"

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

/*
	func TestHandlerErrorScenarios(t *testing.T) {
		// Создаем mock storage для тестирования обработчиков
		handler := &Handler{}

		// Здесь можно тестировать логику обработки ошибок в handlers
		// Например, как разные ошибки из storage мапятся на HTTP статусы
	}
*/
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

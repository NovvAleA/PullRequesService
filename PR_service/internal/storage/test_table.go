package storage

import (
	"PR_service/internal/models"
	"time"
)

// TestCase представляет один тестовый случай
type TestCase struct {
	name           string
	testType       string
	input          interface{}
	expectedResult interface{}
	wantError      bool
}

// PickRandomInput входные данные для тестирования pickRandomDistinct
type PickRandomInput struct {
	arr []string
	n   int
}

// TeamInput входные данные для тестирования операций с командами
type TeamInput struct {
	team models.Team
}

// UserInput входные данные для тестирования операций с пользователями
type UserInput struct {
	user models.User
}

// PRInput входные данные для тестирования операций с PR
type PRInput struct {
	pr models.PullRequest
}

// CreatePRInput входные данные для тестирования создания PR
type CreatePRInput struct {
	pr models.CreatePRRequest
}

// SetActiveInput входные данные для тестирования изменения активности пользователя
type SetActiveInput struct {
	req models.SetActiveRequest
}

// ReassignInput входные данные для тестирования переназначения ревьюера
type ReassignInput struct {
	req models.ReassignRequest
}

// testTable возвращает тестовые случаи для логических функций storage
func testTable() []TestCase {
	// Создаем тестовые данные с обновленными моделями
	now := time.Now()
	mergedAt := "2023-01-01T12:00:00Z"

	return []TestCase{
		{
			name:     "Pick random from array",
			testType: "PickRandomDistinct",
			input: PickRandomInput{
				arr: []string{"a", "b", "c", "d"},
				n:   2,
			},
			wantError: false,
		},
		{
			name:     "Pick random from empty array",
			testType: "PickRandomDistinct",
			input: PickRandomInput{
				arr: []string{},
				n:   2,
			},
			wantError: false,
		},
		{
			name:     "Valid team creation",
			testType: "TeamOperations",
			input: TeamInput{
				team: models.Team{
					TeamName: "backend-team",
					Members: []models.User{
						{
							UserID:   "user1",
							Username: "Developer One",
							TeamName: "backend-team",
							IsActive: true,
						},
						{
							UserID:   "user2",
							Username: "Developer Two",
							TeamName: "backend-team",
							IsActive: true,
						},
					},
				},
			},
			wantError: false,
		},
		{
			name:     "Team with inactive users",
			testType: "TeamOperations",
			input: TeamInput{
				team: models.Team{
					TeamName: "mixed-team",
					Members: []models.User{
						{
							UserID:   "active-user",
							Username: "Active User",
							TeamName: "mixed-team",
							IsActive: true,
						},
						{
							UserID:   "inactive-user",
							Username: "Inactive User",
							TeamName: "mixed-team",
							IsActive: false,
						},
					},
				},
			},
			wantError: false,
		},
		{
			name:     "Valid user with team",
			testType: "UserOperations",
			input: UserInput{
				user: models.User{
					UserID:   "test-user",
					Username: "Test User",
					TeamName: "test-team",
					IsActive: true,
				},
			},
			wantError: false,
		},
		{
			name:     "User without team",
			testType: "UserOperations",
			input: UserInput{
				user: models.User{
					UserID:   "no-team-user",
					Username: "No Team User",
					TeamName: "",
					IsActive: true,
				},
			},
			wantError: false,
		},
		{
			name:     "Valid PR creation request",
			testType: "PROperations",
			input: CreatePRInput{
				pr: models.CreatePRRequest{
					PullRequestID:   "test-pr",
					PullRequestName: "Test Pull Request",
					AuthorID:        "author1",
				},
			},
			wantError: false,
		},
		{
			name:     "PR creation with empty fields",
			testType: "PROperations",
			input: CreatePRInput{
				pr: models.CreatePRRequest{
					PullRequestID:   "",
					PullRequestName: "Test PR",
					AuthorID:        "author1",
				},
			},
			wantError: true,
		},
		{
			name:     "Open PR with reviewers",
			testType: "PROperations",
			input: PRInput{
				pr: models.PullRequest{
					PullRequestID:   "open-pr",
					PullRequestName: "Open Pull Request",
					AuthorID:        "author1",
					Status:          "OPEN",
					Reviewers:       []string{"reviewer1", "reviewer2"},
					CreatedAt:       now,
					MergedAt:        nil,
				},
			},
			wantError: false,
		},
		{
			name:     "Merged PR with date",
			testType: "PROperations",
			input: PRInput{
				pr: models.PullRequest{
					PullRequestID:   "merged-pr",
					PullRequestName: "Merged Pull Request",
					AuthorID:        "author1",
					Status:          "MERGED",
					Reviewers:       []string{"reviewer1"},
					CreatedAt:       now,
					MergedAt:        &mergedAt,
				},
			},
			wantError: false,
		},
		{
			name:     "PR without reviewers",
			testType: "PROperations",
			input: PRInput{
				pr: models.PullRequest{
					PullRequestID:   "no-reviewers-pr",
					PullRequestName: "PR Without Reviewers",
					AuthorID:        "author1",
					Status:          "OPEN",
					Reviewers:       []string{},
					CreatedAt:       now,
					MergedAt:        nil,
				},
			},
			wantError: false,
		},
		{
			name:     "Set user active",
			testType: "UserOperations",
			input: SetActiveInput{
				req: models.SetActiveRequest{
					UserID: "user1",
					Active: true,
				},
			},
			wantError: false,
		},
		{
			name:     "Set user inactive",
			testType: "UserOperations",
			input: SetActiveInput{
				req: models.SetActiveRequest{
					UserID: "user1",
					Active: false,
				},
			},
			wantError: false,
		},
		{
			name:     "Valid reassign request",
			testType: "ReassignOperations",
			input: ReassignInput{
				req: models.ReassignRequest{
					PullRequestID: "pr1",
					OldUserID:     "old-reviewer",
				},
			},
			wantError: false,
		},
		{
			name:     "Reassign with empty PR ID",
			testType: "ReassignOperations",
			input: ReassignInput{
				req: models.ReassignRequest{
					PullRequestID: "",
					OldUserID:     "old-reviewer",
				},
			},
			wantError: true,
		},
		{
			name:     "PullRequestShort for listing",
			testType: "PROperations",
			input: PRInput{
				pr: models.PullRequest{
					PullRequestID:   "short-pr",
					PullRequestName: "Short PR Display",
					AuthorID:        "author1",
					Status:          "OPEN",
					// Для PullRequestShort остальные поля не используются
				},
			},
			wantError: false,
		},
	}
}

// GetTestCases возвращает тестовые случаи для использования в тестах
func GetTestCases() []TestCase {
	return testTable()
}

// GetTestCasesByType возвращает тестовые случаи определенного типа
func GetTestCasesByType(testType string) []TestCase {
	allCases := testTable()
	var filteredCases []TestCase

	for _, testCase := range allCases {
		if testCase.testType == testType {
			filteredCases = append(filteredCases, testCase)
		}
	}

	return filteredCases
}

// GetErrorTestCases возвращает тестовые случаи которые должны вернуть ошибку
func GetErrorTestCases() []TestCase {
	allCases := testTable()
	var errorCases []TestCase

	for _, testCase := range allCases {
		if testCase.wantError {
			errorCases = append(errorCases, testCase)
		}
	}

	return errorCases
}

// GetSuccessTestCases возвращает тестовые случаи которые не должны вернуть ошибку
func GetSuccessTestCases() []TestCase {
	allCases := testTable()
	var successCases []TestCase

	for _, testCase := range allCases {
		if !testCase.wantError {
			successCases = append(successCases, testCase)
		}
	}

	return successCases
}

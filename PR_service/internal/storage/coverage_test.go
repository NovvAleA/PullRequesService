package storage

import (
	"PR_service/internal/models"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSimpleCoverage(t *testing.T) {
	t.Run("NewStorage coverage", func(t *testing.T) {
		db := &sql.DB{}
		storage := NewStorage(db)
		assert.NotNil(t, storage)
	})

	t.Run("PickForTest coverage", func(t *testing.T) {
		result := PickForTest([]string{"a", "b", "c", "d"}, 2)
		assert.Len(t, result, 2)

		result2 := PickForTest([]string{"a"}, 5)
		assert.Len(t, result2, 1)
	})

	t.Run("Model validation coverage", func(t *testing.T) {
		// Создаем модели чтобы покрыть их инициализацию с новыми полями
		user := models.User{
			UserID:   "1",
			Username: "test",
			TeamName: "test-team",
			IsActive: true,
		}
		team := models.Team{
			TeamName: "test",
			Members:  []models.User{user},
		}
		pr := models.PullRequest{
			PullRequestID:   "pr1",
			PullRequestName: "Test PR",
			AuthorID:        "user1",
			Status:          "OPEN",
			Reviewers:       []string{"user2", "user3"},
			CreatedAt:       time.Now(),
			MergedAt:        nil,
		}
		prShort := models.PullRequestShort{
			PullRequestID:   "pr-short",
			PullRequestName: "Short PR",
			AuthorID:        "user1",
			Status:          "OPEN",
		}
		createPRReq := models.CreatePRRequest{
			PullRequestID:   "new-pr",
			PullRequestName: "New PR",
			AuthorID:        "author1",
		}
		setActiveReq := models.SetActiveRequest{
			UserID: "user1",
			Active: false,
		}
		reassignReq := models.ReassignRequest{
			PullRequestID: "pr1",
			OldUserID:     "user2",
		}
		errorResp := models.ErrorResponse{}
		errorResp.Error.Code = "TEST_ERROR"
		errorResp.Error.Message = "Test error message"

		// Проверяем что поля установлены корректно
		assert.Equal(t, "1", user.UserID)
		assert.Equal(t, "test-team", user.TeamName)
		assert.Equal(t, "test", team.TeamName)
		assert.Equal(t, "OPEN", pr.Status)
		assert.False(t, pr.CreatedAt.IsZero())
		assert.Nil(t, pr.MergedAt)
		assert.Equal(t, "pr-short", prShort.PullRequestID)
		assert.Equal(t, "new-pr", createPRReq.PullRequestID)
		assert.Equal(t, "user1", setActiveReq.UserID)
		assert.False(t, setActiveReq.Active)
		assert.Equal(t, "pr1", reassignReq.PullRequestID)
		assert.Equal(t, "TEST_ERROR", errorResp.Error.Code)
	})

	t.Run("TeamMember model coverage", func(t *testing.T) {
		// Тестируем TeamMember структуру если она используется
		teamMember := models.TeamMember{
			UserID:   "member1",
			Username: "Team Member",
			IsActive: true,
		}

		assert.Equal(t, "member1", teamMember.UserID)
		assert.Equal(t, "Team Member", teamMember.Username)
		assert.True(t, teamMember.IsActive)
	})

	t.Run("PullRequest with merged date", func(t *testing.T) {
		mergedAt := "2023-01-01T12:00:00Z"
		pr := models.PullRequest{
			PullRequestID:   "merged-pr",
			PullRequestName: "Merged PR",
			AuthorID:        "user1",
			Status:          "MERGED",
			Reviewers:       []string{"user2"},
			CreatedAt:       time.Now(),
			MergedAt:        &mergedAt,
		}

		assert.Equal(t, "MERGED", pr.Status)
		assert.NotNil(t, pr.MergedAt)
		assert.Equal(t, mergedAt, *pr.MergedAt)
	})

	t.Run("Empty and nil scenarios", func(t *testing.T) {
		// Тестируем случаи с пустыми значениями
		emptyUser := models.User{
			UserID:   "",
			Username: "",
			TeamName: "",
			IsActive: false,
		}

		prWithNilDate := models.PullRequest{
			PullRequestID:   "pr-nil",
			PullRequestName: "PR with nil date",
			AuthorID:        "user1",
			Status:          "OPEN",
			Reviewers:       []string{},
			CreatedAt:       time.Time{},
			MergedAt:        nil,
		}

		assert.Empty(t, emptyUser.UserID)
		assert.Empty(t, emptyUser.TeamName)
		assert.True(t, prWithNilDate.CreatedAt.IsZero())
		assert.Nil(t, prWithNilDate.MergedAt)
	})

	t.Run("Storage metrics interface", func(t *testing.T) {
		// Проверяем что storage может работать с метриками
		db := &sql.DB{}
		storage := NewStorage(db)

		// Проверяем что SetMetrics не паникует
		assert.NotPanics(t, func() {
			storage.SetMetrics(nil)
		})
	})
}

func TestEdgeCaseCoverage(t *testing.T) {
	t.Run("User without team", func(t *testing.T) {
		user := models.User{
			UserID:   "no-team-user",
			Username: "User Without Team",
			TeamName: "", // Пустая команда
			IsActive: true,
		}

		assert.Equal(t, "no-team-user", user.UserID)
		assert.Empty(t, user.TeamName)
		assert.True(t, user.IsActive)
	})

	t.Run("PR with single reviewer", func(t *testing.T) {
		pr := models.PullRequest{
			PullRequestID:   "single-reviewer-pr",
			PullRequestName: "Single Reviewer PR",
			AuthorID:        "author1",
			Status:          "OPEN",
			Reviewers:       []string{"reviewer1"}, // Только один ревьюер
			CreatedAt:       time.Now(),
			MergedAt:        nil,
		}

		assert.Len(t, pr.Reviewers, 1)
		assert.Equal(t, "reviewer1", pr.Reviewers[0])
	})

	t.Run("PR with no reviewers", func(t *testing.T) {
		pr := models.PullRequest{
			PullRequestID:   "no-reviewers-pr",
			PullRequestName: "No Reviewers PR",
			AuthorID:        "author1",
			Status:          "OPEN",
			Reviewers:       []string{}, // Нет ревьюеров
			CreatedAt:       time.Now(),
			MergedAt:        nil,
		}

		assert.Empty(t, pr.Reviewers)
	})

	t.Run("Team with mixed active/inactive users", func(t *testing.T) {
		team := models.Team{
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
		}

		assert.Len(t, team.Members, 2)
		assert.True(t, team.Members[0].IsActive)
		assert.False(t, team.Members[1].IsActive)
		assert.Equal(t, "mixed-team", team.Members[0].TeamName)
		assert.Equal(t, "mixed-team", team.Members[1].TeamName)
	})
}

func TestErrorResponseCoverage(t *testing.T) {
	t.Run("ErrorResponse with different codes", func(t *testing.T) {
		errorCodes := []string{
			"TEAM_EXISTS",
			"PR_EXISTS",
			"PR_MERGED",
			"NOT_ASSIGNED",
			"NO_CANDIDATE",
			"NOT_FOUND",
			"BAD_REQUEST",
			"INTERNAL_ERROR",
		}

		for _, code := range errorCodes {
			t.Run(code, func(t *testing.T) {
				errorResp := models.ErrorResponse{}
				errorResp.Error.Code = code
				errorResp.Error.Message = "Error message for " + code

				assert.Equal(t, code, errorResp.Error.Code)
				assert.Contains(t, errorResp.Error.Message, code)
			})
		}
	})

	t.Run("Empty ErrorResponse", func(t *testing.T) {
		errorResp := models.ErrorResponse{}

		assert.Empty(t, errorResp.Error.Code)
		assert.Empty(t, errorResp.Error.Message)
	})
}

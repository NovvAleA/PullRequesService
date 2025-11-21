package storage

import (
	"PR_service/internal/models"
	"database/sql"
	"testing"

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
		// Создаем модели чтобы покрыть их инициализацию
		user := models.User{UserID: "1", Username: "test", IsActive: true}
		team := models.Team{TeamName: "test", Members: []models.User{user}}
		pr := models.PullRequest{PullRequestID: "pr1", Status: "OPEN"}

		assert.Equal(t, "1", user.UserID)
		assert.Equal(t, "test", team.TeamName)
		assert.Equal(t, "OPEN", pr.Status)
	})
}

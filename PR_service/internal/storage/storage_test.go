package storage_test

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/example/pr-reviewer/internal/models"
	"github.com/example/pr-reviewer/internal/storage"
)

func newTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.ApplyMigrations(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCreatePR(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	s := storage.NewStorage(db)

	// Setup team
	err := s.UpsertTeam(ctx, models.Team{
		TeamName: "backend",
		Members: []models.User{
			{UserID: "u1", Username: "Mark", IsActive: true},
			{UserID: "u2", Username: "Alex", IsActive: true},
			{UserID: "u3", Username: "John", IsActive: false},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create PR
	pr, err := s.CreatePR(ctx, models.CreatePRRequest{
		PullRequestID:   "pr1",
		PullRequestName: "Fix critical bug",
		AuthorID:        "u1",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Should have up to 2 reviewers, none is author, and all active
	if len(pr.Reviewers) == 0 {
		t.Fatal("expected reviewers")
	}
	for _, r := range pr.Reviewers {
		if r == "u1" {
			t.Fatal("author cannot be reviewer")
		}
		if r == "u3" {
			t.Fatal("inactive user assigned as reviewer")
		}
	}
}

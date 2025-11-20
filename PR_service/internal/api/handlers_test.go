package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/example/pr-reviewer/internal/api"
	"github.com/example/pr-reviewer/internal/models"
)

// ---- простой мок стораджа ----

type mockStore struct {
	CreatePRFunc func(ctx context.Context, req models.CreatePRRequest) (*models.PullRequest, error)
}

func (m *mockStore) CreatePR(ctx context.Context, req models.CreatePRRequest) (*models.PullRequest, error) {
	return m.CreatePRFunc(ctx, req)
}

// ---- тест хэндлера ----

func TestCreatePRHandler(t *testing.T) {
	tests := []struct {
		name           string
		body           interface{}
		mockReturn     *models.PullRequest
		mockErr        error
		wantStatusCode int
	}{
		{
			name: "valid request",
			body: models.CreatePRRequest{
				PullRequestID:   "pr1",
				PullRequestName: "Fix bug",
				AuthorID:        "u1",
			},
			mockReturn: &models.PullRequest{
				PullRequestID:   "pr1",
				PullRequestName: "Fix bug",
				AuthorID:        "u1",
				Status:          "OPEN",
			},
			wantStatusCode: http.StatusCreated,
		},
		{
			name:           "missing fields",
			body:           map[string]interface{}{"pull_request_id": "pr1"},
			wantStatusCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			m := &mockStore{
				CreatePRFunc: func(ctx context.Context, req models.CreatePRRequest) (*models.PullRequest, error) {
					return tt.mockReturn, tt.mockErr
				},
			}

			h := api.NewHandler(m)

			b, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/pr/create", bytes.NewReader(b))
			w := httptest.NewRecorder()

			h.CreatePR(w, req)

			res := w.Result()
			if res.StatusCode != tt.wantStatusCode {
				t.Fatalf("got %d, want %d", res.StatusCode, tt.wantStatusCode)
			}
		})
	}
}

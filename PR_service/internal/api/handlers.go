package api

import (
	"context"
	"encoding/json"
	"net/http"

	//"github.com/gorilla/mux"

	"log"

	"github.com/example/pr-reviewer/internal/models"
	"github.com/example/pr-reviewer/internal/storage"
)

type Handler struct {
	store *storage.StorageData
	ctx   context.Context
}

func NewHandler(s *storage.StorageData) *Handler {
	return &Handler{store: s, ctx: context.Background()}
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func (h *Handler) AddTeam(w http.ResponseWriter, r *http.Request) {
	var t models.Team
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if t.TeamName == "" {
		writeError(w, http.StatusBadRequest, "team_name required")
		return
	}
	if err := h.store.UpsertTeam(h.ctx, t); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "Created"})
}

func (h *Handler) SetIsActive(w http.ResponseWriter, r *http.Request) {
	var sreq models.SetActiveRequest
	if err := json.NewDecoder(r.Body).Decode(&sreq); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if sreq.UserID == "" {
		writeError(w, http.StatusBadRequest, "user_id required")
		return
	}
	if err := h.store.SetUserActive(h.ctx, sreq.UserID, sreq.Active); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) CreatePR(w http.ResponseWriter, r *http.Request) {
	var req models.CreatePRRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Валидация полей
	if req.PullRequestID == "" {
		writeError(w, http.StatusBadRequest, "pull_request_id is required")
		return
	}
	if req.PullRequestName == "" {
		writeError(w, http.StatusBadRequest, "pull_request_name is required")
		return
	}
	if req.AuthorID == "" {
		writeError(w, http.StatusBadRequest, "author_id is required")
		return
	}

	// Создаем PR и получаем информацию о созданном PR
	createdPR, err := h.store.CreatePR(h.ctx, req)
	if err != nil {
		// Маппинг ошибок на HTTP статусы
		switch err.Error() {
		case "pr already exists":
			writeError(w, http.StatusConflict, "PR id already exists")
		case "author not found":
			writeError(w, http.StatusNotFound, "author not found")
		case "author is not in any team":
			writeError(w, http.StatusNotFound, "author team not found")
		default:
			log.Printf("CreatePR error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	// Возвращаем созданный PR согласно OpenAPI спецификации
	response := map[string]interface{}{
		"pr": createdPR,
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *Handler) MergePR(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PullRequestID string `json:"pull_request_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.PullRequestID == "" {
		writeError(w, http.StatusBadRequest, "pull_request_id is required")
		return
	}

	// Мерджим PR и получаем обновленный объект
	mergedPR, err := h.store.MergePR(h.ctx, req.PullRequestID)
	if err != nil {
		if err.Error() == "pr not found" {
			writeError(w, http.StatusNotFound, "PR not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Возвращаем обновленный PR согласно спецификации
	response := map[string]interface{}{
		"pr": mergedPR,
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) ReassignReviewer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PullRequestID string `json:"pull_request_id"`
		OldUserID     string `json:"old_user_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.PullRequestID == "" {
		writeError(w, http.StatusBadRequest, "pull_request_id is required")
		return
	}
	if req.OldUserID == "" {
		writeError(w, http.StatusBadRequest, "old_user_id is required")
		return
	}

	// Переназначаем ревьюера и получаем обновленный PR
	updatedPR, replacedBy, err := h.store.ReassignReviewer(h.ctx, req.PullRequestID, req.OldUserID)
	if err != nil {
		// Маппинг ошибок на HTTP статусы
		switch err.Error() {
		case "pr not found", "old reviewer not in any team":
			writeError(w, http.StatusNotFound, err.Error())
		case "cannot modify reviewers after merge":
			writeError(w, http.StatusConflict, "cannot reassign on merged PR")
		case "reviewer is not assigned to this PR":
			writeError(w, http.StatusConflict, "reviewer is not assigned to this PR")
		case "no active replacement candidate in team":
			writeError(w, http.StatusConflict, "no active replacement candidate in team")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	// Возвращаем ответ согласно спецификации
	response := map[string]interface{}{
		"pr":          updatedPR,
		"replaced_by": replacedBy,
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) GetPRsForUser(w http.ResponseWriter, r *http.Request) {
	// Получаем user_id из query параметров
	uid := r.URL.Query().Get("user_id")
	if uid == "" {
		writeError(w, http.StatusBadRequest, "user_id required")
		return
	}

	prs, err := h.store.GetPRsForUser(h.ctx, uid)
	if err != nil {
		log.Printf("GetPRsForUser error: %v", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Форматируем ответ согласно OpenAPI спецификации
	response := map[string]interface{}{
		"user_id":       uid,
		"pull_requests": prs,
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) GetTeam(w http.ResponseWriter, r *http.Request) {
	// Получаем team_name из query параметров
	teamName := r.URL.Query().Get("team_name")
	if teamName == "" {
		writeError(w, http.StatusBadRequest, "team_name parameter is required")
		return
	}

	team, err := h.store.GetTeam(h.ctx, teamName)
	if err != nil {
		if err.Error() == "team not found" {
			writeError(w, http.StatusNotFound, "team not found")
			return
		}
		log.Printf("GetTeam error: %v", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Форматируем ответ согласно OpenAPI спецификации
	response := map[string]interface{}{
		"team": team,
	}
	writeJSON(w, http.StatusOK, response)
}

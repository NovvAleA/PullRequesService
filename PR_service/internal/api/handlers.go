package api

import (
	"log"
	"net/http"

	"github.com/example/pr-reviewer/internal/models"
	"github.com/example/pr-reviewer/internal/storage"
)

type Handler struct {
	store *storage.StorageData
}

func NewHandler(s *storage.StorageData) *Handler {
	return &Handler{store: s}
}

func (h *Handler) AddTeam(w http.ResponseWriter, r *http.Request) {
	var t models.Team
	if !h.bindJSON(w, r, &t) {
		return
	}

	if errMsg := validateRequiredFields(map[string]string{
		"team_name": t.TeamName,
	}); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if err := h.store.UpsertTeam(r.Context(), t); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeSuccess(w, http.StatusCreated, "team created")
}

func (h *Handler) GetTeam(w http.ResponseWriter, r *http.Request) {
	teamName := r.URL.Query().Get("team_name")
	if teamName == "" {
		writeError(w, http.StatusBadRequest, "team_name query parameter is required")
		return
	}

	team, err := h.store.GetTeam(r.Context(), teamName)
	if err != nil {
		h.handleStorageError(w, err, "GetTeam")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"team": team})
}

func (h *Handler) SetIsActive(w http.ResponseWriter, r *http.Request) {
	var req models.SetActiveRequest
	if !h.bindJSON(w, r, &req) {
		return
	}

	if errMsg := validateRequiredFields(map[string]string{
		"user_id": req.UserID,
	}); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if err := h.store.SetUserActive(r.Context(), req.UserID, req.Active); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeSuccess(w, http.StatusOK, "user updated")
}

func (h *Handler) CreatePR(w http.ResponseWriter, r *http.Request) {
	var req models.CreatePRRequest
	if !h.bindJSON(w, r, &req) {
		return
	}

	if errMsg := validateRequiredFields(map[string]string{
		"pull_request_id":   req.PullRequestID,
		"pull_request_name": req.PullRequestName,
		"author_id":         req.AuthorID,
	}); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	createdPR, err := h.store.CreatePR(r.Context(), req)
	if err != nil {
		h.handleCreatePRError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{"pr": createdPR})
}

func (h *Handler) MergePR(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PullRequestID string `json:"pull_request_id"`
	}

	if !h.bindJSON(w, r, &req) {
		return
	}

	if req.PullRequestID == "" {
		writeError(w, http.StatusBadRequest, "pull_request_id is required")
		return
	}

	mergedPR, err := h.store.MergePR(r.Context(), req.PullRequestID)
	if err != nil {
		h.handleStorageError(w, err, "MergePR")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"pr": mergedPR})
}

func (h *Handler) ReassignReviewer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PullRequestID string `json:"pull_request_id"`
		OldUserID     string `json:"old_user_id"`
	}

	if !h.bindJSON(w, r, &req) {
		return
	}

	if errMsg := validateRequiredFields(map[string]string{
		"pull_request_id": req.PullRequestID,
		"old_user_id":     req.OldUserID,
	}); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	updatedPR, replacedBy, err := h.store.ReassignReviewer(r.Context(), req.PullRequestID, req.OldUserID)
	if err != nil {
		h.handleReassignError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"pr":          updatedPR,
		"replaced_by": replacedBy,
	})
}

func (h *Handler) GetPRsForUser(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("user_id")
	if uid == "" {
		writeError(w, http.StatusBadRequest, "user_id query parameter is required")
		return
	}

	prs, err := h.store.GetPRsForUser(r.Context(), uid)
	if err != nil {
		log.Printf("GetPRsForUser error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":       uid,
		"pull_requests": prs,
	})
}

// Вспомогательные функции для обработки ошибок
func (h *Handler) handleStorageError(w http.ResponseWriter, err error, handlerName string) {
	log.Printf("%s error: %v", handlerName, err)

	switch err.Error() {
	case "pr not found", "team not found", "user not found", "author not found",
		"author is not in any team", "old reviewer not in any team":
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func (h *Handler) handleCreatePRError(w http.ResponseWriter, err error) {
	log.Printf("CreatePR error: %v", err)

	switch err.Error() {
	case "pr already exists":
		writeError(w, http.StatusConflict, "PR id already exists")
	case "author not found", "author is not in any team":
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func (h *Handler) handleReassignError(w http.ResponseWriter, err error) {
	log.Printf("ReassignReviewer error: %v", err)

	switch err.Error() {
	case "pr not found", "user not found", "user not in any team", "old reviewer not in any team":
		// ДОБАВИТЬ "old reviewer not in any team" в 404 ошибки
		writeError(w, http.StatusNotFound, err.Error())
	case "cannot modify reviewers after merge", "reviewer is not assigned to this PR",
		"no active replacement candidate in team":
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

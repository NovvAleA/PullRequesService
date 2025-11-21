package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"PR_service/internal/models"
	"PR_service/internal/storage"
)

type Handler struct {
	store   *storage.StorageData
	metrics *Metrics
}

func (h *Handler) Metrics() *Metrics {
	return h.metrics
}

func NewHandler(s *storage.StorageData) *Handler {
	return &Handler{
		store:   s,
		metrics: NewMetrics(), // Инициализируем метрики
	}
}

func (h *Handler) AddTeam(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var t models.Team
	if !h.bindJSON(w, r, &t) {
		return
	}

	if errMsg := validateRequiredFields(map[string]string{
		"team_name": t.TeamName,
	}); errMsg != "" {
		h.metrics.httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, "400").Inc()
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if err := h.store.UpsertTeam(r.Context(), t); err != nil {
		h.metrics.httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, "500").Inc()
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Метрики для команды
	h.metrics.SetTeamMembersCount(t.TeamName, len(t.Members))
	h.metrics.ObserveDBQuery("upsert", "teams", time.Since(start))

	writeSuccess(w, http.StatusCreated, "team created")
}

func (h *Handler) GetTeam(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	teamName := r.URL.Query().Get("team_name")
	if teamName == "" {
		h.metrics.httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, "400").Inc()
		writeError(w, http.StatusBadRequest, "team_name query parameter is required")
		return
	}

	team, err := h.store.GetTeam(r.Context(), teamName)
	if err != nil {
		h.metrics.httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, "404").Inc()
		h.handleStorageError(w, err, "GetTeam")
		return
	}

	h.metrics.ObserveDBQuery("select", "teams", time.Since(start))

	writeJSON(w, http.StatusOK, map[string]interface{}{"team": team})
}

func (h *Handler) SetIsActive(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req models.SetActiveRequest
	if !h.bindJSON(w, r, &req) {
		return
	}

	if errMsg := validateRequiredFields(map[string]string{
		"user_id": req.UserID,
	}); errMsg != "" {
		h.metrics.httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, "400").Inc()
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if err := h.store.SetUserActive(r.Context(), req.UserID, req.Active); err != nil {
		h.metrics.httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, "500").Inc()
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.metrics.ObserveDBQuery("update", "users", time.Since(start))

	writeSuccess(w, http.StatusOK, "user updated")
}

func (h *Handler) CreatePR(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req models.CreatePRRequest
	if !h.bindJSON(w, r, &req) {
		return
	}

	if errMsg := validateRequiredFields(map[string]string{
		"pull_request_id":   req.PullRequestID,
		"pull_request_name": req.PullRequestName,
		"author_id":         req.AuthorID,
	}); errMsg != "" {
		h.metrics.httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, "400").Inc()
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	createdPR, err := h.store.CreatePR(r.Context(), req)
	if err != nil {
		h.metrics.httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, "500").Inc()
		h.handleCreatePRError(w, err)
		return
	}

	// Бизнес-метрики
	h.metrics.IncPRCreated()
	h.metrics.ObserveReviewersAssigned("unknown", len(createdPR.Reviewers))
	h.metrics.ObserveDBQuery("create", "pull_requests", time.Since(start))

	writeJSON(w, http.StatusCreated, map[string]interface{}{"pr": createdPR})
}

func (h *Handler) MergePR(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req struct {
		PullRequestID string `json:"pull_request_id"`
	}

	if !h.bindJSON(w, r, &req) {
		return
	}

	if req.PullRequestID == "" {
		h.metrics.httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, "400").Inc()
		writeError(w, http.StatusBadRequest, "pull_request_id is required")
		return
	}

	mergedPR, err := h.store.MergePR(r.Context(), req.PullRequestID)
	if err != nil {
		h.metrics.httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, "500").Inc()
		h.handleStorageError(w, err, "MergePR")
		return
	}

	// Бизнес-метрики
	h.metrics.IncPRMerged()
	h.metrics.ObserveDBQuery("update", "pull_requests", time.Since(start))

	writeJSON(w, http.StatusOK, map[string]interface{}{"pr": mergedPR})
}

func (h *Handler) ReassignReviewer(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

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
		h.metrics.httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, "400").Inc()
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	updatedPR, replacedBy, err := h.store.ReassignReviewer(r.Context(), req.PullRequestID, req.OldUserID)
	if err != nil {
		h.metrics.httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, "500").Inc()
		h.handleReassignError(w, err)
		return
	}

	// Метрики для переназначения
	h.metrics.ObserveReviewersAssigned("unknown", len(updatedPR.Reviewers))
	h.metrics.ObserveDBQuery("update", "pr_reviewers", time.Since(start))

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"pr":          updatedPR,
		"replaced_by": replacedBy,
	})
}

func (h *Handler) GetPRsForUser(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	uid := r.URL.Query().Get("user_id")
	if uid == "" {
		h.metrics.httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, "400").Inc()
		writeError(w, http.StatusBadRequest, "user_id query parameter is required")
		return
	}

	prs, err := h.store.GetPRsForUser(r.Context(), uid)
	if err != nil {
		h.metrics.httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, "500").Inc()
		log.Printf("GetPRsForUser error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	h.metrics.ObserveDBQuery("select", "pull_requests", time.Since(start))

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

// HealthCheck выполняет комплексную проверку здоровья сервиса
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	// Собираем информацию о различных компонентах
	healthStatus := struct {
		Status    string            `json:"status"`
		Timestamp time.Time         `json:"timestamp"`
		Checks    map[string]string `json:"checks"`
		Version   string            `json:"version"`
	}{
		Status:    "healthy",
		Timestamp: time.Now().UTC(),
		Checks:    make(map[string]string),
		Version:   getVersion(), // функция для получения версии приложения
	}

	// Проверка 1: База данных
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := h.store.HealthCheck(ctx); err != nil {
		healthStatus.Status = "unhealthy"
		healthStatus.Checks["database"] = fmt.Sprintf("ERROR: %v", err)
		writeJSON(w, http.StatusServiceUnavailable, healthStatus)
		return
	}
	healthStatus.Checks["database"] = "OK"

	// Проверка 2: Доступность файловой системы (опционально)
	if _, err := os.Stat("."); err != nil {
		healthStatus.Checks["filesystem"] = fmt.Sprintf("WARNING: %v", err)
	} else {
		healthStatus.Checks["filesystem"] = "OK"
	}

	// Проверка 3: Память (опционально)
	if stat, err := getMemoryStats(); err != nil {
		healthStatus.Checks["memory"] = fmt.Sprintf("WARNING: %v", err)
	} else {
		healthStatus.Checks["memory"] = stat
	}

	// Определяем HTTP статус
	statusCode := http.StatusOK
	if healthStatus.Status == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	}

	writeJSON(w, statusCode, healthStatus)
}

// getMemoryStats возвращает статистику использования памяти
func getMemoryStats() (string, error) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Преобразуем в мегабайты
	allocMB := m.Alloc / 1024 / 1024
	sysMB := m.Sys / 1024 / 1024

	return fmt.Sprintf("Alloc: %dMB, Sys: %dMB", allocMB, sysMB), nil
}

// getVersion возвращает версию приложения
func getVersion() string {
	if version := os.Getenv("APP_VERSION"); version != "" {
		return version
	}
	return "1.0.0"
}

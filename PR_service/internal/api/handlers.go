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

func NewHandler(s *storage.StorageData, m *Metrics) *Handler {
	if m != nil {
		s.SetMetrics(m)
	}

	return &Handler{
		store:   s,
		metrics: m,
	}
}

// Root обрабатывает корневой endpoint
func (h *Handler) Root(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer h.recordHandlerDuration(r, start, "200")

	WriteJSON(w, http.StatusOK, map[string]string{
		"service": "PR Reviewer Assignment Service",
		"version": "1.0.0",
		"status":  "running",
	})
}

func (h *Handler) AddTeam(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	status := "201"

	defer func() {
		h.recordHandlerDuration(r, start, status)
	}()

	var t models.Team
	if !h.bindJSON(w, r, &t) {
		status = "400"
		if h.metrics != nil {
			h.metrics.IncBusinessError("INVALID_REQUEST")
		}
		return
	}

	if errMsg := validateRequiredFields(map[string]string{
		"team_name": t.TeamName,
	}); errMsg != "" {
		status = "400"
		if h.metrics != nil {
			h.metrics.IncBusinessError("MISSING_REQUIRED_FIELDS")
		}
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if err := h.store.UpsertTeam(r.Context(), t); err != nil {
		status = "500"
		if h.metrics != nil {
			h.metrics.IncBusinessError("TEAM_CREATION_ERROR")
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Метрики для команды
	if h.metrics != nil {
		h.metrics.SetTeamMembersCount(t.TeamName, len(t.Members))
	}

	// Возвращаем команду в соответствии со спецификацией
	WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"team": t,
	})
}

func (h *Handler) GetTeam(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	status := "200"

	defer func() {
		h.recordHandlerDuration(r, start, status)
	}()

	teamName := r.URL.Query().Get("team_name")
	if teamName == "" {
		status = "400"
		if h.metrics != nil {
			h.metrics.IncBusinessError("MISSING_TEAM_NAME")
		}
		writeError(w, http.StatusBadRequest, "team_name query parameter is required")
		return
	}

	team, err := h.store.GetTeam(r.Context(), teamName)
	if err != nil {
		status = "404"
		if h.metrics != nil {
			h.metrics.IncBusinessError("TEAM_NOT_FOUND")
		}
		h.handleStorageError(w, err, "GetTeam")
		return
	}

	// Возвращаем команду в соответствии со спецификацией
	WriteJSON(w, http.StatusOK, team)
}

func (h *Handler) SetIsActive(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	status := "200"

	defer func() {
		h.recordHandlerDuration(r, start, status)
	}()

	var req models.SetActiveRequest
	if !h.bindJSON(w, r, &req) {
		status = "400"
		if h.metrics != nil {
			h.metrics.IncBusinessError("INVALID_REQUEST")
		}
		return
	}

	if errMsg := validateRequiredFields(map[string]string{
		"user_id": req.UserID,
	}); errMsg != "" {
		status = "400"
		if h.metrics != nil {
			h.metrics.IncBusinessError("MISSING_REQUIRED_FIELDS")
		}
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	if err := h.store.SetUserActive(r.Context(), req.UserID, req.Active); err != nil {
		status = "500"
		if h.metrics != nil {
			h.metrics.IncBusinessError("USER_UPDATE_ERROR")
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Получаем обновленного пользователя для ответа
	user, err := h.getUserWithTeam(r.Context(), req.UserID)
	if err != nil {
		// Если не удалось получить пользователя с командой, возвращаем простой ответ
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"status": "user updated",
		})
		return
	}

	// Возвращаем пользователя в соответствии со спецификацией
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"user": user,
	})
}

func (h *Handler) CreatePR(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	status := "201"

	defer func() {
		h.recordHandlerDuration(r, start, status)
	}()

	var req models.CreatePRRequest
	if !h.bindJSON(w, r, &req) {
		status = "400"
		if h.metrics != nil {
			h.metrics.IncBusinessError("INVALID_REQUEST")
		}
		return
	}

	if errMsg := validateRequiredFields(map[string]string{
		"pull_request_id":   req.PullRequestID,
		"pull_request_name": req.PullRequestName,
		"author_id":         req.AuthorID,
	}); errMsg != "" {
		status = "400"
		if h.metrics != nil {
			h.metrics.IncBusinessError("MISSING_REQUIRED_FIELDS")
		}
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	createdPR, err := h.store.CreatePR(r.Context(), req)
	if err != nil {
		status = "500"
		h.handleCreatePRError(w, err)
		return
	}

	// Бизнес-метрики
	if h.metrics != nil {
		h.metrics.IncPRCreated()

		// Получаем реальное имя команды автора
		teamName := h.getAuthorTeam(r.Context(), req.AuthorID)
		if teamName == "" {
			teamName = "unknown"
		}
		h.metrics.ObserveReviewersAssigned(teamName, len(createdPR.Reviewers))
	}

	// Возвращаем PR в соответствии со спецификацией
	WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"pr": createdPR,
	})
}

func (h *Handler) MergePR(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	status := "200"

	defer func() {
		h.recordHandlerDuration(r, start, status)
	}()

	var req struct {
		PullRequestID string `json:"pull_request_id"`
	}

	if !h.bindJSON(w, r, &req) {
		status = "400"
		if h.metrics != nil {
			h.metrics.IncBusinessError("INVALID_REQUEST")
		}
		return
	}

	if req.PullRequestID == "" {
		status = "400"
		if h.metrics != nil {
			h.metrics.IncBusinessError("MISSING_PR_ID")
		}
		writeError(w, http.StatusBadRequest, "pull_request_id is required")
		return
	}

	mergedPR, err := h.store.MergePR(r.Context(), req.PullRequestID)
	if err != nil {
		status = "500"
		h.handleStorageError(w, err, "MergePR")
		return
	}

	// Бизнес-метрики
	if h.metrics != nil {
		h.metrics.IncPRMerged()
	}

	// Возвращаем PR в соответствии со спецификацией
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"pr": mergedPR,
	})
}

func (h *Handler) ReassignReviewer(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	status := "200"

	defer func() {
		h.recordHandlerDuration(r, start, status)
	}()

	var req struct {
		PullRequestID string `json:"pull_request_id"`
		OldUserID     string `json:"old_user_id"`
	}

	if !h.bindJSON(w, r, &req) {
		status = "400"
		if h.metrics != nil {
			h.metrics.IncBusinessError("INVALID_REQUEST")
		}
		return
	}

	if errMsg := validateRequiredFields(map[string]string{
		"pull_request_id": req.PullRequestID,
		"old_user_id":     req.OldUserID,
	}); errMsg != "" {
		status = "400"
		if h.metrics != nil {
			h.metrics.IncBusinessError("MISSING_REQUIRED_FIELDS")
		}
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	updatedPR, replacedBy, err := h.store.ReassignReviewer(r.Context(), req.PullRequestID, req.OldUserID)
	if err != nil {
		status = "500"
		h.handleReassignError(w, err)
		return
	}

	// Метрики для переназначения
	if h.metrics != nil {
		teamName := h.getAuthorTeam(r.Context(), updatedPR.AuthorID)
		if teamName == "" {
			teamName = "unknown"
		}
		h.metrics.ObserveReviewersAssigned(teamName, len(updatedPR.Reviewers))
	}

	// Возвращаем ответ в соответствии со спецификацией
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"pr":          updatedPR,
		"replaced_by": replacedBy,
	})
}

func (h *Handler) GetPRsForUser(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	status := "200"

	defer func() {
		h.recordHandlerDuration(r, start, status)
	}()

	uid := r.URL.Query().Get("user_id")
	if uid == "" {
		status = "400"
		if h.metrics != nil {
			h.metrics.IncBusinessError("MISSING_USER_ID")
		}
		writeError(w, http.StatusBadRequest, "user_id query parameter is required")
		return
	}

	prs, err := h.store.GetPRsForUser(r.Context(), uid)
	if err != nil {
		status = "500"
		if h.metrics != nil {
			h.metrics.IncBusinessError("GET_PRS_ERROR")
		}
		log.Printf("GetPRsForUser error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Возвращаем в соответствии со спецификацией - PullRequestShort
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":       uid,
		"pull_requests": prs,
	})
}

// HealthCheck выполняет комплексную проверку здоровья сервиса
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer h.recordHandlerDuration(r, start, "200")

	healthStatus := struct {
		Status    string            `json:"status"`
		Timestamp time.Time         `json:"timestamp"`
		Checks    map[string]string `json:"checks"`
		Version   string            `json:"version"`
	}{
		Status:    "healthy",
		Timestamp: time.Now().UTC(),
		Checks:    make(map[string]string),
		Version:   getVersion(),
	}

	// Проверка 1: База данных
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := h.store.HealthCheck(ctx); err != nil {
		healthStatus.Status = "unhealthy"
		healthStatus.Checks["database"] = fmt.Sprintf("ERROR: %v", err)
		WriteJSON(w, http.StatusServiceUnavailable, healthStatus)
		return
	}
	healthStatus.Checks["database"] = "OK"

	// Проверка 2: Доступность файловой системы
	if _, err := os.Stat("."); err != nil {
		healthStatus.Checks["filesystem"] = fmt.Sprintf("WARNING: %v", err)
	} else {
		healthStatus.Checks["filesystem"] = "OK"
	}

	// Проверка 3: Память
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

	WriteJSON(w, statusCode, healthStatus)
}

// Вспомогательная функция для записи длительности хендлера
func (h *Handler) recordHandlerDuration(r *http.Request, start time.Time, status string) {
	if h.metrics != nil {
		duration := time.Since(start)
		h.metrics.RecordHTTPRequest(r.Method, r.URL.Path, status, duration)
		log.Printf("HANDLER DURATION: %s %s %s - %.6fs", r.Method, r.URL.Path, status, duration.Seconds())
	}
}

// Вспомогательные функции для обработки ошибок
func (h *Handler) handleStorageError(w http.ResponseWriter, err error, handlerName string) {
	log.Printf("%s error: %v", handlerName, err)

	if h.metrics != nil {
		h.metrics.IncBusinessError("STORAGE_ERROR")
	}

	// Создаем ErrorResponse в соответствии со спецификацией
	errorResp := models.ErrorResponse{}
	errorResp.Error.Message = err.Error()

	switch err.Error() {
	case "pr not found", "team not found", "user not found", "author not found",
		"author is not in any team", "old reviewer not in any team":
		errorResp.Error.Code = "NOT_FOUND"
		WriteJSON(w, http.StatusNotFound, errorResp)
	default:
		errorResp.Error.Code = "INTERNAL_ERROR"
		WriteJSON(w, http.StatusInternalServerError, errorResp)
	}
}

func (h *Handler) handleCreatePRError(w http.ResponseWriter, err error) {
	log.Printf("CreatePR error: %v", err)

	// Создаем ErrorResponse в соответствии со спецификацией
	errorResp := models.ErrorResponse{}
	errorResp.Error.Message = err.Error()

	if h.metrics != nil {
		switch err.Error() {
		case "pr already exists":
			h.metrics.IncBusinessError("PR_EXISTS")
			errorResp.Error.Code = "PR_EXISTS"
		case "author not found":
			h.metrics.IncBusinessError("AUTHOR_NOT_FOUND")
			errorResp.Error.Code = "NOT_FOUND"
		case "author is not in any team":
			h.metrics.IncBusinessError("AUTHOR_NO_TEAM")
			errorResp.Error.Code = "NOT_FOUND"
		default:
			h.metrics.IncBusinessError("PR_CREATION_ERROR")
			errorResp.Error.Code = "INTERNAL_ERROR"
		}
	}

	switch err.Error() {
	case "pr already exists":
		WriteJSON(w, http.StatusConflict, errorResp)
	case "author not found", "author is not in any team":
		WriteJSON(w, http.StatusNotFound, errorResp)
	default:
		WriteJSON(w, http.StatusInternalServerError, errorResp)
	}
}

func (h *Handler) handleReassignError(w http.ResponseWriter, err error) {
	log.Printf("ReassignReviewer error: %v", err)

	// Создаем ErrorResponse в соответствии со спецификацией
	errorResp := models.ErrorResponse{}
	errorResp.Error.Message = err.Error()

	if h.metrics != nil {
		switch err.Error() {
		case "cannot modify reviewers after merge":
			h.metrics.IncBusinessError("PR_ALREADY_MERGED")
			errorResp.Error.Code = "PR_MERGED"
		case "reviewer is not assigned to this PR":
			h.metrics.IncBusinessError("REVIEWER_NOT_ASSIGNED")
			errorResp.Error.Code = "NOT_ASSIGNED"
		case "no active replacement candidate in team":
			h.metrics.IncBusinessError("NO_REPLACEMENT_CANDIDATE")
			errorResp.Error.Code = "NO_CANDIDATE"
		default:
			h.metrics.IncBusinessError("REASSIGN_ERROR")
			errorResp.Error.Code = "INTERNAL_ERROR"
		}
	}

	switch err.Error() {
	case "pr not found", "user not found", "user not in any team", "old reviewer not in any team":
		errorResp.Error.Code = "NOT_FOUND"
		WriteJSON(w, http.StatusNotFound, errorResp)
	case "cannot modify reviewers after merge", "reviewer is not assigned to this PR",
		"no active replacement candidate in team":
		WriteJSON(w, http.StatusConflict, errorResp)
	default:
		errorResp.Error.Code = "INTERNAL_ERROR"
		WriteJSON(w, http.StatusInternalServerError, errorResp)
	}
}

// Вспомогательная функция для получения команды автора
func (h *Handler) getAuthorTeam(ctx context.Context, authorID string) string {
	// Получаем команду пользователя через существующий метод storage
	team, err := h.store.GetTeamByUserID(ctx, authorID)
	if err != nil {
		return ""
	}
	return team.TeamName
}

// Вспомогательная функция для получения пользователя с информацией о команде
func (h *Handler) getUserWithTeam(ctx context.Context, userID string) (*models.User, error) {
	// Находим команду пользователя
	team, err := h.store.GetTeamByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Находим пользователя в команде
	for _, member := range team.Members {
		if member.UserID == userID {
			// Создаем полную модель User с информацией о команде
			return &models.User{
				UserID:   member.UserID,
				Username: member.Username,
				TeamName: team.TeamName,
				IsActive: member.IsActive,
			}, nil
		}
	}

	return nil, fmt.Errorf("user not found in team")
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

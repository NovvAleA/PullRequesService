package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"PR_service/internal/models"
)

// WriteJSON универсальная функция для JSON ответов (теперь экспортирована)
func WriteJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("JSON encode error: %v", err)
		}
	}
}

// writeError универсальная функция для ошибок (теперь использует ErrorResponse)
func writeError(w http.ResponseWriter, statusCode int, message string) {
	errorResp := models.ErrorResponse{}
	errorResp.Error.Message = message

	// Устанавливаем код ошибки в зависимости от статуса
	switch statusCode {
	case 400:
		errorResp.Error.Code = "BAD_REQUEST"
	case 404:
		errorResp.Error.Code = "NOT_FOUND"
	case 409:
		errorResp.Error.Code = "CONFLICT"
	case 500:
		errorResp.Error.Code = "INTERNAL_ERROR"
	default:
		errorResp.Error.Code = "UNKNOWN_ERROR"
	}

	WriteJSON(w, statusCode, errorResp)
}

// writeSuccess универсальная функция для успешных операций
func writeSuccess(w http.ResponseWriter, statusCode int, message string) {
	WriteJSON(w, statusCode, map[string]string{"status": message})
}

// bindJSON универсальная функция для парсинга JSON тела
func (h *Handler) bindJSON(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

// validateRequiredFields проверяет обязательные поля
func validateRequiredFields(fields map[string]string) string {
	for field, value := range fields {
		if value == "" {
			return field + " is required"
		}
	}
	return ""
}

// formatDateTime форматирует время в строку RFC3339 (для JSON ответов)
func formatDateTime(t time.Time) string {
	return t.Format(time.RFC3339)
}

// parseDateTime парсит строку времени из RFC3339
func parseDateTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

// createErrorResponse создает стандартизированный ответ с ошибкой
func createErrorResponse(code, message string) models.ErrorResponse {
	return models.ErrorResponse{
		Error: struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}{
			Code:    code,
			Message: message,
		},
	}
}

// createTeamResponse создает ответ для операций с командой
func createTeamResponse(team models.Team) map[string]interface{} {
	return map[string]interface{}{
		"team": team,
	}
}

// createUserResponse создает ответ для операций с пользователем
func createUserResponse(user models.User) map[string]interface{} {
	return map[string]interface{}{
		"user": user,
	}
}

// createPRResponse создает ответ для операций с PR
func createPRResponse(pr models.PullRequest) map[string]interface{} {
	return map[string]interface{}{
		"pr": pr,
	}
}

// createPRShortResponse создает ответ для операций с коротким PR
func createPRShortResponse(prs []models.PullRequestShort, userID string) map[string]interface{} {
	return map[string]interface{}{
		"user_id":       userID,
		"pull_requests": prs,
	}
}

// createReassignResponse создает ответ для операции переназначения
func createReassignResponse(pr models.PullRequest, replacedBy string) map[string]interface{} {
	return map[string]interface{}{
		"pr":          pr,
		"replaced_by": replacedBy,
	}
}

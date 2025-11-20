package api

import (
	"encoding/json"
	"log"
	"net/http"
)

// writeJSON универсальная функция для JSON ответов
func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("JSON encode error: %v", err)
		}
	}
}

// writeError универсальная функция для ошибок
func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]string{"error": message})
}

// writeSuccess универсальная функция для успешных операций
func writeSuccess(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]string{"status": message})
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

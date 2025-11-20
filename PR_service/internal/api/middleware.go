package api

import (
	"context"
	"net/http"
	"time"
)

const RequestTimeout = 300 * time.Millisecond

// TimeoutMiddleware добавляет таймаут ко всем HTTP-запросам
func TimeoutMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Создаём контекст с таймаутом
		ctx, cancel := context.WithTimeout(r.Context(), RequestTimeout)
		defer cancel()

		// Подменяем контекст запроса
		r = r.WithContext(ctx)

		// Канал, который закроется, если запрос завершён
		done := make(chan struct{})

		go func() {
			next.ServeHTTP(w, r)
			close(done)
		}()

		select {
		case <-ctx.Done():
			// Таймаут или отмена клиента
			http.Error(w, "request timed out", http.StatusGatewayTimeout)
			return
		case <-done:
			return
		}
	})
}

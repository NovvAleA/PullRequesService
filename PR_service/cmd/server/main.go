package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"PR_service/internal/api"
	"PR_service/internal/storage"

	"github.com/gorilla/mux"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	// Конфигурация
	dbURL := getEnv("DATABASE_URL", "postgres://pguser:password@localhost:5432/pr_reviewer_db?sslmode=disable")
	port := getEnv("PORT", "8080")

	// Инициализация БД
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Проверяем подключение к БД
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Применяем миграции
	log.Println("Applying database migrations...")
	if err := storage.ApplyMigrations(db); err != nil {
		log.Fatalf("Failed to apply migrations: %v", err)
	}
	log.Println("Migrations applied successfully")

	// Инициализация storage
	store := storage.NewStorage(db)

	// Инициализация метрик
	metrics := api.NewMetrics()

	// Инициализация handler с метриками
	handler := api.NewHandler(store, metrics)

	// Настройка роутинга
	router := mux.NewRouter()

	// Middleware
	router.Use(metrics.MetricsMiddleware) // Метрики HTTP запросов
	router.Use(api.TimeoutMiddleware)     // Таймауты

	// API routes
	// Root endpoint
	router.HandleFunc("/", handler.Root).Methods("GET")

	// Teams endpoints
	router.HandleFunc("/team/add", handler.AddTeam).Methods("POST")
	router.HandleFunc("/team/get", handler.GetTeam).Methods("GET")

	// Users endpoints
	router.HandleFunc("/users/setIsActive", handler.SetIsActive).Methods("POST")
	router.HandleFunc("/users/getReview", handler.GetPRsForUser).Methods("GET")

	// Pull Requests endpoints
	router.HandleFunc("/pullRequest/create", handler.CreatePR).Methods("POST")
	router.HandleFunc("/pullRequest/merge", handler.MergePR).Methods("POST")
	router.HandleFunc("/pullRequest/reassign", handler.ReassignReviewer).Methods("POST")

	// Health and metrics endpoints
	router.HandleFunc("/health", handler.HealthCheck).Methods("GET")
	router.Handle("/metrics", metrics.InstrumentedHandler()).Methods("GET")
	router.HandleFunc("/metrics/data", handler.MetricsData).Methods("GET")

	// Настройка HTTP сервера
	srv := &http.Server{
		//Addr:         ":" + port,
		Addr:         "0.0.0.0:" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	done := make(chan bool, 1)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Server is shutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		srv.SetKeepAlivesEnabled(false)
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("Could not gracefully shutdown the server: %v", err)
		}
		close(done)
	}()

	log.Printf("Server is running on port %s", port)
	log.Println("Available endpoints:")
	log.Println("  GET  /")
	log.Println("  GET  /health")
	log.Println("  POST /team/add")
	log.Println("  GET  /team/get")
	log.Println("  POST /users/setIsActive")
	log.Println("  GET  /users/getReview")
	log.Println("  POST /pullRequest/create")
	log.Println("  POST /pullRequest/merge")
	log.Println("  POST /pullRequest/reassign")
	log.Println("  GET  /metrics")
	log.Println("  GET  /metrics/data")

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Could not listen on port %s: %v", port, err)
	}

	<-done
	log.Println("Server stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

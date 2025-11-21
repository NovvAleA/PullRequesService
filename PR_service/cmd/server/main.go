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

	"github.com/gorilla/mux"
	_ "github.com/jackc/pgx/v5/stdlib"

	"PR_service/internal/api"
	"PR_service/internal/storage"
)

func main() {
	startServer()
}

func startServer() {
	dsn := getDatabaseDSN()

	// Ждем подключения к БД
	db, err := waitForDB(dsn)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer db.Close()

	// Настройка пула соединений
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)

	// Применяем миграции
	if err := storage.ApplyMigrations(db); err != nil {
		log.Fatalf("apply migrations: %v", err)
	}

	st := storage.NewStorage(db)
	h := api.NewHandler(st)

	// Устанавливаем метрики в storage
	st.SetMetrics(h.Metrics())

	r := mux.NewRouter()

	r.Use(h.Metrics().MetricsMiddleware)
	r.Handle("/metrics", h.Metrics().InstrumentedHandler()).Methods("GET")

	// Остальные маршруты
	r.HandleFunc("/team/add", h.AddTeam).Methods("POST")
	r.HandleFunc("/team/get", h.GetTeam).Methods("GET")
	r.HandleFunc("/users/setIsActive", h.SetIsActive).Methods("POST")
	r.HandleFunc("/users/getReview", h.GetPRsForUser).Methods("GET")
	r.HandleFunc("/pullRequest/create", h.CreatePR).Methods("POST")
	r.HandleFunc("/pullRequest/merge", h.MergePR).Methods("POST")
	r.HandleFunc("/pullRequest/reassign", h.ReassignReviewer).Methods("POST")
	r.HandleFunc("/health", h.HealthCheck).Methods("GET")

	// Создаем HTTP сервер с настройками
	server := &http.Server{
		Addr:         ":" + getPort(),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	done := make(chan bool, 1)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Server is shutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Fatalf("Could not gracefully shutdown the server: %v", err)
		}
		close(done)
	}()

	log.Printf("🚀 Server listening on :%s", getPort())

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Could not listen on %s: %v", getPort(), err)
	}

	<-done
	log.Println("Server stopped")
}

func getDatabaseDSN() string {
	// В Docker используем переменную окружения
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		return dsn
	}

	// Для разработки - проверяем оба варианта
	hosts := []string{"db", "localhost"}
	for _, host := range hosts {
		testDSN := "postgres://pguser:password@" + host + ":5432/pr_reviewer_db?sslmode=disable"
		log.Printf("Trying database at: %s", host)

		db, err := sql.Open("pgx", testDSN)
		if err != nil {
			continue
		}

		if err := db.Ping(); err == nil {
			db.Close()
			log.Printf("Connected to database at: %s", host)
			return testDSN
		}
		db.Close()
	}

	// Fallback
	return "postgres://pguser:password@localhost:5432/pr_reviewer_db?sslmode=disable"
}

func waitForDB(dsn string) (*sql.DB, error) {
	var db *sql.DB
	var err error

	// Пытаемся подключиться несколько раз
	for i := 0; i < 10; i++ {
		db, err = sql.Open("pgx", dsn)
		if err != nil {
			log.Printf("Database connection attempt %d failed: %v", i+1, err)
			time.Sleep(2 * time.Second)
			continue
		}

		if err = db.Ping(); err == nil {
			return db, nil
		}

		log.Printf("Database ping attempt %d failed: %v", i+1, err)
		db.Close()
		time.Sleep(2 * time.Second)
	}

	return nil, err
}

func getPort() string {
	if p := os.Getenv("PORT"); p != "" {
		return p
	}
	return "8080"
}

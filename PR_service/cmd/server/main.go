package main

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/example/pr-reviewer/internal/api"
	"github.com/example/pr-reviewer/internal/storage"
)

func main() {
	// Для локальной разработки используем localhost
	dsn := "postgres://pguser:password@localhost:5432/pr_reviewer_db?sslmode=disable"

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}

	// Остальной код без изменений...
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)

	if err := storage.ApplyMigrations(db); err != nil {
		log.Fatalf("apply migrations: %v", err)
	}

	st := storage.NewStorage(db)
	h := api.NewHandler(st)

	r := mux.NewRouter()
	r.Use(api.TimeoutMiddleware) //устанавливаю таймауты
	
	r.HandleFunc("/team/add", h.AddTeam).Methods("POST")
	r.HandleFunc("/team/get", h.GetTeam).Methods("GET") //

	r.HandleFunc("/users/setIsActive", h.SetIsActive).Methods("POST")
	r.HandleFunc("/users/getReview", h.GetPRsForUser).Methods("GET")

	r.HandleFunc("/pullRequest/create", h.CreatePR).Methods("POST")
	r.HandleFunc("/pullRequest/merge", h.MergePR).Methods("POST")
	r.HandleFunc("/pullRequest/reassign", h.ReassignReviewer).Methods("POST")

	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods("GET")

	log.Printf("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

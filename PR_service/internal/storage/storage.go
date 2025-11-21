package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"PR_service/internal/models"
)

type StorageData struct {
	db      *sql.DB
	metrics MetricsInterface // Интерфейс для метрик
}

type MetricsInterface interface {
	ObserveDBQuery(operation, table string, duration time.Duration)
	SetDBConnections(count int)
}

func NewStorage(db *sql.DB) *StorageData {
	return &StorageData{db: db}
}

// SetMetrics устанавливает метрики (можно вызвать после инициализации)
func (s *StorageData) SetMetrics(metrics MetricsInterface) {
	s.metrics = metrics
}

// ApplyMigrations применяет миграции базы данных
func ApplyMigrations(db *sql.DB) error {
	ddl := `-- 0001 init
CREATE TABLE IF NOT EXISTS teams (
  team_name TEXT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS users (
  user_id TEXT PRIMARY KEY,
  username TEXT,
  is_active BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE IF NOT EXISTS team_members (
  team_name TEXT REFERENCES teams(team_name) ON DELETE CASCADE,
  user_id TEXT REFERENCES users(user_id) ON DELETE CASCADE,
  PRIMARY KEY (team_name,user_id)
);

CREATE TABLE IF NOT EXISTS pull_requests (
  pull_request_id TEXT PRIMARY KEY,
  pull_request_name TEXT,
  author_id TEXT REFERENCES users(user_id),
  status TEXT NOT NULL DEFAULT 'OPEN',
  merged_at TIMESTAMP WITH TIME ZONE NULL
);

CREATE TABLE IF NOT EXISTS pr_reviewers (
  pull_request_id TEXT REFERENCES pull_requests(pull_request_id) ON DELETE CASCADE,
  user_id TEXT REFERENCES users(user_id) ON DELETE CASCADE,
  PRIMARY KEY (pull_request_id,user_id)
);

CREATE INDEX IF NOT EXISTS idx_team_members_team ON team_members(team_name);
CREATE INDEX IF NOT EXISTS idx_users_active ON users(is_active);
`
	_, err := db.Exec(ddl)
	return err
}

// Обертки для методов БД с метриками
func (s *StorageData) execWithMetrics(ctx context.Context, operation, table string, query string, args ...interface{}) (sql.Result, error) {
	start := time.Now()
	result, err := s.db.ExecContext(ctx, query, args...)

	if s.metrics != nil {
		s.metrics.ObserveDBQuery(operation, table, time.Since(start))
	}

	return result, err
}

func (s *StorageData) queryWithMetrics(ctx context.Context, operation, table string, query string, args ...interface{}) (*sql.Rows, error) {
	start := time.Now()
	rows, err := s.db.QueryContext(ctx, query, args...)

	if s.metrics != nil {
		s.metrics.ObserveDBQuery(operation, table, time.Since(start))
	}

	return rows, err
}

func (s *StorageData) queryRowWithMetrics(ctx context.Context, operation, table string, query string, args ...interface{}) *sql.Row {
	start := time.Now()

	if s.metrics != nil {
		defer func() {
			s.metrics.ObserveDBQuery(operation, table, time.Since(start))
		}()
	}

	return s.db.QueryRowContext(ctx, query, args...)
}

// Обертки для транзакций с метриками
func (s *StorageData) txExecWithMetrics(tx *sql.Tx, ctx context.Context, operation, table string, query string, args ...interface{}) (sql.Result, error) {
	start := time.Now()
	result, err := tx.ExecContext(ctx, query, args...)

	if s.metrics != nil {
		s.metrics.ObserveDBQuery(operation, table, time.Since(start))
	}

	return result, err
}

func (s *StorageData) txQueryWithMetrics(tx *sql.Tx, ctx context.Context, operation, table string, query string, args ...interface{}) (*sql.Rows, error) {
	start := time.Now()
	rows, err := tx.QueryContext(ctx, query, args...)

	if s.metrics != nil {
		s.metrics.ObserveDBQuery(operation, table, time.Since(start))
	}

	return rows, err
}

func (s *StorageData) txQueryRowWithMetrics(tx *sql.Tx, ctx context.Context, operation, table string, query string, args ...interface{}) *sql.Row {
	start := time.Now()

	if s.metrics != nil {
		defer func() {
			s.metrics.ObserveDBQuery(operation, table, time.Since(start))
		}()
	}

	return tx.QueryRowContext(ctx, query, args...)
}

// Обновленные методы с метриками

func (s *StorageData) UpsertTeam(ctx context.Context, t models.Team) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Если команда новая - создаем, иначе игнорируем
	if _, err := s.txExecWithMetrics(tx, ctx, "insert", "teams",
		`INSERT INTO teams(team_name) VALUES($1) ON CONFLICT (team_name) DO NOTHING`, t.TeamName); err != nil {
		return err
	}

	// Upsert users and members:
	for _, u := range t.Members {
		// Создает/обновляет пользователя
		if _, err := s.txExecWithMetrics(tx, ctx, "upsert", "users",
			`INSERT INTO users(user_id, username, is_active) VALUES($1,$2,$3) ON CONFLICT (user_id) DO UPDATE SET username=EXCLUDED.username`,
			u.UserID, u.Username, u.IsActive); err != nil {
			return err
		}
		// Добавляет в команду (если не состоит)
		if _, err := s.txExecWithMetrics(tx, ctx, "insert", "team_members",
			`INSERT INTO team_members(team_name,user_id) VALUES($1,$2) ON CONFLICT DO NOTHING`,
			t.TeamName, u.UserID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *StorageData) SetUserActive(ctx context.Context, userID string, active bool) error {
	_, err := s.execWithMetrics(ctx, "update", "users",
		`UPDATE users SET is_active=$1 WHERE user_id=$2`, active, userID)
	return err
}

func (s *StorageData) CreatePR(ctx context.Context, pr models.CreatePRRequest) (*models.PullRequest, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Проверяем существование автора
	var authorExists bool
	err = s.txQueryRowWithMetrics(tx, ctx, "select", "users",
		`SELECT EXISTS(SELECT 1 FROM users WHERE user_id = $1)`, pr.AuthorID).Scan(&authorExists)
	if err != nil {
		return nil, err
	}
	if !authorExists {
		return nil, fmt.Errorf("author not found")
	}

	// Проверяем что автор состоит хотя бы в одной команде
	var teamName string
	err = s.txQueryRowWithMetrics(tx, ctx, "select", "team_members",
		`SELECT team_name FROM team_members WHERE user_id = $1 LIMIT 1`, pr.AuthorID).Scan(&teamName)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("author is not in any team")
		}
		return nil, err
	}

	// Проверяем существование PR
	var prExists bool
	err = s.txQueryRowWithMetrics(tx, ctx, "select", "pull_requests",
		`SELECT EXISTS(SELECT 1 FROM pull_requests WHERE pull_request_id = $1)`, pr.PullRequestID).Scan(&prExists)
	if err != nil {
		return nil, err
	}
	if prExists {
		return nil, fmt.Errorf("pr already exists")
	}

	// Создаем PR
	if _, err := s.txExecWithMetrics(tx, ctx, "insert", "pull_requests",
		`INSERT INTO pull_requests(pull_request_id, pull_request_name, author_id, status) VALUES($1,$2,$3,'OPEN')`,
		pr.PullRequestID, pr.PullRequestName, pr.AuthorID); err != nil {
		return nil, err
	}

	// Собираем активных кандидатов исключая автора
	rows, err := s.txQueryWithMetrics(tx, ctx, "select", "users",
		`SELECT u.user_id 
        FROM users u 
        JOIN team_members tm ON u.user_id = tm.user_id 
        WHERE tm.team_name = $1 AND u.is_active = true AND u.user_id <> $2`,
		teamName, pr.AuthorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		candidates = append(candidates, uid)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Выбираем до 2 случайных ревьюеров
	selected := pickRandomDistinct(candidates, 2)
	var reviewers []string

	for _, r := range selected {
		if _, err := s.txExecWithMetrics(tx, ctx, "insert", "pr_reviewers",
			`INSERT INTO pr_reviewers(pull_request_id, user_id) VALUES($1,$2)`,
			pr.PullRequestID, r); err != nil {
			return nil, err
		}
		reviewers = append(reviewers, r)
	}

	// Коммитим транзакцию
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Возвращаем созданный PR
	createdPR := &models.PullRequest{
		PullRequestID:   pr.PullRequestID,
		PullRequestName: pr.PullRequestName,
		AuthorID:        pr.AuthorID,
		Status:          "OPEN",
		Reviewers:       reviewers,
	}

	return createdPR, nil
}

func (s *StorageData) MergePR(ctx context.Context, prID string) (*models.PullRequest, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Получаем текущий PR с блокировкой
	var pr models.PullRequest
	err = s.txQueryRowWithMetrics(tx, ctx, "select", "pull_requests",
		`SELECT pull_request_id, pull_request_name, author_id, status 
         FROM pull_requests WHERE pull_request_id = $1 FOR UPDATE`,
		prID).Scan(&pr.PullRequestID, &pr.PullRequestName, &pr.AuthorID, &pr.Status)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("pr not found")
		}
		return nil, err
	}

	// Если уже мерджен - возвращаем текущее состояние
	if pr.Status == "MERGED" {
		// Получаем ревьюеров для ответа
		reviewers, err := s.getReviewersForPR(ctx, tx, prID)
		if err != nil {
			return nil, err
		}
		pr.Reviewers = reviewers
		return &pr, tx.Commit()
	}

	// Обновляем статус на MERGED и устанавливаем время мерджа
	_, err = s.txExecWithMetrics(tx, ctx, "update", "pull_requests",
		`UPDATE pull_requests SET status = 'MERGED', merged_at = CURRENT_TIMESTAMP 
         WHERE pull_request_id = $1`,
		prID)
	if err != nil {
		return nil, err
	}

	// Получаем ревьюеров
	reviewers, err := s.getReviewersForPR(ctx, tx, prID)
	if err != nil {
		return nil, err
	}
	pr.Reviewers = reviewers
	pr.Status = "MERGED"

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &pr, nil
}

// Вспомогательная функция для получения ревьюеров PR
func (s *StorageData) getReviewersForPR(ctx context.Context, tx *sql.Tx, prID string) ([]string, error) {
	rows, err := s.txQueryWithMetrics(tx, ctx, "select", "pr_reviewers",
		`SELECT user_id FROM pr_reviewers WHERE pull_request_id = $1`,
		prID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reviewers []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		reviewers = append(reviewers, userID)
	}
	return reviewers, rows.Err()
}

// Заменяет одного ревьюера на другого случайного активного пользователя из той же команды.
func (s *StorageData) ReassignReviewer(ctx context.Context, prID string, oldReviewerID string) (*models.PullRequest, string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback()

	// Получаем информацию о PR с блокировкой
	var pr models.PullRequest
	var authorID string
	err = s.txQueryRowWithMetrics(tx, ctx, "select", "pull_requests",
		`SELECT pull_request_id, pull_request_name, author_id, status 
         FROM pull_requests WHERE pull_request_id = $1 FOR UPDATE`,
		prID).Scan(&pr.PullRequestID, &pr.PullRequestName, &authorID, &pr.Status)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, "", fmt.Errorf("pr not found")
		}
		return nil, "", err
	}

	// Проверяем что PR не мерджен
	if pr.Status == "MERGED" {
		return nil, "", fmt.Errorf("cannot modify reviewers after merge")
	}

	// СНАЧАЛА проверяем существование пользователя
	var userExists bool
	err = s.txQueryRowWithMetrics(tx, ctx, "select", "users",
		`SELECT EXISTS(SELECT 1 FROM users WHERE user_id = $1)`,
		oldReviewerID).Scan(&userExists)
	if err != nil {
		return nil, "", err
	}
	if !userExists {
		return nil, "", fmt.Errorf("old reviewer not in any team")
	}

	// ПОТОМ проверяем что старый ревьюер действительно назначен на этот PR
	var isAssigned bool
	err = s.txQueryRowWithMetrics(tx, ctx, "select", "pr_reviewers",
		`SELECT EXISTS(SELECT 1 FROM pr_reviewers WHERE pull_request_id = $1 AND user_id = $2)`,
		prID, oldReviewerID).Scan(&isAssigned)
	if err != nil {
		return nil, "", err
	}
	if !isAssigned {
		return nil, "", fmt.Errorf("reviewer is not assigned to this PR")
	}

	// Находим команду старого ревьюера
	var teamName string
	err = s.txQueryRowWithMetrics(tx, ctx, "select", "team_members",
		`SELECT team_name FROM team_members WHERE user_id = $1 LIMIT 1`,
		oldReviewerID).Scan(&teamName)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, "", fmt.Errorf("old reviewer not in any team")
		}
		return nil, "", err
	}

	// Ищем кандидатов для замены
	rows, err := s.txQueryWithMetrics(tx, ctx, "select", "users", `
        SELECT u.user_id 
        FROM users u
        JOIN team_members tm ON u.user_id = tm.user_id
        LEFT JOIN pr_reviewers pr ON u.user_id = pr.user_id AND pr.pull_request_id = $1
        WHERE tm.team_name = $2 
          AND u.is_active = true 
          AND u.user_id <> $3
          AND pr.user_id IS NULL`,
		prID, teamName, authorID)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var candidates []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, "", err
		}
		candidates = append(candidates, uid)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	// Удаляем старого ревьюера
	_, err = s.txExecWithMetrics(tx, ctx, "delete", "pr_reviewers",
		`DELETE FROM pr_reviewers WHERE pull_request_id = $1 AND user_id = $2`,
		prID, oldReviewerID)
	if err != nil {
		return nil, "", err
	}

	var replacedBy string

	// Выбираем нового ревьюера если есть кандидаты
	if len(candidates) > 0 {
		selected := pickRandomDistinct(candidates, 1)
		newID := selected[0]

		_, err = s.txExecWithMetrics(tx, ctx, "insert", "pr_reviewers",
			`INSERT INTO pr_reviewers(pull_request_id, user_id) VALUES($1, $2)`,
			prID, newID)
		if err != nil {
			return nil, "", err
		}
		replacedBy = newID
	} else {
		// Нет доступных кандидатов
		replacedBy = ""
	}

	// Получаем обновленный список ревьюеров
	reviewers, err := s.getReviewersForPR(ctx, tx, prID)
	if err != nil {
		return nil, "", err
	}
	pr.Reviewers = reviewers
	pr.AuthorID = authorID

	if err := tx.Commit(); err != nil {
		return nil, "", err
	}

	return &pr, replacedBy, nil
}

// Get PRs where user is reviewer
func (s *StorageData) GetPRsForUser(ctx context.Context, userID string) ([]models.PullRequest, error) {
	rows, err := s.queryWithMetrics(ctx, "select", "pull_requests",
		`SELECT pr.pull_request_id, pr.pull_request_name, pr.author_id, pr.status
        FROM pull_requests pr
        JOIN pr_reviewers r ON pr.pull_request_id = r.pull_request_id
        WHERE r.user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []models.PullRequest
	for rows.Next() {
		var pr models.PullRequest
		if err := rows.Scan(&pr.PullRequestID, &pr.PullRequestName, &pr.AuthorID, &pr.Status); err != nil {
			return nil, err
		}
		// fetch reviewers
		rrows, err := s.queryWithMetrics(ctx, "select", "pr_reviewers",
			`SELECT user_id FROM pr_reviewers WHERE pull_request_id=$1`, pr.PullRequestID)
		if err != nil {
			return nil, err
		}
		var revs []string
		for rrows.Next() {
			var uid string
			if err := rrows.Scan(&uid); err != nil {
				rrows.Close()
				return nil, err
			}
			revs = append(revs, uid)
		}
		rrows.Close()
		pr.Reviewers = revs
		res = append(res, pr)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

// GetTeam возвращает команду с участниками (с транзакцией)
func (s *StorageData) GetTeam(ctx context.Context, teamName string) (*models.Team, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Проверяем существование команды
	var exists bool
	err = s.txQueryRowWithMetrics(tx, ctx, "select", "teams",
		"SELECT EXISTS(SELECT 1 FROM teams WHERE team_name = $1)", teamName).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("team not found")
	}

	// Получаем участников команды
	rows, err := s.txQueryWithMetrics(tx, ctx, "select", "users", `
        SELECT u.user_id, u.username, u.is_active 
        FROM users u
        JOIN team_members tm ON u.user_id = tm.user_id
        WHERE tm.team_name = $1
        ORDER BY u.user_id`, teamName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []models.User
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.UserID, &user.Username, &user.IsActive); err != nil {
			return nil, err
		}
		members = append(members, user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	team := &models.Team{
		TeamName: teamName,
		Members:  members,
	}

	return team, nil
}

// HealthCheck проверяет доступность базы данных
func (s *StorageData) HealthCheck(ctx context.Context) error {
	// Создаем контекст с таймаутом для health check
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Проверяем подключение к БД
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Проверяем, что можем выполнить простой запрос
	var result int
	if err := s.queryRowWithMetrics(ctx, "select", "system", "SELECT 1").Scan(&result); err != nil {
		return fmt.Errorf("database query failed: %w", err)
	}

	if result != 1 {
		return fmt.Errorf("database health check failed: unexpected result %d", result)
	}

	return nil
}

// pickRandomDistinct выбирает случайные уникальные элементы из массива
func pickRandomDistinct(arr []string, n int) []string {
	if arr == nil || n <= 0 {
		return []string{}
	}

	if len(arr) <= n {
		res := make([]string, len(arr))
		copy(res, arr)
		return res
	}

	res := make([]string, len(arr))
	copy(res, arr)
	for i := len(res) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		res[i], res[j] = res[j], res[i]
	}
	return res[:n]
}

// PickForTest экспортирует функцию для тестов
func PickForTest(arr []string, n int) []string {
	return pickRandomDistinct(arr, n)
}

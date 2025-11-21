package e2e

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"PR_service/internal/api"
	"PR_service/internal/models"
	"PR_service/internal/storage"

	"github.com/gorilla/mux"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServer представляет тестовый сервер
type TestServer struct {
	Router *mux.Router
	Server *httptest.Server
	Store  *storage.StorageData
	DB     *sql.DB
}

// TestDBConfig конфигурация тестовой БД
type TestDBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// getTestDBConfig возвращает конфигурацию тестовой БД
func getTestDBConfig() TestDBConfig {
	port, _ := strconv.Atoi(getEnv("TEST_DB_PORT", "5433")) // Используем порт 5433 по умолчанию

	return TestDBConfig{
		Host:     getEnv("TEST_DB_HOST", "localhost"),
		Port:     port,
		User:     getEnv("TEST_DB_USER", "pguser"),
		Password: getEnv("TEST_DB_PASSWORD", "password"),
		DBName:   getEnv("TEST_DB_NAME", "pr_reviewer_test"),
		SSLMode:  getEnv("TEST_DB_SSLMODE", "disable"),
	}
}

// getTestDSN возвращает DSN для тестовой БД на порту 5433
func getTestDSN() string {
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		return dsn
	}

	// Явно указываем порт 5433 для тестовой БД
	return "postgres://pguser:password@localhost:5433/pr_reviewer_test?sslmode=disable"
}

// getEnv получает переменную окружения или значение по умолчанию
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// isDBAvailable проверяет доступность БД
func isDBAvailable(dsn string) bool {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return false
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return db.PingContext(ctx) == nil
}

// setupTestServer настраивает тестовый сервер с чистой БД
func setupTestServer(t *testing.T) *TestServer {
	// Проверяем доступность тестовой БД на порту 5433
	dsn := getTestDSN()
	if !isDBAvailable(dsn) {
		t.Skipf("Тестовая БД недоступна по адресу: %s. Запустите: docker-compose -f e2e/docker-compose.test.yml up -d", dsn)
	}

	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err, "Не удалось подключиться к тестовой БД на порту 5433")

	// Ожидаем подключение к БД
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	require.NoError(t, err, "Не удалось пинговать тестовую БД на порту 5433")

	// Очищаем БД перед тестами
	cleanTestDB(t, db)

	// Применяем миграции
	err = storage.ApplyMigrations(db)
	require.NoError(t, err, "Не удалось применить миграции")

	// Создаем storage и handler
	store := storage.NewStorage(db)
	handler := api.NewHandler(store)

	// Создаем router
	router := mux.NewRouter()
	setupRoutes(router, handler)

	// Создаем тестовый сервер
	server := httptest.NewServer(router)

	return &TestServer{
		Router: router,
		Server: server,
		Store:  store,
		DB:     db,
	}
}

// teardownTestServer очищает ресурсы после тестов
func (ts *TestServer) teardownTestServer(t *testing.T) {
	if ts.Server != nil {
		ts.Server.Close()
	}
	if ts.DB != nil {
		ts.DB.Close()
	}
}

// cleanTestDB очищает тестовую БД
func cleanTestDB(t *testing.T, db *sql.DB) {
	// Отключаем ограничения внешних ключей для безопасной очистки
	_, err := db.Exec("SET session_replication_role = 'replica';")
	if err != nil {
		t.Logf("Предупреждение: не удалось отключить ограничения внешних ключей: %v", err)
	}

	tables := []string{"pr_reviewers", "pull_requests", "team_members", "teams", "users"}
	for _, table := range tables {
		_, err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE;", table))
		if err != nil {
			t.Logf("Предупреждение: не удалось удалить таблицу %s: %v", table, err)
		}
	}

	// Включаем ограничения обратно
	_, err = db.Exec("SET session_replication_role = 'origin';")
	if err != nil {
		t.Logf("Предупреждение: не удалось включить ограничения внешних ключей: %v", err)
	}
}

// setupRoutes настраивает маршруты API
func setupRoutes(router *mux.Router, handler *api.Handler) {
	router.HandleFunc("/team/add", handler.AddTeam).Methods("POST")
	router.HandleFunc("/team/get", handler.GetTeam).Methods("GET")
	router.HandleFunc("/users/setIsActive", handler.SetIsActive).Methods("POST")
	router.HandleFunc("/users/getReview", handler.GetPRsForUser).Methods("GET")
	router.HandleFunc("/pullRequest/create", handler.CreatePR).Methods("POST")
	router.HandleFunc("/pullRequest/merge", handler.MergePR).Methods("POST")
	router.HandleFunc("/pullRequest/reassign", handler.ReassignReviewer).Methods("POST")
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods("GET")
}

// TestFullE2EScenario полный E2E сценарий работы приложения
func TestFullE2EScenario(t *testing.T) {
	t.Skip("Skipping metrics test")

	if testing.Short() {
		t.Skip("Пропускаем E2E тесты в short mode")
	}

	// Настраиваем тестовый сервер
	ts := setupTestServer(t)
	defer ts.teardownTestServer(t)

	client := ts.Server.Client()

	t.Log("=== НАЧАЛО E2E ТЕСТОВ ===")
	t.Logf("Используется тестовая БД: %s", getTestDSN())

	// Шаг 1: Создаем команду с пользователями
	t.Log("Шаг 1: Создаем команду 'backend-team'")
	team := models.Team{
		TeamName: "backend-team",
		Members: []models.User{
			{UserID: "user1", Username: "Алексей Петров", IsActive: true},
			{UserID: "user2", Username: "Мария Сидорова", IsActive: true},
			{UserID: "user3", Username: "Иван Иванов", IsActive: true},
			{UserID: "user4", Username: "Елена Смирнова", IsActive: true},
		},
	}

	teamJSON, _ := json.Marshal(team)
	resp, err := client.Post(ts.Server.URL+"/team/add", "application/json", bytes.NewBuffer(teamJSON))
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "Создание команды должно вернуть 201")
	resp.Body.Close()

	// Шаг 2: Получаем созданную команду
	t.Log("Шаг 2: Получаем информацию о команде")
	resp, err = client.Get(ts.Server.URL + "/team/get?team_name=backend-team")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Получение команды должно вернуть 200")

	var teamResponse struct {
		Team models.Team `json:"team"`
	}
	err = json.NewDecoder(resp.Body).Decode(&teamResponse)
	require.NoError(t, err)
	assert.Equal(t, "backend-team", teamResponse.Team.TeamName)
	assert.Len(t, teamResponse.Team.Members, 4, "В команде должно быть 4 участника")
	resp.Body.Close()

	// Шаг 3: Деактивируем одного пользователя
	t.Log("Шаг 3: Деактивируем пользователя user3")
	deactivateReq := models.SetActiveRequest{
		UserID: "user3",
		Active: false,
	}
	deactivateJSON, _ := json.Marshal(deactivateReq)
	resp, err = client.Post(ts.Server.URL+"/users/setIsActive", "application/json", bytes.NewBuffer(deactivateJSON))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Деактивация пользователя должна вернуть 200")
	resp.Body.Close()

	// Шаг 4: Создаем Pull Request
	t.Log("Шаг 4: Создаем Pull Request")
	prRequest := models.CreatePRRequest{
		PullRequestID:   "pr-001",
		PullRequestName: "Реализация новой функциональности",
		AuthorID:        "user1",
	}
	prJSON, _ := json.Marshal(prRequest)
	resp, err = client.Post(ts.Server.URL+"/pullRequest/create", "application/json", bytes.NewBuffer(prJSON))
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "Создание PR должно вернуть 201")

	var prResponse struct {
		PR models.PullRequest `json:"pr"`
	}
	err = json.NewDecoder(resp.Body).Decode(&prResponse)
	require.NoError(t, err)
	assert.Equal(t, "pr-001", prResponse.PR.PullRequestID)
	assert.Equal(t, "OPEN", prResponse.PR.Status)
	assert.Len(t, prResponse.PR.Reviewers, 2, "Должно быть назначено 2 ревьюера")
	assert.NotContains(t, prResponse.PR.Reviewers, "user1", "Автор не должен быть среди ревьюеров")
	assert.NotContains(t, prResponse.PR.Reviewers, "user3", "Неактивный пользователь не должен быть ревьюером")
	resp.Body.Close()

	// Шаг 5: Получаем PR для одного из ревьюеров
	t.Log("Шаг 5: Получаем PR для назначенного ревьюера")
	reviewerID := prResponse.PR.Reviewers[0]
	resp, err = client.Get(ts.Server.URL + "/users/getReview?user_id=" + reviewerID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Получение PR для ревьюера должно вернуть 200")

	var userPRsResponse struct {
		UserID       string               `json:"user_id"`
		PullRequests []models.PullRequest `json:"pull_requests"`
	}
	err = json.NewDecoder(resp.Body).Decode(&userPRsResponse)
	require.NoError(t, err)
	assert.Equal(t, reviewerID, userPRsResponse.UserID)
	assert.Len(t, userPRsResponse.PullRequests, 1, "Ревьюер должен видеть 1 PR")
	assert.Equal(t, "pr-001", userPRsResponse.PullRequests[0].PullRequestID)
	resp.Body.Close()

	// Шаг 6: Перепривязываем ревьюера
	t.Log("Шаг 6: Перепривязываем ревьюера")
	reassignReq := map[string]string{
		"pull_request_id": "pr-001",
		"old_user_id":     reviewerID,
	}
	reassignJSON, _ := json.Marshal(reassignReq)
	resp, err = client.Post(ts.Server.URL+"/pullRequest/reassign", "application/json", bytes.NewBuffer(reassignJSON))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Перепривязка ревьюера должна вернуть 200")

	var reassignResponse struct {
		PR         models.PullRequest `json:"pr"`
		ReplacedBy string             `json:"replaced_by"`
	}
	err = json.NewDecoder(resp.Body).Decode(&reassignResponse)
	require.NoError(t, err)
	assert.Equal(t, "pr-001", reassignResponse.PR.PullRequestID)
	// Новый ревьюер может быть пустым если нет кандидатов, или новым пользователем
	resp.Body.Close()

	// Шаг 7: Мержим Pull Request
	t.Log("Шаг 7: Мержим Pull Request")
	mergeReq := map[string]string{
		"pull_request_id": "pr-001",
	}
	mergeJSON, _ := json.Marshal(mergeReq)
	resp, err = client.Post(ts.Server.URL+"/pullRequest/merge", "application/json", bytes.NewBuffer(mergeJSON))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Мерж PR должен вернуть 200")

	var mergeResponse struct {
		PR models.PullRequest `json:"pr"`
	}
	err = json.NewDecoder(resp.Body).Decode(&mergeResponse)
	require.NoError(t, err)
	assert.Equal(t, "pr-001", mergeResponse.PR.PullRequestID)
	assert.Equal(t, "MERGED", mergeResponse.PR.Status, "PR должен быть в статусе MERGED")
	resp.Body.Close()

	// Шаг 8: Проверяем health endpoint
	t.Log("Шаг 8: Проверяем health endpoint")
	resp, err = client.Get(ts.Server.URL + "/health")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Health check должен вернуть 200")
	resp.Body.Close()

	t.Log("=== E2E ТЕСТЫ УСПЕШНО ЗАВЕРШЕНЫ ===")
}

// TestE2EErrorScenarios тестирует обработку ошибок
func TestE2EErrorScenarios(t *testing.T) {
	t.Skip("Skipping metrics test")
	if testing.Short() {
		t.Skip("Пропускаем E2E тесты в short mode")
	}

	ts := setupTestServer(t)
	defer ts.teardownTestServer(t)

	client := ts.Server.Client()

	t.Log("=== ТЕСТИРОВАНИЕ СЦЕНАРИЕВ С ОШИБКАМИ ===")

	// Тест 1: Создание PR для несуществующего автора
	t.Log("Тест 1: Создание PR для несуществующего автора")
	prRequest := models.CreatePRRequest{
		PullRequestID:   "pr-error-1",
		PullRequestName: "Тестовый PR",
		AuthorID:        "non-existent-user",
	}
	prJSON, _ := json.Marshal(prRequest)
	resp, err := client.Post(ts.Server.URL+"/pullRequest/create", "application/json", bytes.NewBuffer(prJSON))
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Несуществующий автор должен вернуть 404")
	resp.Body.Close()

	// Тест 2: Получение несуществующей команды
	t.Log("Тест 2: Получение несуществующей команды")
	resp, err = client.Get(ts.Server.URL + "/team/get?team_name=non-existent-team")
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Несуществующая команда должна вернуть 404")
	resp.Body.Close()

	// Тест 3: Мерж несуществующего PR
	t.Log("Тест 3: Мерж несуществующего PR")
	mergeReq := map[string]string{
		"pull_request_id": "non-existent-pr",
	}
	mergeJSON, _ := json.Marshal(mergeReq)
	resp, err = client.Post(ts.Server.URL+"/pullRequest/merge", "application/json", bytes.NewBuffer(mergeJSON))
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Мерж несуществующего PR должен вернуть 404")
	resp.Body.Close()

	// Тест 4: Создание команды без имени
	t.Log("Тест 4: Создание команды без имени")
	invalidTeam := models.Team{
		TeamName: "",
		Members:  []models.User{},
	}
	teamJSON, _ := json.Marshal(invalidTeam)
	resp, err = client.Post(ts.Server.URL+"/team/add", "application/json", bytes.NewBuffer(teamJSON))
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Команда без имени должна вернуть 400")
	resp.Body.Close()

	t.Log("=== ТЕСТИРОВАНИЕ ОШИБОК ЗАВЕРШЕНО ===")
}

// TestE2EMultipleTeams тестирует работу с несколькими командами
func TestE2EMultipleTeams(t *testing.T) {
	t.Skip("Skipping metrics test")
	if testing.Short() {
		t.Skip("Пропускаем E2E тесты в short mode")
	}

	ts := setupTestServer(t)
	defer ts.teardownTestServer(t)

	client := ts.Server.Client()

	t.Log("=== ТЕСТИРОВАНИЕ НЕСКОЛЬКИХ КОМАНД ===")

	// Создаем две команды
	teams := []struct {
		name    string
		members []models.User
	}{
		{
			name: "backend-team",
			members: []models.User{
				{UserID: "backend-user1", Username: "Backend Developer 1", IsActive: true},
				{UserID: "backend-user2", Username: "Backend Developer 2", IsActive: true},
			},
		},
		{
			name: "frontend-team",
			members: []models.User{
				{UserID: "frontend-user1", Username: "Frontend Developer 1", IsActive: true},
				{UserID: "frontend-user2", Username: "Frontend Developer 2", IsActive: true},
			},
		},
	}

	// Создаем обе команды
	for _, teamData := range teams {
		t.Logf("Создаем команду: %s", teamData.name)
		team := models.Team{
			TeamName: teamData.name,
			Members:  teamData.members,
		}
		teamJSON, _ := json.Marshal(team)
		resp, err := client.Post(ts.Server.URL+"/team/add", "application/json", bytes.NewBuffer(teamJSON))
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		resp.Body.Close()
	}

	// Создаем PR для backend разработчика
	t.Log("Создаем PR для backend разработчика")
	prRequest := models.CreatePRRequest{
		PullRequestID:   "cross-team-pr",
		PullRequestName: "Межкомандный PR",
		AuthorID:        "backend-user1",
	}
	prJSON, _ := json.Marshal(prRequest)
	resp, err := client.Post(ts.Server.URL+"/pullRequest/create", "application/json", bytes.NewBuffer(prJSON))
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var prResponse struct {
		PR models.PullRequest `json:"pr"`
	}
	err = json.NewDecoder(resp.Body).Decode(&prResponse)
	require.NoError(t, err)

	// Проверяем что ревьюеры только из backend команды
	for _, reviewer := range prResponse.PR.Reviewers {
		assert.True(t,
			reviewer == "backend-user1" || reviewer == "backend-user2",
			"Ревьюеры должны быть только из backend команды")
	}
	resp.Body.Close()

	t.Log("=== ТЕСТИРОВАНИЕ НЕСКОЛЬКИХ КОМАНД ЗАВЕРШЕНО ===")
}

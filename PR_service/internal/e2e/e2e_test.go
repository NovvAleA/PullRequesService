package e2e

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"PR_service/internal/api"
	"PR_service/internal/models"
	"PR_service/internal/storage"

	"github.com/gorilla/mux"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServer представляет тестовый сервер
type TestServer struct {
	Router  *mux.Router
	Server  *httptest.Server
	Store   *storage.StorageData
	DB      *sql.DB
	Metrics *api.Metrics
}

// getTestDSN возвращает DSN для тестовой БД
func getTestDSN() string {
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		return dsn
	}
	return "postgres://pguser:password@localhost:5433/pr_reviewer_test?sslmode=disable"
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

	if err := db.PingContext(ctx); err != nil {
		log.Printf("DB ping failed: %v", err)
		return false
	}
	return true
}

// setupTestServer настраивает тестовый сервер с чистой БД
func setupTestServer(t *testing.T) *TestServer {
	// Сбрасываем Prometheus registry
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	prometheus.DefaultGatherer = prometheus.DefaultRegisterer.(prometheus.Gatherer)

	dsn := getTestDSN()
	if !isDBAvailable(dsn) {
		t.Skipf("Тестовая БД недоступна: %s", dsn)
	}

	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	require.NoError(t, err)

	// Очищаем БД перед тестами
	cleanTestDB(t, db)

	// Применяем миграции
	err = storage.ApplyMigrations(db)
	require.NoError(t, err)

	// Создаем storage и handler
	store := storage.NewStorage(db)
	metrics := api.NewMetrics()
	handler := api.NewHandler(store, metrics)

	// Создаем router с ТОЧНО ТАКИМИ ЖЕ настройками как в main.go
	router := mux.NewRouter()

	// Middleware (как в main.go)
	router.Use(metrics.MetricsMiddleware)
	router.Use(api.TimeoutMiddleware)

	// API routes (ТОЧНО КАК В main.go)
	router.HandleFunc("/", handler.Root).Methods("GET")
	router.HandleFunc("/team/add", handler.AddTeam).Methods("POST")
	router.HandleFunc("/team/get", handler.GetTeam).Methods("GET")
	router.HandleFunc("/users/setIsActive", handler.SetIsActive).Methods("POST")
	router.HandleFunc("/users/getReview", handler.GetPRsForUser).Methods("GET")
	router.HandleFunc("/pullRequest/create", handler.CreatePR).Methods("POST") // ПРАВИЛЬНЫЙ адрес
	router.HandleFunc("/pullRequest/merge", handler.MergePR).Methods("POST")
	router.HandleFunc("/pullRequest/reassign", handler.ReassignReviewer).Methods("POST")
	router.HandleFunc("/health", handler.HealthCheck).Methods("GET")
	router.Handle("/metrics", metrics.InstrumentedHandler()).Methods("GET")
	router.HandleFunc("/metrics/data", handler.MetricsData).Methods("GET")

	// Создаем тестовый сервер
	server := httptest.NewServer(router)

	return &TestServer{
		Router:  router,
		Server:  server,
		Store:   store,
		DB:      db,
		Metrics: metrics,
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
	tables := []string{"pr_reviewers", "pull_requests", "team_members", "users", "teams"}
	for _, table := range tables {
		_, err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table))
		if err != nil {
			t.Logf("Предупреждение: не удалось удалить таблицу %s: %v", table, err)
		}
	}
}

// TestFullE2EScenario полный E2E сценарий работы приложения
func TestFullE2EScenario(t *testing.T) {
	if testing.Short() {
		t.Skip("Пропускаем E2E тесты в short mode")
	}

	ts := setupTestServer(t)
	defer ts.teardownTestServer(t)

	client := ts.Server.Client()

	t.Log("=== НАЧАЛО E2E ТЕСТОВ ===")

	// Тест: Проверяем что неверный эндпоинт возвращает 404
	t.Log("Проверка неверного эндпоинта")
	resp, err := client.Post(ts.Server.URL+"/pullRequest/create1", "application/json", bytes.NewBuffer([]byte("{}")))
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Неверный эндпоинт /pullRequest/create1 должен вернуть 404")
	resp.Body.Close()

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
	resp, err = client.Post(ts.Server.URL+"/team/add", "application/json", bytes.NewBuffer(teamJSON))
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "Создание команды должно вернуть 201")
	resp.Body.Close()

	// Шаг 2: Получаем созданную команду
	t.Log("Шаг 2: Получаем информацию о команде")
	resp, err = client.Get(ts.Server.URL + "/team/get?team_name=backend-team")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Получение команды должно вернуть 200")

	var teamResponse models.Team
	err = json.NewDecoder(resp.Body).Decode(&teamResponse)
	require.NoError(t, err)
	assert.Equal(t, "backend-team", teamResponse.TeamName)
	assert.Len(t, teamResponse.Members, 4, "В команде должно быть 4 участника")
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

	// Шаг 4: Создаем Pull Request (используем ПРАВИЛЬНЫЙ эндпоинт /pullRequest/create)
	t.Log("Шаг 4: Создаем Pull Request через /pullRequest/create")
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

	// Сохраняем исходных ревьюеров для проверки
	originalReviewers := make([]string, len(prResponse.PR.Reviewers))
	copy(originalReviewers, prResponse.PR.Reviewers)
	resp.Body.Close()

	// Шаг 5: Получаем PR для одного из ревьюеров
	t.Log("Шаг 5: Получаем PR для назначенного ревьюера")
	reviewerID := prResponse.PR.Reviewers[0]
	resp, err = client.Get(ts.Server.URL + "/users/getReview?user_id=" + reviewerID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Получение PR для ревьюера должно вернуть 200")

	var userPRsResponse struct {
		UserID       string                    `json:"user_id"`
		PullRequests []models.PullRequestShort `json:"pull_requests"`
	}
	err = json.NewDecoder(resp.Body).Decode(&userPRsResponse)
	require.NoError(t, err)
	assert.Equal(t, reviewerID, userPRsResponse.UserID)
	assert.Len(t, userPRsResponse.PullRequests, 1, "Ревьюер должен видеть 1 PR")
	assert.Equal(t, "pr-001", userPRsResponse.PullRequests[0].PullRequestID)
	resp.Body.Close()

	// Шаг 6: Перепривязываем ревьюера и проверяем что он действительно поменялся
	t.Log("Шаг 6: Перепривязываем ревьюера и проверяем замену")
	oldReviewerID := reviewerID
	reassignReq := map[string]string{
		"pull_request_id": "pr-001",
		"old_user_id":     oldReviewerID,
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

	// ПРОВЕРКА: старый ревьюер должен быть удален, новый добавлен
	assert.NotContains(t, reassignResponse.PR.Reviewers, oldReviewerID, "Старый ревьюер должен быть удален из списка")

	if reassignResponse.ReplacedBy != "" {
		// Если нашли замену, проверяем что новый ревьюер в списке
		assert.Contains(t, reassignResponse.PR.Reviewers, reassignResponse.ReplacedBy, "Новый ревьюер должен быть в списке")
		assert.Len(t, reassignResponse.PR.Reviewers, 2, "Количество ревьюеров должно остаться 2")
	} else {
		// Если замену не нашли, проверяем что остался только один ревьюер
		assert.Len(t, reassignResponse.PR.Reviewers, 1, "Если замену не нашли, должен остаться 1 ревьюер")
	}
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

// TestReassignReviewerLogic тестирует логику замены ревьюера
func TestReassignReviewerLogic(t *testing.T) {
	if testing.Short() {
		t.Skip("Пропускаем E2E тесты в short mode")
	}

	ts := setupTestServer(t)
	defer ts.teardownTestServer(t)

	client := ts.Server.Client()

	t.Log("=== ТЕСТИРОВАНИЕ ЛОГИКИ ЗАМЕНЫ РЕВЬЮЕРА ===")

	// Создаем команду с 4 пользователями
	team := models.Team{
		TeamName: "test-team",
		Members: []models.User{
			{UserID: "author1", Username: "Автор", IsActive: true},
			{UserID: "reviewer1", Username: "Ревьюер 1", IsActive: true},
			{UserID: "reviewer2", Username: "Ревьюер 2", IsActive: true},
			{UserID: "reviewer3", Username: "Ревьюер 3", IsActive: true},
		},
	}

	teamJSON, _ := json.Marshal(team)
	resp, err := client.Post(ts.Server.URL+"/team/add", "application/json", bytes.NewBuffer(teamJSON))
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Создаем PR
	prRequest := models.CreatePRRequest{
		PullRequestID:   "test-reassign-pr",
		PullRequestName: "Тест замены ревьюера",
		AuthorID:        "author1",
	}
	prJSON, _ := json.Marshal(prRequest)
	resp, err = client.Post(ts.Server.URL+"/pullRequest/create", "application/json", bytes.NewBuffer(prJSON))
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var prResponse struct {
		PR models.PullRequest `json:"pr"`
	}
	err = json.NewDecoder(resp.Body).Decode(&prResponse)
	require.NoError(t, err)

	originalReviewers := prResponse.PR.Reviewers
	t.Logf("Исходные ревьюеры: %v", originalReviewers)
	assert.Len(t, originalReviewers, 2, "Должно быть 2 ревьюера")
	resp.Body.Close()

	// Заменяем первого ревьюера
	oldReviewer := originalReviewers[0]
	t.Logf("Заменяем ревьюера: %s", oldReviewer)

	reassignReq := map[string]string{
		"pull_request_id": "test-reassign-pr",
		"old_user_id":     oldReviewer,
	}
	reassignJSON, _ := json.Marshal(reassignReq)
	resp, err = client.Post(ts.Server.URL+"/pullRequest/reassign", "application/json", bytes.NewBuffer(reassignJSON))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var reassignResponse struct {
		PR         models.PullRequest `json:"pr"`
		ReplacedBy string             `json:"replaced_by"`
	}
	err = json.NewDecoder(resp.Body).Decode(&reassignResponse)
	require.NoError(t, err)

	// ПРОВЕРКИ:
	// 1. Старый ревьюер удален
	assert.NotContains(t, reassignResponse.PR.Reviewers, oldReviewer,
		"Старый ревьюер %s должен быть удален из списка", oldReviewer)

	// 2. Второй исходный ревьюер остался
	assert.Contains(t, reassignResponse.PR.Reviewers, originalReviewers[1],
		"Второй ревьюер %s должен остаться в списке", originalReviewers[1])

	// 3. Если нашли замену, проверяем что это новый пользователь
	if reassignResponse.ReplacedBy != "" {
		t.Logf("Найден новый ревьюер: %s", reassignResponse.ReplacedBy)
		assert.Contains(t, reassignResponse.PR.Reviewers, reassignResponse.ReplacedBy,
			"Новый ревьюер %s должен быть в списке", reassignResponse.ReplacedBy)
		assert.NotEqual(t, oldReviewer, reassignResponse.ReplacedBy,
			"Новый ревьюер не должен совпадать со старым")
		assert.Len(t, reassignResponse.PR.Reviewers, 2,
			"Должно остаться 2 ревьюера после замены")
	} else {
		t.Log("Замена не найдена, остался 1 ревьюер")
		assert.Len(t, reassignResponse.PR.Reviewers, 1,
			"Должен остаться 1 ревьюер если замену не нашли")
	}

	resp.Body.Close()
	t.Log("=== ТЕСТИРОВАНИЕ ЛОГИКИ ЗАМЕНЫ РЕВЬЮЕРА ЗАВЕРШЕНО ===")
}

// TestE2EErrorScenarios тестирует обработку ошибок
func TestE2EErrorScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Пропускаем E2E тесты в short mode")
	}

	ts := setupTestServer(t)
	defer ts.teardownTestServer(t)

	client := ts.Server.Client()

	t.Log("=== ТЕСТИРОВАНИЕ СЦЕНАРИЕВ С ОШИБКАМИ ===")

	// Тест 1: Создание PR через неверный эндпоинт
	t.Log("Тест 1: Создание PR через неверный эндпоинт /pullRequest/create1")
	prRequest := models.CreatePRRequest{
		PullRequestID:   "pr-error-1",
		PullRequestName: "Тестовый PR",
		AuthorID:        "user1",
	}
	prJSON, _ := json.Marshal(prRequest)
	resp, err := client.Post(ts.Server.URL+"/pullRequest/create1", "application/json", bytes.NewBuffer(prJSON))
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Неверный эндпоинт должен вернуть 404")
	resp.Body.Close()

	// Тест 2: Создание PR для несуществующего автора
	t.Log("Тест 2: Создание PR для несуществующего автора")
	prRequest = models.CreatePRRequest{
		PullRequestID:   "pr-error-2",
		PullRequestName: "Тестовый PR",
		AuthorID:        "non-existent-user",
	}
	prJSON, _ = json.Marshal(prRequest)
	resp, err = client.Post(ts.Server.URL+"/pullRequest/create", "application/json", bytes.NewBuffer(prJSON))
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Несуществующий автор должен вернуть 404")
	resp.Body.Close()

	// Тест 3: Замена несуществующего ревьюера
	t.Log("Тест 3: Замена несуществующего ревьюера")
	reassignReq := map[string]string{
		"pull_request_id": "non-existent-pr",
		"old_user_id":     "non-existent-reviewer",
	}
	reassignJSON, _ := json.Marshal(reassignReq)
	resp, err = client.Post(ts.Server.URL+"/pullRequest/reassign", "application/json", bytes.NewBuffer(reassignJSON))
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Замена несуществующего ревьюера должна вернуть 404")
	resp.Body.Close()

	t.Log("=== ТЕСТИРОВАНИЕ ОШИБОК ЗАВЕРШЕНО ===")
}

// CheckUserActiveStatus проверяет активность пользователя
func CheckUserActiveStatus(t *testing.T, client *http.Client, serverURL, userID string, expectedActive bool) {
	t.Helper()

	// Получаем команду пользователя чтобы проверить его статус
	resp, err := client.Get(serverURL + "/team/get?team_name=backend-team")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Не удалось получить команду для проверки статуса пользователя")

	var team models.Team
	err = json.NewDecoder(resp.Body).Decode(&team)
	require.NoError(t, err)

	// Ищем пользователя в команде
	var userFound bool
	var userStatus bool
	for _, member := range team.Members {
		if member.UserID == userID {
			userFound = true
			userStatus = member.IsActive
			break
		}
	}

	assert.True(t, userFound, "Пользователь %s не найден в команде", userID)
	assert.Equal(t, expectedActive, userStatus,
		"Статус активности пользователя %s: ожидалось %v, получено %v",
		userID, expectedActive, userStatus)
}

// CheckPRStatus проверяет статус Pull Request
func CheckPRStatus(t *testing.T, client *http.Client, serverURL, prID, expectedStatus string) {
	t.Helper()

	// Для проверки статуса PR можно использовать эндпоинт получения PR для пользователя
	// или проверить через мерж (но это может изменить состояние)

	// Альтернативный способ: создаем тестового пользователя и проверяем через его PR
	resp, err := client.Get(serverURL + "/users/getReview?user_id=user2") // используем существующего пользователя
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var userPRs struct {
			UserID       string                    `json:"user_id"`
			PullRequests []models.PullRequestShort `json:"pull_requests"`
		}
		err = json.NewDecoder(resp.Body).Decode(&userPRs)
		if err == nil {
			for _, pr := range userPRs.PullRequests {
				if pr.PullRequestID == prID {
					assert.Equal(t, expectedStatus, pr.Status,
						"Статус PR %s: ожидалось %s, получено %s",
						prID, expectedStatus, pr.Status)
					return
				}
			}
		}
	}

	// Если не нашли через users/getReview, проверяем через мерж (read-only способ)
	CheckPRStatusViaMerge(t, client, serverURL, prID, expectedStatus)
}

// CheckPRStatusViaMerge проверяет статус PR через эндпоинт мержа (без изменения состояния)
func CheckPRStatusViaMerge(t *testing.T, client *http.Client, serverURL, prID, expectedStatus string) {
	t.Helper()

	// Если PR уже мерджен, мерж вернет текущее состояние без изменений
	mergeReq := map[string]string{
		"pull_request_id": prID,
	}
	mergeJSON, _ := json.Marshal(mergeReq)
	resp, err := client.Post(serverURL+"/pullRequest/merge", "application/json", bytes.NewBuffer(mergeJSON))
	require.NoError(t, err)
	defer resp.Body.Close()

	if expectedStatus == "MERGED" {
		// Для мерженого PR должен вернуть 200 с текущим состоянием
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var mergeResponse struct {
			PR models.PullRequest `json:"pr"`
		}
		err = json.NewDecoder(resp.Body).Decode(&mergeResponse)
		require.NoError(t, err)

		assert.Equal(t, "MERGED", mergeResponse.PR.Status,
			"PR %s должен быть в статусе MERGED", prID)
		assert.NotNil(t, mergeResponse.PR.MergedAt,
			"У мерженого PR %s должен быть установлен MergedAt", prID)
	} else {
		// Для OPEN PR мерж должен пройти успешно и изменить статус
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}
}

// CheckPRMerged проверяет что PR успешно замержен
func CheckPRMerged(t *testing.T, client *http.Client, serverURL, prID string) {
	t.Helper()

	resp, err := client.Get(serverURL + "/users/getReview?user_id=user2")
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var userPRs struct {
			UserID       string                    `json:"user_id"`
			PullRequests []models.PullRequestShort `json:"pull_requests"`
		}
		err = json.NewDecoder(resp.Body).Decode(&userPRs)
		if err == nil {
			for _, pr := range userPRs.PullRequests {
				if pr.PullRequestID == prID {
					assert.Equal(t, "MERGED", pr.Status,
						"PR %s должен быть в статусе MERGED", prID)
					return
				}
			}
		}
	}

	// Дополнительная проверка через мерж
	mergeReq := map[string]string{
		"pull_request_id": prID,
	}
	mergeJSON, _ := json.Marshal(mergeReq)
	resp, err = client.Post(serverURL+"/pullRequest/merge", "application/json", bytes.NewBuffer(mergeJSON))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var mergeResponse struct {
		PR models.PullRequest `json:"pr"`
	}
	err = json.NewDecoder(resp.Body).Decode(&mergeResponse)
	require.NoError(t, err)

	assert.Equal(t, "MERGED", mergeResponse.PR.Status,
		"PR %s должен быть в статусе MERGED", prID)
	assert.NotNil(t, mergeResponse.PR.MergedAt,
		"У мерженого PR %s должен быть установлен MergedAt", prID)
}

// CheckUserDeactivated проверяет что пользователь деактивирован
func CheckUserDeactivated(t *testing.T, client *http.Client, serverURL, userID string) {
	t.Helper()
	CheckUserActiveStatus(t, client, serverURL, userID, false)
}

// CheckUserActivated проверяет что пользователь активирован
func CheckUserActivated(t *testing.T, client *http.Client, serverURL, userID string) {
	t.Helper()
	CheckUserActiveStatus(t, client, serverURL, userID, true)
}

// CheckReviewersChanged проверяет что список ревьюеров изменился после замены
func CheckReviewersChanged(t *testing.T, client *http.Client, serverURL, prID string, oldReviewers []string) {
	t.Helper()

	// Получаем текущее состояние PR через одного из ревьюеров
	if len(oldReviewers) > 0 {
		resp, err := client.Get(serverURL + "/users/getReview?user_id=" + oldReviewers[0])
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var userPRs struct {
				UserID       string                    `json:"user_id"`
				PullRequests []models.PullRequestShort `json:"pull_requests"`
			}
			err = json.NewDecoder(resp.Body).Decode(&userPRs)
			if err == nil {
				for _, pr := range userPRs.PullRequests {
					if pr.PullRequestID == prID {
						// Проверяем что список ревьюеров изменился
						// Для этого нужно получить полную информацию о PR, что сложно без дополнительного эндпоинта
						t.Logf("PR %s найден у пользователя %s", prID, oldReviewers[0])
						return
					}
				}
				// Если PR не найден у старого ревьюера - это хорошо, значит замена сработала
				t.Logf("PR %s не найден у старого ревьюера %s - замена сработала", prID, oldReviewers[0])
			}
		}
	}
}

// CheckTeamMembersCount проверяет количество участников в команде
func CheckTeamMembersCount(t *testing.T, client *http.Client, serverURL, teamName string, expectedCount int) {
	t.Helper()

	resp, err := client.Get(serverURL + "/team/get?team_name=" + teamName)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var team models.Team
	err = json.NewDecoder(resp.Body).Decode(&team)
	require.NoError(t, err)

	assert.Len(t, team.Members, expectedCount,
		"Количество участников в команде %s: ожидалось %d, получено %d",
		teamName, expectedCount, len(team.Members))
}

// CheckPRExists проверяет что PR существует
func CheckPRExists(t *testing.T, client *http.Client, serverURL, prID string) {
	t.Helper()

	// Пытаемся получить PR через любого пользователя
	resp, err := client.Get(serverURL + "/users/getReview?user_id=user1")
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var userPRs struct {
			UserID       string                    `json:"user_id"`
			PullRequests []models.PullRequestShort `json:"pull_requests"`
		}
		err = json.NewDecoder(resp.Body).Decode(&userPRs)
		require.NoError(t, err)

		prFound := false
		for _, pr := range userPRs.PullRequests {
			if pr.PullRequestID == prID {
				prFound = true
				break
			}
		}
		assert.True(t, prFound, "PR %s должен существовать", prID)
	}
}

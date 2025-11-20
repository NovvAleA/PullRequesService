package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
	"database/sql"
)

const baseURL = "http://localhost:8080"

type TestCase struct {
	Name        string
	Method      string
	URL         string
	Body        interface{}
	ExpectCode  int
	ExpectError bool
	Description string
}

func cleanupDatabase() error {
    dsn := "postgres://pguser:password@localhost:5432/pr_reviewer_db?sslmode=disable"
    db, err := sql.Open("pgx", dsn)
    if err != nil {
        return err
    }
    defer db.Close()

    // Очищаем все таблицы в правильном порядке (из-за foreign keys)
    _, err = db.Exec(`
        DELETE FROM pr_reviewers;
        DELETE FROM pull_requests;
        DELETE FROM team_members;
        DELETE FROM teams;
        DELETE FROM users;
    `)
    return err
}

func main() {
    fmt.Println("=== Тестирование PR Reviewer Service ===")
    fmt.Printf("Базовая URL: %s\n\n", baseURL)

    // Очищаем БД перед тестами
    fmt.Println("🧹 Очистка базы данных...")
    if err := cleanupDatabase(); err != nil {
        log.Printf("⚠️  Не удалось очистить БД: %v", err)
    }

    // Ждем пока сервис запустится
    if !waitForService() {
        log.Fatal("Сервис не доступен!")
    }

    // Запускаем основные тесты
    runTests()

    // Запускаем дополнительные тесты
    runAdditionalTests()
}

func waitForService() bool {
	fmt.Println("Ожидание запуска сервиса...")
	for i := 0; i < 10; i++ {
		resp, err := http.Get(baseURL + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			fmt.Println("✅ Сервис доступен")
			return true
		}
		time.Sleep(1 * time.Second)
	}
	fmt.Println("❌ Сервис не доступен")
	return false
}

func runTests() {
	testCases := []TestCase{
		// 1. Health check
		{
			Name:        "Health Check",
			Method:      "GET",
			URL:         "/health",
			ExpectCode:  200,
			Description: "Проверка доступности сервиса",
		},

		// 2. Создание команд
		{
			Name:   "Create Backend Team",
			Method: "POST",
			URL:    "/team/add",
			Body: map[string]interface{}{
				"team_name": "backend",
				"members": []map[string]interface{}{
					{"user_id": "u1", "username": "Alice", "is_active": true},
					{"user_id": "u2", "username": "Bob", "is_active": true},
					{"user_id": "u3", "username": "Charlie", "is_active": true},
					{"user_id": "u4", "username": "David", "is_active": true},
				},
			},
			ExpectCode:  201,
			Description: "Создание команды backend с 4 участниками",
		},

		{
			Name:   "Create Frontend Team",
			Method: "POST",
			URL:    "/team/add",
			Body: map[string]interface{}{
				"team_name": "frontend",
				"members": []map[string]interface{}{
					{"user_id": "u5", "username": "Eve", "is_active": true},
					{"user_id": "u6", "username": "Frank", "is_active": true},
				},
			},
			ExpectCode:  201,
			Description: "Создание команды frontend с 2 участниками",
		},

		// 3. Получение информации о командах
		{
			Name:        "Get Backend Team",
			Method:      "GET",
			URL:         "/team/get?team_name=backend",
			ExpectCode:  200,
			Description: "Получение информации о команде backend",
		},

		{
			Name:        "Get Nonexistent Team",
			Method:      "GET",
			URL:         "/team/get?team_name=nonexistent",
			ExpectCode:  404,
			Description: "Попытка получить несуществующую команду",
		},

		// 4. Управление активностью пользователей
		{
			Name:   "Deactivate User",
			Method: "POST",
			URL:    "/users/setIsActive",
			Body: map[string]interface{}{
				"user_id":  "u2",
				"is_active": false,
			},
			ExpectCode:  200,
			Description: "Деактивация пользователя u2",
		},

		// 5. Создание Pull Requests
		{
			Name:   "Create PR 1",
			Method: "POST",
			URL:    "/pullRequest/create",
			Body: map[string]interface{}{
				"pull_request_id":   "pr-1",
				"pull_request_name": "Add authentication system",
				"author_id":         "u1",
			},
			ExpectCode:  201,
			Description: "Создание PR от пользователя u1 (автоматическое назначение ревьюеров)",
		},

		{
			Name:   "Create PR 2",
			Method: "POST",
			URL:    "/pullRequest/create",
			Body: map[string]interface{}{
				"pull_request_id":   "pr-2",
				"pull_request_name": "Fix database connection",
				"author_id":         "u3",
			},
			ExpectCode:  201,
			Description: "Создание PR от пользователя u3",
		},

		{
			Name:   "Create Duplicate PR",
			Method: "POST",
			URL:    "/pullRequest/create",
			Body: map[string]interface{}{
				"pull_request_id":   "pr-1",
				"pull_request_name": "Duplicate PR",
				"author_id":         "u1",
			},
			ExpectCode:  409,
			Description: "Попытка создать PR с существующим ID (конфликт)",
		},

		// 6. Получение PR пользователей
		{
			Name:        "Get PRs for User u3",
			Method:      "GET",
			URL:         "/users/getReview?user_id=u3",
			ExpectCode:  200,
			Description: "Получение PR где пользователь u3 назначен ревьюером",
		},

		{
			Name:        "Get PRs for User u4",
			Method:      "GET",
			URL:         "/users/getReview?user_id=u4",
			ExpectCode:  200,
			Description: "Получение PR где пользователь u4 назначен ревьюером",
		},

		// 7. Переназначение ревьюеров
		{
			Name:   "Reassign Reviewer in PR-1",
			Method: "POST",
			URL:    "/pullRequest/reassign",
			Body: map[string]interface{}{
				"pull_request_id": "pr-1",
				"old_user_id":     "u3",
			},
			ExpectCode:  200,
			Description: "Переназначение ревьюера u3 в PR-1",
		},

		// 8. Мердж PR
		{
			Name:   "Merge PR-1",
			Method: "POST",
			URL:    "/pullRequest/merge",
			Body: map[string]interface{}{
				"pull_request_id": "pr-1",
			},
			ExpectCode:  200,
			Description: "Мердж PR-1",
		},

		{
			Name:   "Merge PR-1 Again (Idempotent)",
			Method: "POST",
			URL:    "/pullRequest/merge",
			Body: map[string]interface{}{
				"pull_request_id": "pr-1",
			},
			ExpectCode:  200,
			Description: "Повторный мердж PR-1 (проверка идемпотентности)",
		},

		// 9. Попытка изменить мердженый PR
		{
			Name:   "Reassign in Merged PR",
			Method: "POST",
			URL:    "/pullRequest/reassign",
			Body: map[string]interface{}{
				"pull_request_id": "pr-1",
				"old_user_id":     "u4",
			},
			ExpectCode:  409,
			Description: "Попытка переназначения в мердженом PR (должна быть ошибка)",
		},

		// 10. Edge cases
		{
			Name:        "Get PRs for Nonexistent User",
			Method:      "GET",
			URL:         "/users/getReview?user_id=u999",
			ExpectCode:  200,
			Description: "Получение PR для несуществующего пользователя (пустой список)",
		},

		{
			Name:   "Create PR with Nonexistent Author",
			Method: "POST",
			URL:    "/pullRequest/create",
			Body: map[string]interface{}{
				"pull_request_id":   "pr-4",
				"pull_request_name": "Invalid author",
				"author_id":         "u999",
			},
			ExpectCode:  404,
			Description: "Создание PR от несуществующего пользователя",
		},
	}

	// Запускаем все тесты
	passed := 0
	failed := 0

	for _, tc := range testCases {
		fmt.Printf("🧪 Тест: %s\n", tc.Name)
		fmt.Printf("   📝 %s\n", tc.Description)

		success := runTestCase(tc)
		if success {
			passed++
			fmt.Printf("   ✅ УСПЕХ\n\n")
		} else {
			failed++
			fmt.Printf("   ❌ ПРОВАЛ\n\n")
		}

		// Небольшая пауза между запросами
		time.Sleep(100 * time.Millisecond)
	}

	// Итоги
	fmt.Println("=== РЕЗУЛЬТАТЫ ТЕСТИРОВАНИЯ ===")
	fmt.Printf("✅ Успешных: %d\n", passed)
	fmt.Printf("❌ Проваленных: %d\n", failed)
	fmt.Printf("📊 Общее количество: %d\n", passed+failed)

	if failed == 0 {
		fmt.Println("🎉 Все тесты пройдены успешно!")
	} else {
		fmt.Println("💥 Некоторые тесты провалились")
	}
}

func runTestCase(tc TestCase) bool {
	var bodyBytes []byte
	var err error

	// Подготавливаем тело запроса если есть
	if tc.Body != nil {
		bodyBytes, err = json.Marshal(tc.Body)
		if err != nil {
			fmt.Printf("   ❌ Ошибка подготовки тела запроса: %v\n", err)
			return false
		}
	}

	// Создаем запрос
	req, err := http.NewRequest(tc.Method, baseURL+tc.URL, bytes.NewReader(bodyBytes))
	if err != nil {
		fmt.Printf("   ❌ Ошибка создания запроса: %v\n", err)
		return false
	}

	if tc.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Отправляем запрос
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("   ❌ Ошибка отправки запроса: %v\n", err)
		return false
	}
	defer resp.Body.Close()

	// Читаем ответ
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("   ❌ Ошибка чтения ответа: %v\n", err)
		return false
	}

	// Проверяем статус код
	if resp.StatusCode != tc.ExpectCode {
		fmt.Printf("   ❌ Неверный статус код. Ожидался: %d, Получен: %d\n", tc.ExpectCode, resp.StatusCode)
		fmt.Printf("   📄 Ответ: %s\n", string(respBody))
		return false
	}

	// Парсим JSON для красивого вывода
	var prettyJSON bytes.Buffer
	if len(respBody) > 0 {
		if err := json.Indent(&prettyJSON, respBody, "      ", "  "); err == nil {
			fmt.Printf("   📄 Ответ:\n%s\n", prettyJSON.String())
		} else {
			fmt.Printf("   📄 Ответ: %s\n", string(respBody))
		}
	} else {
		fmt.Printf("   📄 Ответ: (пусто)\n")
	}

	return true
}

func runAdditionalTests() {
	fmt.Println("\n=== ДОПОЛНИТЕЛЬНЫЕ ТЕСТЫ ===")

	additionalTestCases := []TestCase{
		// 1. Тест на создание PR без ревьюеров (мало участников в команде)
		{
			Name:   "Create PR with Minimal Team",
			Method: "POST",
			URL:    "/team/add",
			Body: map[string]interface{}{
				"team_name": "minimal-team",
				"members": []map[string]interface{}{
					{"user_id": "u10", "username": "Solo", "is_active": true},
				},
			},
			ExpectCode:  201,
			Description: "Создание команды с 1 участником (только автор)",
		},

		{
			Name:   "Create PR in Minimal Team",
			Method: "POST",
			URL:    "/pullRequest/create",
			Body: map[string]interface{}{
				"pull_request_id":   "pr-minimal",
				"pull_request_name": "Minimal team PR",
				"author_id":         "u10",
			},
			ExpectCode:  201,
			Description: "Создание PR в команде где нет других активных участников (0 ревьюеров)",
		},

		// 2. Тест на переназначение когда нет кандидатов
		{
			Name:   "Reassign with No Candidates",
			Method: "POST",
			URL:    "/pullRequest/reassign",
			Body: map[string]interface{}{
				"pull_request_id": "pr-minimal",
				"old_user_id":     "u999", // несуществующий ревьюер
			},
			ExpectCode:  404,
			Description: "Переназначение несуществующего ревьюера",
		},

		// 3. Тест на массовую деактивацию
		{
			Name:   "Deactivate Multiple Users",
			Method: "POST",
			URL:    "/users/setIsActive",
			Body: map[string]interface{}{
				"user_id":  "u3",
				"is_active": false,
			},
			ExpectCode:  200,
			Description: "Деактивация пользователя u3",
		},

		{
			Name:   "Deactivate User u4",
			Method: "POST",
			URL:    "/users/setIsActive",
			Body: map[string]interface{}{
				"user_id":  "u4",
				"is_active": false,
			},
			ExpectCode:  200,
			Description: "Деактивация пользователя u4",
		},

		// 4. Тест на создание PR когда большинство пользователей неактивны
		{
			Name:   "Create PR with Inactive Team",
			Method: "POST",
			URL:    "/pullRequest/create",
			Body: map[string]interface{}{
				"pull_request_id":   "pr-inactive-team",
				"pull_request_name": "Inactive team PR",
				"author_id":         "u1",
			},
			ExpectCode:  201,
			Description: "Создание PR когда большинство участников команды неактивны",
		},

		// 5. Тест на повторную активацию пользователя
		{
			Name:   "Reactivate User u2",
			Method: "POST",
			URL:    "/users/setIsActive",
			Body: map[string]interface{}{
				"user_id":  "u2",
				"is_active": true,
			},
			ExpectCode:  200,
			Description: "Повторная активация пользователя u2",
		},

		// 6. Тест на создание PR после реактивации
		{
			Name:   "Create PR After Reactivation",
			Method: "POST",
			URL:    "/pullRequest/create",
			Body: map[string]interface{}{
				"pull_request_id":   "pr-reactivated",
				"pull_request_name": "After reactivation PR",
				"author_id":         "u1",
			},
			ExpectCode:  201,
			Description: "Создание PR после реактивации пользователей",
		},

		// 7. Тест на получение PR для пользователя без PR
		{
			Name:        "Get PRs for User Without PRs",
			Method:      "GET",
			URL:         "/users/getReview?user_id=u10",
			ExpectCode:  200,
			Description: "Получение PR для пользователя без назначенных PR (пустой список)",
		},

		// 8. Тест на создание команды с дублирующимися пользователями
		{
			Name:   "Create Team with Duplicate Users",
			Method: "POST",
			URL:    "/team/add",
			Body: map[string]interface{}{
				"team_name": "duplicate-team",
				"members": []map[string]interface{}{
					{"user_id": "u1", "username": "Alice-Updated", "is_active": true},
					{"user_id": "u2", "username": "Bob-Updated", "is_active": false},
				},
			},
			ExpectCode:  201,
			Description: "Создание команды с пользователями которые уже существуют (должно обновить username)",
		},

		// 9. Тест на переназначение на самого себя (edge case)
		{
			Name:   "Reassign to Same User Attempt",
			Method: "POST",
			URL:    "/pullRequest/reassign",
			Body: map[string]interface{}{
				"pull_request_id": "pr-2",
				"old_user_id":     "u4", // если u4 еще ревьюер
			},
			ExpectCode:  200,
			Description: "Переназначение ревьюера (нормальный случай)",
		},

		// 10. Тест на получение команды после обновления пользователей
		{
			Name:        "Get Team After User Updates",
			Method:      "GET",
			URL:         "/team/get?team_name=duplicate-team",
			ExpectCode:  200,
			Description: "Получение команды после обновления информации о пользователях",
		},

		// 11. Тест на создание PR с автором из нескольких команд
		{
			Name:   "Add User to Multiple Teams",
			Method: "POST",
			URL:    "/team/add",
			Body: map[string]interface{}{
				"team_name": "second-team-for-u1",
				"members": []map[string]interface{}{
					{"user_id": "u1", "username": "Alice", "is_active": true},
					{"user_id": "u20", "username": "MultiTeamUser", "is_active": true},
				},
			},
			ExpectCode:  201,
			Description: "Добавление пользователя u1 во вторую команду",
		},

		{
			Name:   "Create PR from MultiTeam User",
			Method: "POST",
			URL:    "/pullRequest/create",
			Body: map[string]interface{}{
				"pull_request_id":   "pr-multiteam",
				"pull_request_name": "Multi-team author PR",
				"author_id":         "u1",
			},
			ExpectCode:  201,
			Description: "Создание PR пользователем который состоит в нескольких командах",
		},

		// 12. Тест на граничные значения
		{
			Name:   "Create PR with Empty Name",
			Method: "POST",
			URL:    "/pullRequest/create",
			Body: map[string]interface{}{
				"pull_request_id":   "pr-empty-name",
				"pull_request_name": "",
				"author_id":         "u1",
			},
			ExpectCode:  400,
			Description: "Создание PR с пустым названием (должна быть ошибка)",
		},
	}

	// Запускаем дополнительные тесты
	passed := 0
	failed := 0

	for _, tc := range additionalTestCases {
		fmt.Printf("🧪 Доп. тест: %s\n", tc.Name)
		fmt.Printf("   📝 %s\n", tc.Description)

		success := runTestCase(tc)
		if success {
			passed++
			fmt.Printf("   ✅ УСПЕХ\n\n")
		} else {
			failed++
			fmt.Printf("   ❌ ПРОВАЛ\n\n")
		}

		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("=== РЕЗУЛЬТАТЫ ДОПОЛНИТЕЛЬНЫХ ТЕСТОВ ===")
	fmt.Printf("✅ Успешных: %d\n", passed)
	fmt.Printf("❌ Проваленных: %d\n", failed)
}

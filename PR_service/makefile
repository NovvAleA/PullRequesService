.PHONY: build run test test-all docker-up docker-down db-up db-down clean

# ========== РАЗРАБОТКА ==========
build:
	@echo "🔨 Сборка приложения..."
	go build -o bin/pr_service main.go

run: build
	@echo "🚀 Запуск приложения..."
	./bin/pr_service

run-docker: docker-up
	@echo "🐳 Приложение запущено в Docker на http://localhost:8080"

# ========== DOCKER (ОСНОВНОЕ ПРИЛОЖЕНИЕ) ==========
docker-up:
	@echo "🐳 Запуск приложения и БД в Docker..."
	docker-compose up -d
	@echo "⏳ Ожидание запуска..."
	@sleep 5
	@echo "✅ Приложение доступно на http://localhost:8080"

docker-down:
	@echo "🛑 Остановка Docker контейнеров..."
	docker-compose down

docker-logs:
	docker-compose logs -f app

# ========== ТЕСТИРОВАНИЕ ==========
test-unit:
	@echo "🧪 Запуск unit тестов..."
	go test ./internal/api ./internal/storage -v

test-e2e: db-up
	@echo "🚀 Запуск E2E тестов..."
	go test ./internal/e2e -v -timeout 5m

test-all: test-unit test-e2e

# ========== ТЕСТОВАЯ БД ==========
db-up:
	@echo "🐘 Запуск тестовой БД..."
	docker-compose -f docker-compose.test.yml up -d
	@echo "⏳ Ожидание готовности БД..."
	@until docker-compose -f docker-compose.test.yml exec -T postgres-test pg_isready -U pguser; do \
		sleep 1; \
	done
	@echo "✅ Тестовая БД готова на localhost:5433"

db-down:
	@echo "🛑 Остановка тестовой БД..."
	docker-compose -f docker-compose.test.yml down

db-logs:
	docker-compose -f docker-compose.test.yml logs -f

# ========== УТИЛИТЫ ==========
clean: docker-down db-down
	@echo "🧹 Очистка..."
	rm -rf bin/
	docker system prune -f

migrate:
	@echo "🗃️ Применение миграций..."
	@go run main.go --migrate || echo "💡 Запустите приложение: make run"

status:
	@echo "📊 Статус сервисов:"
	@echo "Основное приложение:"
	@-docker-compose ps
	@echo ""
	@echo "Тестовая БД:"
	@-docker-compose -f docker-compose.test.yml ps

help:
	@echo "Доступные команды:"
	@echo "  make run          - Запуск приложения локально"
	@echo "  make run-docker   - Запуск в Docker"
	@echo "  make test-unit    - Unit тесты"
	@echo "  make test-e2e     - E2E тесты (требует тестовую БД)"
	@echo "  make db-up        - Поднять тестовую БД"
	@echo "  make db-down      - Остановить тестовую БД"
	@echo "  make docker-up    - Поднять основное приложение + БД"
	@echo "  make docker-down  - Остановить основное приложение"
	@echo "  make status       - Статус всех сервисов"

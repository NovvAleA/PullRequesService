#!/bin/bash

set -e

echo "🔧 Настройка тестового окружения"

# Переменные окружения по умолчанию
export TEST_DB_HOST=${TEST_DB_HOST:-localhost}
export TEST_DB_PORT=${TEST_DB_PORT:-5433}
export TEST_DB_USER=${TEST_DB_USER:-pguser}
export TEST_DB_PASSWORD=${TEST_DB_PASSWORD:-password}
export TEST_DB_NAME=${TEST_DB_NAME:-pr_reviewer_test}

# Проверка зависимостей
echo "📋 Проверка зависимостей..."
command -v docker >/dev/null 2>&1 || { echo "❌ Docker не установлен"; exit 1; }
command -v docker-compose >/dev/null 2>&1 || { echo "❌ Docker Compose не установлен"; exit 1; }

# Запуск тестовой БД
echo "🐘 Запуск тестовой PostgreSQL..."
docker-compose -f e2e/docker-compose.test.yml up -d

# Ожидание готовности БД
echo "⏳ Ожидание готовности БД..."
until docker-compose -f e2e/docker-compose.test.yml exec -T postgres-test pg_isready -U $TEST_DB_USER; do
  sleep 1
done

echo "✅ Тестовое окружение готово"
echo "📊 Конфигурация:"
echo "   Host: $TEST_DB_HOST"
echo "   Port: $TEST_DB_PORT"
echo "   User: $TEST_DB_USER"
echo "   DB: $TEST_DB_NAME"

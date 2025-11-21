# E2E Тесты для PR Review Service

## 🚀 Быстрый старт

```bash
# 1. Настроить тестовую БД
chmod +x e2e/scripts/setup_test_db.sh
./e2e/scripts/setup_test_db.sh

# 2. Запустить тесты
cd PR_service
go test ./e2e/... -v
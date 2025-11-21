#!/bin/bash

# Сборка образа
docker-compose build

# Тегирование (если нужно пушить в registry)
# docker tag pr-service-app:latest yourusername/pr-reviewer-service:latest
# docker push yourusername/pr-reviewer-service:latest

echo "✅ Образ собран. Запустите: docker-compose up"

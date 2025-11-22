FROM golang:1.25-alpine AS builder

WORKDIR /app

# Копируем зависимости
COPY go.mod go.sum ./
RUN go mod download

# Копируем ВСЕ исходные файлы
COPY . .

# Собираем приложение из правильного пути
RUN CGO_ENABLED=0 GOOS=linux go build -o pr_service ./cmd/server/main.go

# Финальный образ
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Копируем бинарник
COPY --from=builder /app/pr_service .

# Экспортируем порт
EXPOSE 8080

# Запускаем приложение
CMD ["./pr_service"]

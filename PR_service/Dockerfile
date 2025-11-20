FROM golang:1.21-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://proxy.golang.org
RUN go mod download
COPY . .
WORKDIR /src/cmd/server
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/pr-reviewer

FROM alpine:3.18
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/pr-reviewer /app/pr-reviewer
WORKDIR /app
EXPOSE 8080
ENTRYPOINT ["/app/pr-reviewer"]


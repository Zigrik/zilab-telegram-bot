FROM golang:1.24-alpine AS builder

WORKDIR /app

# Копирование зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копирование исходников
COPY . .

# Сборка для Linux/AMD64 (стандарт для большинства VPS)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o bot .

# Финальный образ
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

ENV TZ=Europe/Moscow

WORKDIR /app

COPY --from=builder /app/bot .
COPY .env .env

RUN adduser -D -u 1000 appuser && chown -R appuser:appuser /app
USER appuser

ENTRYPOINT ["./bot"]
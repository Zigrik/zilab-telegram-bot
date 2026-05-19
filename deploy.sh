#!/bin/bash

set -e

echo "🚀 Deploying Zilab Telegram Bot on VPS..."

# Проверка .env
if [ ! -f .env ]; then
    echo "❌ .env file not found!"
    echo "Please create .env file from .env.example"
    exit 1
fi

# Остановка старого контейнера
echo "🛑 Stopping old container..."
docker-compose down || true

# Очистка
echo "🧹 Cleaning..."
docker system prune -f

# Сборка и запуск
echo "🔨 Building and starting..."
docker-compose up -d --build

# Проверка статуса
sleep 3
docker-compose ps

echo "📋 Logs:"
docker-compose logs --tail=20

echo "✅ Bot is running!"
echo "📊 Monitor: docker-compose logs -f"
echo "🛑 Stop: docker-compose down"
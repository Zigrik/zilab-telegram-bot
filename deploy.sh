#!/bin/bash

set -e

echo "🚀 Deploying Zilab Telegram Bot..."

if [ ! -f .env ]; then
    echo "❌ .env file not found!"
    echo "Please create .env file from .env.example"
    exit 1
fi

echo "🛑 Stopping old container..."
docker-compose down 2>/dev/null || true

echo "🧹 Cleaning..."
docker system prune -f 2>/dev/null || true

echo "🔨 Building and starting..."
docker-compose up -d --build

sleep 3
docker-compose ps

echo "✅ Bot is running!"
echo "📊 Monitor: docker-compose logs -f"
echo "🛑 Stop: docker-compose down"
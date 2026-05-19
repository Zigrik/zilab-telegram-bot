# zilab-telegram-bot# Zilab Telegram Bot

Telegram бот для приёма обращений с отправкой на email и уведомлениями администраторам.

## Возможности

- Приветствие пользователей
- Ограничение: 1 сообщение в минуту
- Отправка сообщений на email через SMTP
- Уведомления администраторам в Telegram

## Быстрый старт

1. Скопируйте `.env.example` в `.env` и заполните данными
2. Запустите `chmod +x deploy.sh && ./deploy.sh`

## Требования

- Docker
- Docker Compose
- Токен Telegram бота (через @BotFather)

## Конфигурация

Создайте `.env` файл:

```env
TELEGRAM_BOT_TOKEN=ваш_токен
SMTP_HOST=smtp.yandex.ru
SMTP_PORT=465
SMTP_USER=ваша_почта
SMTP_PASSWORD=пароль_приложения
RECIPIENT_EMAILS=email1@gmail.com,email2@gmail.com
ADMIN_TELEGRAM_IDS=123456789,987654321
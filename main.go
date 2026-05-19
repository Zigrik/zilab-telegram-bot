package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

// BotState управляет ограничением частоты сообщений
type BotState struct {
	mu              sync.RWMutex
	lastMessageTime map[int64]time.Time
}

func NewBotState() *BotState {
	return &BotState{
		lastMessageTime: make(map[int64]time.Time),
	}
}

func (s *BotState) CanSend(userID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	lastTime, exists := s.lastMessageTime[userID]
	if !exists {
		s.lastMessageTime[userID] = time.Now()
		return true
	}

	if time.Since(lastTime) < time.Minute {
		return false
	}

	s.lastMessageTime[userID] = time.Now()
	return true
}

func (s *BotState) Cleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()
			for userID, lastTime := range s.lastMessageTime {
				if now.Sub(lastTime) > time.Hour {
					delete(s.lastMessageTime, userID)
				}
			}
			s.mu.Unlock()
		}
	}
}

type EmailConfig struct {
	Host     string
	Port     string
	User     string
	Password string
}

func SendEmail(config EmailConfig, to []string, subject, body string) error {
	addr := fmt.Sprintf("%s:%s", config.Host, config.Port)
	auth := smtp.PlainAuth("", config.User, config.Password, config.Host)

	msg := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/plain; charset=UTF-8\r\n"+
		"\r\n"+
		"%s\r\n", config.User, strings.Join(to, ","), subject, body)

	err := smtp.SendMail(addr, auth, config.User, to, []byte(msg))
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}
	return nil
}

func SendTelegramMessage(bot *tgbotapi.BotAPI, adminIDs []int64, username, message string) {
	header := fmt.Sprintf("📬 Новое обращение от @%s\n\n", username)
	fullMessage := header + message

	for _, adminID := range adminIDs {
		msg := tgbotapi.NewMessage(adminID, fullMessage)
		if _, err := bot.Send(msg); err != nil {
			slog.Error("Failed to send to admin", "admin_id", adminID, "error", err)
		}
	}
}

func HandleMessage(bot *tgbotapi.BotAPI, state *BotState, emailConfig EmailConfig,
	recipientEmails []string, adminIDs []int64, message *tgbotapi.Message) {

	userID := message.From.ID
	username := message.From.UserName
	if username == "" {
		username = message.From.FirstName
	}

	if !state.CanSend(userID) {
		msg := tgbotapi.NewMessage(userID, "⏳ Пожалуйста, подождите минуту перед отправкой следующего сообщения.")
		bot.Send(msg)
		return
	}

	userMessage := message.Text
	slog.Info("Message received", "from", username, "user_id", userID)

	subject := fmt.Sprintf("Обращение от @%s", username)
	emailBody := fmt.Sprintf("От: @%s (ID: %d)\nВремя: %s\n\nСообщение:\n%s",
		username, userID, time.Now().Format("2006-01-02 15:04:05"), userMessage)

	if err := SendEmail(emailConfig, recipientEmails, subject, emailBody); err != nil {
		slog.Error("Email error", "error", err)
		msg := tgbotapi.NewMessage(userID, "❌ Ошибка отправки. Попробуйте позже.")
		bot.Send(msg)
		return
	}

	slog.Info("Email sent", "to", recipientEmails)

	msg := tgbotapi.NewMessage(userID, "✅ Ваше сообщение отправлено! Мы свяжемся с вами.")
	bot.Send(msg)

	SendTelegramMessage(bot, adminIDs, username, userMessage)
}

func ParseUserIDs(idStrings []string) ([]int64, error) {
	var ids []int64
	for _, idStr := range idStrings {
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			continue
		}
		var id int64
		if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
			return nil, fmt.Errorf("invalid user ID: %s", idStr)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func main() {
	if err := godotenv.Load(); err != nil {
		slog.Warn("No .env file found")
	}

	logHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(logHandler))

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		slog.Error("TELEGRAM_BOT_TOKEN is required")
		os.Exit(1)
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		slog.Error("Failed to create bot", "error", err)
		os.Exit(1)
	}

	bot.Debug = true
	slog.Info("Bot authorized", "username", bot.Self.UserName)

	emailConfig := EmailConfig{
		Host:     os.Getenv("SMTP_HOST"),
		Port:     os.Getenv("SMTP_PORT"),
		User:     os.Getenv("SMTP_USER"),
		Password: os.Getenv("SMTP_PASSWORD"),
	}

	recipientEmails := ParseEmails(strings.Split(os.Getenv("RECIPIENT_EMAILS"), ","))
	if len(recipientEmails) == 0 {
		slog.Error("RECIPIENT_EMAILS is required")
		os.Exit(1)
	}

	adminIDsStr := os.Getenv("ADMIN_TELEGRAM_IDS")
	var adminIDs []int64
	if adminIDsStr != "" {
		adminIDs, _ = ParseUserIDs(strings.Split(adminIDsStr, ","))
	}

	state := NewBotState()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go state.Cleanup(ctx)

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60
	updates := bot.GetUpdatesChan(updateConfig)

	welcomeMessage := "👋 Добро пожаловать в телеграм-бот студии Zilab!\n\n" +
		"Можете оставить свое обращение здесь.\n\n" +
		"📝 Напишите ваше сообщение ниже.\n" +
		"⏱️ Ограничение: 1 сообщение в минуту."

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for update := range updates {
			if update.Message == nil {
				continue
			}

			if update.Message.IsCommand() && update.Message.Command() == "start" {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, welcomeMessage)
				bot.Send(msg)
				continue
			}

			HandleMessage(bot, state, emailConfig, recipientEmails, adminIDs, update.Message)
		}
	}()

	slog.Info("Bot is running on VPS...")
	<-sigChan
	slog.Info("Shutting down...")
	cancel()
	time.Sleep(2 * time.Second)
}

func ParseEmails(emailStrings []string) []string {
	var emails []string
	for _, email := range emailStrings {
		email = strings.TrimSpace(email)
		if email != "" {
			emails = append(emails, email)
		}
	}
	return emails
}

package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port                       int
	DatabaseURL                string
	TelegramToken              string
	SMTPHost                   string
	SMTPPort                   int
	SMTPUser                   string
	SMTPPass                   string
	SMTPFrom                   string
	LemonSqueezyWebhookSecret string
	BaseURL                    string
}

func Load() (*Config, error) {
	port, _ := strconv.Atoi(getEnv("PORT", "8080"))
	smtpPort, _ := strconv.Atoi(getEnv("SMTP_PORT", "587"))

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return &Config{
		Port:                       port,
		DatabaseURL:                dbURL,
		TelegramToken:              os.Getenv("TELEGRAM_BOT_TOKEN"),
		SMTPHost:                   os.Getenv("SMTP_HOST"),
		SMTPPort:                   smtpPort,
		SMTPUser:                   os.Getenv("SMTP_USER"),
		SMTPPass:                   os.Getenv("SMTP_PASS"),
		SMTPFrom:                   getEnv("SMTP_FROM", "noreply@pingcast.io"),
		LemonSqueezyWebhookSecret: os.Getenv("LEMONSQUEEZY_WEBHOOK_SECRET"),
		BaseURL:                    getEnv("BASE_URL", "http://localhost:8080"),
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

package config

import (
	"fmt"
	"os"
	"strconv"
)

type APIConfig struct {
	Port                       int
	DatabaseURL                string
	RedisURL                   string
	NatsURL                    string
	LemonSqueezyWebhookSecret string
	BaseURL                    string
}

type CheckerConfig struct {
	DatabaseURL string
	RedisURL    string
	NatsURL     string
}

type NotifierConfig struct {
	DatabaseURL   string
	RedisURL      string
	NatsURL       string
	TelegramToken string
	SMTPHost      string
	SMTPPort      int
	SMTPUser      string
	SMTPPass      string
	SMTPFrom      string
}

func LoadAPI() (*APIConfig, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	port, _ := strconv.Atoi(getEnv("PORT", "8080"))

	return &APIConfig{
		Port:                       port,
		DatabaseURL:                dbURL,
		RedisURL:                   getEnv("REDIS_URL", "redis://localhost:6379"),
		NatsURL:                    getEnv("NATS_URL", "nats://localhost:4222"),
		LemonSqueezyWebhookSecret: os.Getenv("LEMONSQUEEZY_WEBHOOK_SECRET"),
		BaseURL:                    getEnv("BASE_URL", "http://localhost:8080"),
	}, nil
}

func LoadChecker() (*CheckerConfig, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return &CheckerConfig{
		DatabaseURL: dbURL,
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:6379"),
		NatsURL:     getEnv("NATS_URL", "nats://localhost:4222"),
	}, nil
}

func LoadNotifier() (*NotifierConfig, error) {
	smtpPort, _ := strconv.Atoi(getEnv("SMTP_PORT", "587"))

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return &NotifierConfig{
		DatabaseURL:   dbURL,
		RedisURL:      getEnv("REDIS_URL", "redis://localhost:6379"),
		NatsURL:       getEnv("NATS_URL", "nats://localhost:4222"),
		TelegramToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		SMTPHost:      os.Getenv("SMTP_HOST"),
		SMTPPort:      smtpPort,
		SMTPUser:      os.Getenv("SMTP_USER"),
		SMTPPass:      os.Getenv("SMTP_PASS"),
		SMTPFrom:      getEnv("SMTP_FROM", "noreply@pingcast.io"),
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

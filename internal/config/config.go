package config

import (
	"fmt"
	"os"
	"strconv"
)

type APIConfig struct {
	Port                       int
	DatabaseURL                string
	MaxDBConns                 int
	RedisURL                   string
	NatsURL                    string
	OTelEndpoint               string
	LemonSqueezyWebhookSecret string
	BaseURL                    string
	EncryptionKey              string
	EncryptionKeyOld           string
}

type CheckerConfig struct {
	DatabaseURL        string
	MaxDBConns         int
	RedisURL           string
	NatsURL            string
	OTelEndpoint       string
	WorkerPoolSize     int
	HostConcurrency    int
	RetentionDays      int
	DefaultTimeoutSecs int
}

type NotifierConfig struct {
	DatabaseURL   string
	MaxDBConns    int
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
	maxDBConns, _ := strconv.Atoi(getEnv("MAX_DB_CONNS", "10"))

	return &APIConfig{
		Port:                       port,
		DatabaseURL:                dbURL,
		MaxDBConns:                 maxDBConns,
		RedisURL:                   getEnv("REDIS_URL", "redis://localhost:6379"),
		NatsURL:                    getEnv("NATS_URL", "nats://localhost:4222"),
		OTelEndpoint:               os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		LemonSqueezyWebhookSecret: os.Getenv("LEMONSQUEEZY_WEBHOOK_SECRET"),
		BaseURL:                    getEnv("BASE_URL", "http://localhost:8080"),
		EncryptionKey:              os.Getenv("ENCRYPTION_KEY"),
		EncryptionKeyOld:           os.Getenv("ENCRYPTION_KEY_OLD"),
	}, nil
}

func LoadChecker() (*CheckerConfig, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	maxDBConns, _ := strconv.Atoi(getEnv("MAX_DB_CONNS", "15"))
	workerPoolSize, _ := strconv.Atoi(getEnv("WORKER_POOL_SIZE", "100"))
	hostConcurrency, _ := strconv.Atoi(getEnv("HOST_CONCURRENCY", "3"))
	retentionDays, _ := strconv.Atoi(getEnv("RETENTION_DAYS", "90"))
	defaultTimeout, _ := strconv.Atoi(getEnv("DEFAULT_TIMEOUT_SECS", "10"))

	return &CheckerConfig{
		DatabaseURL:        dbURL,
		MaxDBConns:         maxDBConns,
		RedisURL:           getEnv("REDIS_URL", "redis://localhost:6379"),
		NatsURL:            getEnv("NATS_URL", "nats://localhost:4222"),
		OTelEndpoint:       os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		WorkerPoolSize:     workerPoolSize,
		HostConcurrency:    hostConcurrency,
		RetentionDays:      retentionDays,
		DefaultTimeoutSecs: defaultTimeout,
	}, nil
}

func LoadNotifier() (*NotifierConfig, error) {
	smtpPort, _ := strconv.Atoi(getEnv("SMTP_PORT", "587"))

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	maxDBConns, _ := strconv.Atoi(getEnv("MAX_DB_CONNS", "5"))

	return &NotifierConfig{
		DatabaseURL:   dbURL,
		MaxDBConns:    maxDBConns,
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

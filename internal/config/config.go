package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

// EncryptionConfig is embedded in all service configs.
type EncryptionConfig struct {
	// Format: "1:base64key,2:base64key,3:base64key". Empty = encryption disabled.
	EncryptionKeys           string `env:"ENCRYPTION_KEYS"`
	EncryptionPrimaryVersion string `env:"ENCRYPTION_PRIMARY_VERSION" envDefault:"1"`
}

type APIConfig struct {
	Port                      int    `env:"PORT"                        envDefault:"8080"`
	DatabaseURL               string `env:"DATABASE_URL,required"`
	MaxDBConns                int    `env:"MAX_DB_CONNS"                envDefault:"10"`
	RedisURL                  string `env:"REDIS_URL"                   envDefault:"redis://localhost:6379"`
	NatsURL                   string `env:"NATS_URL"                    envDefault:"nats://localhost:4222"`
	OTelEndpoint              string `env:"OTEL_EXPORTER_OTLP_ENDPOINT"`
	LemonSqueezyWebhookSecret string `env:"LEMONSQUEEZY_WEBHOOK_SECRET"`
	BaseURL                   string `env:"BASE_URL"                    envDefault:"http://localhost:8080"`
	EncryptionConfig
}

type CheckerConfig struct {
	DatabaseURL        string `env:"DATABASE_URL,required"`
	MaxDBConns         int    `env:"MAX_DB_CONNS"            envDefault:"15"`
	RedisURL           string `env:"REDIS_URL"               envDefault:"redis://localhost:6379"`
	NatsURL            string `env:"NATS_URL"                envDefault:"nats://localhost:4222"`
	OTelEndpoint       string `env:"OTEL_EXPORTER_OTLP_ENDPOINT"`
	WorkerPoolSize     int    `env:"WORKER_POOL_SIZE"        envDefault:"100"`
	HostConcurrency    int    `env:"HOST_CONCURRENCY"        envDefault:"3"`
	RetentionDays      int    `env:"RETENTION_DAYS"          envDefault:"90"`
	DefaultTimeoutSecs int    `env:"DEFAULT_TIMEOUT_SECS"    envDefault:"10"`
	EncryptionConfig
}

type NotifierConfig struct {
	DatabaseURL   string `env:"DATABASE_URL,required"`
	MaxDBConns    int    `env:"MAX_DB_CONNS"    envDefault:"5"`
	RedisURL      string `env:"REDIS_URL"       envDefault:"redis://localhost:6379"`
	NatsURL       string `env:"NATS_URL"        envDefault:"nats://localhost:4222"`
	TelegramToken string `env:"TELEGRAM_BOT_TOKEN"`
	SMTPHost      string `env:"SMTP_HOST"`
	SMTPPort      int    `env:"SMTP_PORT"       envDefault:"587"`
	SMTPUser      string `env:"SMTP_USER"`
	SMTPPass      string `env:"SMTP_PASS"`
	SMTPFrom      string `env:"SMTP_FROM"       envDefault:"noreply@pingcast.io"`
	EncryptionConfig
}

func LoadAPI() (*APIConfig, error) {
	cfg := &APIConfig{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse api config: %w", err)
	}
	return cfg, nil
}

func LoadChecker() (*CheckerConfig, error) {
	cfg := &CheckerConfig{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse checker config: %w", err)
	}
	return cfg, nil
}

func LoadNotifier() (*NotifierConfig, error) {
	cfg := &NotifierConfig{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse notifier config: %w", err)
	}
	return cfg, nil
}

package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nats-io/nats.go/jetstream"

	natsadapter "github.com/kirillinakin/pingcast/internal/adapter/nats"
	smtpadapter "github.com/kirillinakin/pingcast/internal/adapter/smtp"
	"github.com/kirillinakin/pingcast/internal/adapter/telegram"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/config"
	"github.com/kirillinakin/pingcast/internal/domain"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.LoadNotifier()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// NATS
	nc, err := natsadapter.Connect(cfg.NatsURL)
	if err != nil {
		slog.Error("failed to connect to nats", "error", err)
		os.Exit(1)
	}
	defer nc.Drain()

	js, err := jetstream.New(nc)
	if err != nil {
		slog.Error("failed to create jetstream context", "error", err)
		os.Exit(1)
	}

	if err := natsadapter.SetupStreams(ctx, js); err != nil {
		slog.Error("failed to setup nats streams", "error", err)
		os.Exit(1)
	}

	// Telegram sender factory
	var tgFactory app.TelegramFactory
	if cfg.TelegramToken != "" {
		tgSender := telegram.New(cfg.TelegramToken)
		tgFactory = tgSender.ForChat
	}

	// SMTP sender factory
	var emailFactory app.EmailFactory
	if cfg.SMTPHost != "" {
		smtpSender := smtpadapter.New(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPFrom)
		emailFactory = smtpSender.ForRecipient
	}

	// Alert service
	alertSvc := app.NewAlertService(tgFactory, emailFactory)

	// Subscribe to alerts
	alertSub := natsadapter.NewAlertSubscriber(js)
	if err := alertSub.Subscribe(ctx, func(ctx context.Context, event *domain.AlertEvent) error {
		return alertSvc.Handle(ctx, event)
	}); err != nil {
		slog.Error("failed to subscribe to alerts", "error", err)
		os.Exit(1)
	}

	slog.Info("notifier started")
	<-ctx.Done()
	slog.Info("notifier shutting down")
	alertSub.Stop()
}

package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/kirillinakin/pingcast/internal/adapter/channel"
	natsadapter "github.com/kirillinakin/pingcast/internal/adapter/nats"
	"github.com/kirillinakin/pingcast/internal/adapter/postgres"
	smtpadapter "github.com/kirillinakin/pingcast/internal/adapter/smtp"
	"github.com/kirillinakin/pingcast/internal/adapter/telegram"
	"github.com/kirillinakin/pingcast/internal/adapter/webhook"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/config"
	"github.com/kirillinakin/pingcast/internal/database"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/observability"
	sqlcgen "github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

func main() {
	inner := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(observability.NewTracingHandler(inner)))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.LoadNotifier()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// PostgreSQL (for channel lookup)
	devMode := os.Getenv("DEV_MODE") == "true"
	slowQueryTracer := observability.NewSlowQueryTracer(100*time.Millisecond, devMode)
	pool, err := database.Connect(ctx, cfg.DatabaseURL, int32(cfg.MaxDBConns), database.WithTracer(slowQueryTracer))
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	queries := sqlcgen.New(pool)
	channelRepo := postgres.NewChannelRepo(pool, queries)
	monitorRepo := postgres.NewMonitorRepo(pool, queries)
	failedAlertRepo := postgres.NewFailedAlertRepo(queries)

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

	// Channel registry (notifier: sending mode, only register with valid credentials)
	channelReg := channel.NewRegistry()
	if cfg.TelegramToken != "" {
		channelReg.Register(domain.ChannelTelegram, "Telegram", telegram.NewFactory(cfg.TelegramToken))
	} else {
		slog.Warn("telegram channel disabled: TELEGRAM_BOT_TOKEN not set")
	}
	if cfg.SMTPHost != "" {
		channelReg.Register(domain.ChannelEmail, "Email", smtpadapter.NewFactory(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPFrom))
	} else {
		slog.Warn("email channel disabled: SMTP_HOST not set")
	}
	channelReg.Register(domain.ChannelWebhook, "Webhook", webhook.NewFactory())
	if len(channelReg.Types()) == 1 {
		slog.Warn("only webhook channel active, telegram and email disabled")
	}

	// Business metrics
	metrics := observability.NewMetrics()

	// Alert service
	alertSvc := app.NewAlertService(channelRepo, monitorRepo, channelReg, failedAlertRepo, metrics)

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

	// Graceful shutdown — drain in-flight alert deliveries
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	done := make(chan struct{})
	go func() {
		alertSub.Stop()
		close(done)
	}()
	select {
	case <-done:
		slog.Info("notifier shutdown complete")
	case <-shutdownCtx.Done():
		slog.Warn("notifier shutdown timed out, force stopping")
	}
}

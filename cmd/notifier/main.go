package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	natsadapter "github.com/kirillinakin/pingcast/internal/adapter/nats"
	"github.com/kirillinakin/pingcast/internal/bootstrap"
	"github.com/kirillinakin/pingcast/internal/config"
	"github.com/kirillinakin/pingcast/internal/database"
	"github.com/kirillinakin/pingcast/internal/observability"
	"github.com/kirillinakin/pingcast/internal/version"
)

func main() {
	inner := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(observability.NewTracingHandler(inner)))

	slog.Info("starting", "service", "notifier", "version", version.Version, "commit", version.Commit)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.LoadNotifier()
	if err != nil {
		slog.Error("config", "error", err)
		os.Exit(1)
	}

	devMode := os.Getenv("DEV_MODE") == "true"
	tracer := observability.NewSlowQueryTracer(100*time.Millisecond, devMode)
	//nolint:gosec // G115
	pool, err := database.Connect(ctx, cfg.DatabaseURL, int32(cfg.MaxDBConns), database.WithTracer(tracer))
	if err != nil {
		slog.Error("db connect", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	nc, err := natsadapter.Connect(cfg.NatsURL)
	if err != nil {
		slog.Error("nats connect", "error", err)
		os.Exit(1)
	}
	defer func() { _ = nc.Drain() }()

	js, err := jetstream.New(nc)
	if err != nil {
		slog.Error("jetstream", "error", err)
		os.Exit(1)
	}
	if err := natsadapter.SetupStreams(ctx, js); err != nil {
		slog.Error("streams", "error", err)
		os.Exit(1)
	}

	cipher, err := bootstrap.InitCipher(cfg.EncryptionConfig)
	if err != nil {
		slog.Error("cipher", "error", err)
		os.Exit(1)
	}

	n, err := bootstrap.NewNotifier(bootstrap.NotifierDeps{
		Pool:          pool,
		NATS:          nc,
		JS:            js,
		Cipher:        cipher,
		TelegramToken: cfg.TelegramToken,
		SMTPHost:      cfg.SMTPHost,
		SMTPPort:      cfg.SMTPPort,
		SMTPUser:      cfg.SMTPUser,
		SMTPPass:      cfg.SMTPPass,
		SMTPFrom:      cfg.SMTPFrom,
	})
	if err != nil {
		slog.Error("compose notifier", "error", err)
		os.Exit(1)
	}

	slog.Info("notifier started")
	<-ctx.Done()
	slog.Info("notifier shutting down")

	n.Stop()
	slog.Info("notifier shutdown complete")
}

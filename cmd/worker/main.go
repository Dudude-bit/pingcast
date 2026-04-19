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

	slog.Info("starting", "service", "worker", "version", version.Version, "commit", version.Commit)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.LoadChecker()
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

	// Worker doesn't need Redis — scheduler owns leader-election locks,
	// API owns rate-limits + sessions. Adding it back would be a drive-by
	// feature. Keep the config field in case a future host-limit lives
	// here, but don't open a connection we don't use.

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
	if streamsErr := natsadapter.SetupStreams(ctx, js); streamsErr != nil {
		slog.Error("streams", "error", streamsErr)
		os.Exit(1)
	}

	cipher, err := bootstrap.InitCipher(cfg.EncryptionConfig)
	if err != nil {
		slog.Error("cipher", "error", err)
		os.Exit(1)
	}

	w, err := bootstrap.NewWorker(bootstrap.WorkerDeps{
		Pool:               pool,
		JS:                 js,
		Cipher:             cipher,
		DefaultTimeoutSecs: cfg.DefaultTimeoutSecs,
	})
	if err != nil {
		slog.Error("compose worker", "error", err)
		os.Exit(1)
	}

	slog.Info("worker started")
	<-ctx.Done()
	slog.Info("worker shutting down")

	w.Stop()
	slog.Info("worker shutdown complete")
}

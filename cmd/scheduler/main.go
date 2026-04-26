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
	redisadapter "github.com/kirillinakin/pingcast/internal/adapter/redis"
	"github.com/kirillinakin/pingcast/internal/bootstrap"
	"github.com/kirillinakin/pingcast/internal/config"
	"github.com/kirillinakin/pingcast/internal/database"
	"github.com/kirillinakin/pingcast/internal/observability"
	"github.com/kirillinakin/pingcast/internal/version"
)

func main() {
	inner := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(observability.NewTracingHandler(inner)))

	slog.Info("starting", "service", "scheduler", "version", version.Version, "commit", version.Commit)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.LoadChecker()
	if err != nil {
		slog.Error("config", "error", err)
		os.Exit(1)
	}

	devMode := os.Getenv("DEV_MODE") == "true"
	tracer := observability.NewSlowQueryTracer(100*time.Millisecond, devMode)
	//nolint:gosec // G115: MaxDBConns far below int32 max
	pool, err := database.Connect(ctx, cfg.DatabaseURL, int32(cfg.MaxDBConns), database.WithTracer(tracer))
	if err != nil {
		slog.Error("db connect", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb, err := redisadapter.Connect(ctx, cfg.RedisURL)
	if err != nil {
		slog.Error("redis connect", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()

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

	sched, err := bootstrap.NewScheduler(bootstrap.SchedulerDeps{
		Pool:           pool,
		Redis:          rdb,
		JS:             js,
		Cipher:         cipher,
		RetentionDays:  cfg.RetentionDays,
		SSLScanEnabled: true,
		CertProvisionerConfig: bootstrap.CertProvisionerConfig{
			CertProvider:     cfg.CertProvider,
			CertACMEEmail:    cfg.CertACMEEmail,
			CertACMEDirURL:   cfg.CertACMEDirURL,
			CertACMEHTTPPort: cfg.CertACMEHTTPPort,
		},
	})
	if err != nil {
		slog.Error("compose scheduler", "error", err)
		os.Exit(1)
	}

	sched.Start(ctx)
	slog.Info("scheduler started")

	<-ctx.Done()
	slog.Info("scheduler shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	sched.Stop(shutdownCtx)
	slog.Info("scheduler shutdown complete")
}

package main

import (
	"context"
	"fmt"
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

	slog.Info("starting", "service", "api", "version", version.Version, "commit", version.Commit)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.LoadAPI()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// OpenTelemetry
	otelShutdown, err := observability.Setup(ctx, "pingcast-api", cfg.OTelEndpoint)
	if err != nil {
		slog.Error("failed to setup otel", "error", err)
		os.Exit(1)
	}
	defer func() { _ = otelShutdown(context.Background()) }()

	// PostgreSQL
	devMode := os.Getenv("DEV_MODE") == "true"
	slowQueryTracer := observability.NewSlowQueryTracer(100*time.Millisecond, devMode)
	//nolint:gosec // G115: MaxDBConns from env config, typical 5-15, far below int32 max
	pool, err := database.Connect(ctx, cfg.DatabaseURL, int32(cfg.MaxDBConns), database.WithTracer(slowQueryTracer))
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if migrateErr := database.Migrate(ctx, pool); migrateErr != nil {
		slog.Error("failed to run migrations", "error", migrateErr)
		os.Exit(1)
	}

	// Redis
	rdb, err := redisadapter.Connect(ctx, cfg.RedisURL)
	if err != nil {
		slog.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()

	// NATS
	nc, err := natsadapter.Connect(cfg.NatsURL)
	if err != nil {
		slog.Error("failed to connect to nats", "error", err)
		os.Exit(1)
	}
	defer func() { _ = nc.Drain() }()

	js, err := jetstream.New(nc)
	if err != nil {
		slog.Error("failed to create jetstream context", "error", err)
		os.Exit(1)
	}
	if streamsErr := natsadapter.SetupStreams(ctx, js); streamsErr != nil {
		slog.Error("failed to setup nats streams", "error", streamsErr)
		os.Exit(1)
	}

	// Encryption (optional — disabled if ENCRYPTION_KEYS not set)
	cipher, err := bootstrap.InitCipher(cfg.EncryptionConfig)
	if err != nil {
		slog.Error("failed to initialize encryption", "error", err)
		os.Exit(1)
	}

	// Compose the API app (wires repos, services, handlers, routes)
	bootApp, err := bootstrap.NewApp(bootstrap.AppDeps{
		Pool:                         pool,
		Redis:                        rdb,
		NATS:                         nc,
		JS:                           js,
		Cipher:                       cipher,
		LemonSqueezySecret:           cfg.LemonSqueezyWebhookSecret,
		LemonSqueezyFounderVariantID: cfg.LemonSqueezyFounderVariantID,
		LemonSqueezyRetailVariantID:  cfg.LemonSqueezyRetailVariantID,
		FounderCap:                   cfg.FounderCap,
	})
	if err != nil {
		slog.Error("failed to compose app", "error", err)
		os.Exit(1)
	}

	// Start
	go func() {
		slog.Info("api started", "port", cfg.Port)
		if err := bootApp.Fiber.Listen(fmt.Sprintf(":%d", cfg.Port)); err != nil {
			slog.Error("server error", "error", err)
		}
	}()

	<-ctx.Done()
	slog.Info("api shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := bootApp.Fiber.ShutdownWithContext(shutdownCtx); err != nil {
		slog.Error("api shutdown error", "error", err)
	}
}

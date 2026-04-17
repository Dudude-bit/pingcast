package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	natsadapter "github.com/kirillinakin/pingcast/internal/adapter/nats"
	"github.com/kirillinakin/pingcast/internal/bootstrap"
	"github.com/kirillinakin/pingcast/internal/adapter/postgres"
	redisadapter "github.com/kirillinakin/pingcast/internal/adapter/redis"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/config"
	"github.com/kirillinakin/pingcast/internal/database"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/observability"
	sqlcgen "github.com/kirillinakin/pingcast/internal/sqlc/gen"
	"github.com/kirillinakin/pingcast/internal/version"

	"github.com/kirillinakin/pingcast/internal/adapter/checker"
)

func main() {
	inner := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(observability.NewTracingHandler(inner)))

	slog.Info("starting", "service", "worker", "version", version.Version, "commit", version.Commit)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.LoadChecker()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

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

	queries := sqlcgen.New(pool)

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

	// Repos
	cipher, err := bootstrap.InitCipher(cfg.EncryptionConfig)
	if err != nil {
		slog.Error("failed to initialize encryption", "error", err)
		os.Exit(1)
	}
	userRepo := postgres.NewUserRepo(queries)
	monitorRepo := postgres.NewMonitorRepo(pool, queries, cipher)
	channelRepo := postgres.NewChannelRepo(pool, queries, cipher)
	checkResultRepo := postgres.NewCheckResultRepo(queries)
	incidentRepo := postgres.NewIncidentRepo(queries)
	uptimeRepo := postgres.NewUptimeRepo(queries)
	txm := postgres.NewTxManager(pool)

	// NATS publisher (for alerts)
	alertPub := natsadapter.NewAlertPublisher(js)

	// Checker registry
	registry := checker.NewRegistry()
	checkTimeout := time.Duration(cfg.DefaultTimeoutSecs) * time.Second
	registry.Register(domain.MonitorHTTP, "HTTP", checker.NewHTTPCheckerWithTimeout(cfg.DefaultTimeoutSecs))
	registry.Register(domain.MonitorTCP, "TCP", checker.NewTCPChecker(checkTimeout))
	registry.Register(domain.MonitorDNS, "DNS", checker.NewDNSChecker())

	// Metrics
	metrics := observability.NewMetrics()

	// App service
	monitoringSvc := app.NewMonitoringService(monitorRepo, channelRepo, checkResultRepo, incidentRepo, userRepo, uptimeRepo, txm, alertPub, nil, registry, metrics)

	// Pull-based NATS consumer — stateless, scales horizontally
	checkSub := natsadapter.NewCheckSubscriber(js)
	if err := checkSub.Subscribe(ctx, func(ctx context.Context, monitorID uuid.UUID) error {
		monitor, err := monitorRepo.GetByID(ctx, monitorID)
		if err != nil {
			return fmt.Errorf("get monitor %s: %w", monitorID, err)
		}
		return monitoringSvc.RunCheck(ctx, monitor)
	}); err != nil {
		slog.Error("failed to subscribe to check tasks", "error", err)
		os.Exit(1)
	}

	slog.Info("worker started")
	<-ctx.Done()
	slog.Info("worker shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	checkSub.Stop()

	select {
	case <-time.After(5 * time.Second):
		slog.Info("worker shutdown complete")
	case <-shutdownCtx.Done():
		slog.Warn("worker shutdown timed out")
	}
}

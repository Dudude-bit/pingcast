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

	"github.com/kirillinakin/pingcast/internal/adapter/channel"
	"github.com/kirillinakin/pingcast/internal/bootstrap"
	"github.com/kirillinakin/pingcast/internal/adapter/checker"
	httpadapter "github.com/kirillinakin/pingcast/internal/adapter/http"
	natsadapter "github.com/kirillinakin/pingcast/internal/adapter/nats"
	"github.com/kirillinakin/pingcast/internal/adapter/postgres"
	redisadapter "github.com/kirillinakin/pingcast/internal/adapter/redis"
	smtpadapter "github.com/kirillinakin/pingcast/internal/adapter/smtp"
	"github.com/kirillinakin/pingcast/internal/adapter/telegram"
	"github.com/kirillinakin/pingcast/internal/adapter/webhook"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/config"
	"github.com/kirillinakin/pingcast/internal/database"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/observability"
	sqlcgen "github.com/kirillinakin/pingcast/internal/sqlc/gen"
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

	// Encryption (optional — disabled if ENCRYPTION_KEYS not set)
	cipher, err := bootstrap.InitCipher(cfg.EncryptionConfig)
	if err != nil {
		slog.Error("failed to initialize encryption", "error", err)
		os.Exit(1)
	}

	// Postgres repos
	userRepo := postgres.NewUserRepo(queries)
	sessionRepo := redisadapter.NewSessionRepo(rdb)
	monitorRepo := postgres.NewMonitorRepo(pool, queries, cipher)
	channelRepo := postgres.NewChannelRepo(pool, queries, cipher)
	checkResultRepo := postgres.NewCheckResultRepo(queries)
	incidentRepo := postgres.NewIncidentRepo(queries)
	uptimeRepo := postgres.NewUptimeRepo(queries)
	txm := postgres.NewTxManager(pool)

	// NATS publishers
	monitorPub := natsadapter.NewMonitorPublisher(js)
	alertPub := natsadapter.NewAlertPublisher(js)

	// Checker registry
	registry := checker.NewRegistry()
	registry.Register(domain.MonitorHTTP, "HTTP", checker.NewHTTPChecker())
	registry.Register(domain.MonitorTCP, "TCP", checker.NewTCPChecker(10*time.Second))
	registry.Register(domain.MonitorDNS, "DNS", checker.NewDNSChecker())

	// API keys
	apiKeyRepo := postgres.NewAPIKeyRepo(queries)

	// Channel registry (API: validation + schema only, no sending credentials needed)
	channelReg := channel.NewRegistry()
	channelReg.Register(domain.ChannelTelegram, "Telegram", telegram.NewFactory(""))
	channelReg.Register(domain.ChannelEmail, "Email", smtpadapter.NewFactory("", 0, "", "", ""))
	channelReg.Register(domain.ChannelWebhook, "Webhook", webhook.NewFactory())
	slog.Info("channel registry initialized (validation-only mode)", "types", len(channelReg.Types()))

	// Failed alerts (DLQ)
	failedAlertRepo := postgres.NewFailedAlertRepo(queries)

	// Business metrics
	metrics := observability.NewMetrics()

	// App services
	authSvc := app.NewAuthService(userRepo, sessionRepo)
	monitoringSvc := app.NewMonitoringService(monitorRepo, channelRepo, checkResultRepo, incidentRepo, userRepo, uptimeRepo, txm, alertPub, monitorPub, registry, metrics)
	alertSvc := app.NewAlertService(channelRepo, monitorRepo, channelReg, failedAlertRepo, metrics)

	// HTTP handlers
	// Shared rate-limit bucket for /api/auth/login and /api/auth/register:
	// 5 attempts per 15 minutes per IP. Keyed by IP for register (Issue 4.7),
	// by email for login.
	rateLimiter := redisadapter.NewRateLimiter(rdb, "auth", 5, 15*time.Minute)
	server := httpadapter.NewServer(authSvc, monitoringSvc, alertSvc, rateLimiter, apiKeyRepo)
	pageHandler := httpadapter.NewPageHandler(authSvc, monitoringSvc, alertSvc, rateLimiter, apiKeyRepo)
	webhookHandler := httpadapter.NewWebhookHandler(authSvc, alertSvc, cfg.LemonSqueezyWebhookSecret)

	// Health checker
	healthChecker := httpadapter.NewHealthChecker(pool, rdb, nc)

	// Wire
	fiberApp := httpadapter.SetupApp(authSvc, pageHandler, server, webhookHandler, apiKeyRepo, healthChecker)

	// Start
	go func() {
		slog.Info("api started", "port", cfg.Port)
		if err := fiberApp.Listen(fmt.Sprintf(":%d", cfg.Port)); err != nil {
			slog.Error("server error", "error", err)
		}
	}()

	<-ctx.Done()
	slog.Info("api shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := fiberApp.ShutdownWithContext(shutdownCtx); err != nil {
		slog.Error("api shutdown error", "error", err)
	}
}

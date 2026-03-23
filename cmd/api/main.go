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
	sqlcgen "github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.LoadAPI()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// PostgreSQL
	pool, err := database.Connect(ctx, cfg.DatabaseURL, int32(cfg.MaxDBConns))
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := database.Migrate(ctx, pool); err != nil {
		slog.Error("failed to run migrations", "error", err)
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

	// Postgres repos
	userRepo := postgres.NewUserRepo(queries)
	sessionRepo := redisadapter.NewSessionRepo(rdb)
	monitorRepo := postgres.NewMonitorRepo(queries)
	checkResultRepo := postgres.NewCheckResultRepo(queries)
	incidentRepo := postgres.NewIncidentRepo(queries)
	uptimeRepo := postgres.NewUptimeRepo(queries)

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
	channelRepo := postgres.NewChannelRepo(queries)
	channelReg := channel.NewRegistry()
	channelReg.Register(domain.ChannelTelegram, "Telegram", telegram.NewFactory(""))
	channelReg.Register(domain.ChannelEmail, "Email", smtpadapter.NewFactory("", 0, "", "", ""))
	channelReg.Register(domain.ChannelWebhook, "Webhook", webhook.NewFactory())
	slog.Info("channel registry initialized (validation-only mode)", "types", len(channelReg.Types()))

	// App services
	authSvc := app.NewAuthService(userRepo, sessionRepo)
	monitoringSvc := app.NewMonitoringService(monitorRepo, checkResultRepo, incidentRepo, userRepo, uptimeRepo, alertPub, registry)
	alertSvc := app.NewAlertService(channelRepo, monitorRepo, channelReg)

	// HTTP handlers
	rateLimiter := redisadapter.NewRateLimiter(rdb, "login", 5, 15*time.Minute)
	server := httpadapter.NewServer(authSvc, monitoringSvc, alertSvc, monitorPub, rateLimiter)
	pageHandler := httpadapter.NewPageHandler(authSvc, monitoringSvc, alertSvc, rateLimiter)
	webhookHandler := httpadapter.NewWebhookHandler(authSvc, alertSvc, cfg.LemonSqueezyWebhookSecret)

	// Wire
	fiberApp := httpadapter.SetupApp(authSvc, pageHandler, server, webhookHandler, apiKeyRepo)

	// Start
	go func() {
		slog.Info("api started", "port", cfg.Port)
		if err := fiberApp.Listen(fmt.Sprintf(":%d", cfg.Port)); err != nil {
			slog.Error("server error", "error", err)
		}
	}()

	<-ctx.Done()
	slog.Info("api shutting down")
	fiberApp.Shutdown()
}

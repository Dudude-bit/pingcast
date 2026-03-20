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

	"github.com/kirillinakin/pingcast/internal/adapter/http"
	natsadapter "github.com/kirillinakin/pingcast/internal/adapter/nats"
	"github.com/kirillinakin/pingcast/internal/adapter/postgres"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/config"
	"github.com/kirillinakin/pingcast/internal/database"
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
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
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
	sessionRepo := postgres.NewSessionRepo(queries)
	monitorRepo := postgres.NewMonitorRepo(queries)
	checkResultRepo := postgres.NewCheckResultRepo(queries)
	incidentRepo := postgres.NewIncidentRepo(queries)

	// NATS publishers
	monitorPub := natsadapter.NewMonitorPublisher(js)
	alertPub := natsadapter.NewAlertPublisher(js)

	// App services
	authSvc := app.NewAuthService(userRepo, sessionRepo)
	monitoringSvc := app.NewMonitoringService(monitorRepo, checkResultRepo, incidentRepo, userRepo, alertPub, nil)

	// HTTP handlers
	rateLimiter := httpadapter.NewRateLimiter(5, 15*time.Minute)
	server := httpadapter.NewServer(authSvc, monitoringSvc, monitorPub, rateLimiter)
	pageHandler := httpadapter.NewPageHandler(authSvc, monitoringSvc, rateLimiter)
	webhookHandler := httpadapter.NewWebhookHandler(authSvc, cfg.LemonSqueezyWebhookSecret)

	// Wire
	fiberApp := httpadapter.SetupApp(authSvc, pageHandler, server, webhookHandler)

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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/kirillinakin/pingcast/internal/auth"
	"github.com/kirillinakin/pingcast/internal/config"
	"github.com/kirillinakin/pingcast/internal/database"
	"github.com/kirillinakin/pingcast/internal/handler"
	natsbus "github.com/kirillinakin/pingcast/internal/nats"
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
	nc, err := natsbus.Connect(cfg.NatsURL)
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

	if err := natsbus.SetupStreams(ctx, js); err != nil {
		slog.Error("failed to setup nats streams", "error", err)
		os.Exit(1)
	}

	// Monitor change callback → publishes to NATS
	onChanged := func(action string, monitorID uuid.UUID, monitor *natsbus.MonitorData) {
		event := natsbus.MonitorChangedEvent{
			Action:    action,
			MonitorID: monitorID,
			Monitor:   monitor,
		}
		data, err := json.Marshal(event)
		if err != nil {
			slog.Error("failed to marshal monitor change event", "error", err)
			return
		}
		if _, err := js.Publish(ctx, "monitors.changed", data); err != nil {
			slog.Error("failed to publish monitor change", "action", action, "monitor_id", monitorID, "error", err)
		}
	}

	// Handlers
	authService := auth.NewService(queries)
	rateLimiter := auth.NewRateLimiter(5, 15*time.Minute)
	pageHandler := handler.NewPageHandler(queries, authService, rateLimiter)
	server := handler.NewServer(queries, authService, rateLimiter, onChanged)
	webhookHandler := handler.NewWebhookHandler(queries, cfg.LemonSqueezyWebhookSecret)

	app := handler.SetupApp(authService, pageHandler, server, webhookHandler)

	// Start
	go func() {
		slog.Info("api started", "port", cfg.Port)
		if err := app.Listen(fmt.Sprintf(":%d", cfg.Port)); err != nil {
			slog.Error("server error", "error", err)
		}
	}()

	<-ctx.Done()
	slog.Info("api shutting down")
	app.Shutdown()
}

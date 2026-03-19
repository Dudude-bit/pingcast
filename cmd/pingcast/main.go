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
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kirillinakin/pingcast/internal/auth"
	"github.com/kirillinakin/pingcast/internal/checker"
	"github.com/kirillinakin/pingcast/internal/config"
	"github.com/kirillinakin/pingcast/internal/database"
	"github.com/kirillinakin/pingcast/internal/handler"
	"github.com/kirillinakin/pingcast/internal/notifier"
	sqlcgen "github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Database
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

	// sqlc queries
	queries := sqlcgen.New(pool)

	// Auth
	authService := auth.NewService(queries)
	rateLimiter := auth.NewRateLimiter(5, 15*time.Minute)

	// Notifier
	var tgSender *notifier.TelegramSender
	if cfg.TelegramToken != "" {
		tgSender = notifier.NewTelegramSender(cfg.TelegramToken)
	}

	var emailSender *notifier.EmailSender
	if cfg.SMTPHost != "" {
		emailSender = notifier.NewEmailSender(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPFrom)
	}

	listener := notifier.NewListener(pool, queries, tgSender, emailSender)
	listener.Start(ctx)

	// Check handler: processes results, manages incidents, publishes PG NOTIFY events
	checkHandler := func(ctx context.Context, monitor *checker.MonitorInfo, result *checker.CheckResult) {
		queries.InsertCheckResult(ctx, sqlcgen.InsertCheckResultParams{
			MonitorID:      monitor.ID,
			Status:         result.Status,
			StatusCode:     pgtype.Int4{Int32: derefInt32(result.StatusCode), Valid: result.StatusCode != nil},
			ResponseTimeMs: int32(result.ResponseTimeMs),
			ErrorMessage:   result.ErrorMessage,
			CheckedAt:      result.CheckedAt,
		})

		// Read current status from DB to avoid race with stale in-memory state
		currentMon, err := queries.GetMonitorByID(ctx, monitor.ID)
		previousStatus := "unknown"
		if err == nil {
			previousStatus = currentMon.CurrentStatus
		}

		queries.UpdateMonitorStatus(ctx, sqlcgen.UpdateMonitorStatusParams{
			ID:            monitor.ID,
			CurrentStatus: result.Status,
		})

		// Status transition handling
		if previousStatus == result.Status {
			return
		}

		if result.Status == "down" {
			failures, _ := queries.ConsecutiveFailures(ctx, monitor.ID)
			if int(failures) >= monitor.AlertAfterFailures {
				inCooldown, _ := queries.IsInCooldown(ctx, monitor.ID)
				if !inCooldown {
					errMsg := ""
					if result.ErrorMessage != nil {
						errMsg = *result.ErrorMessage
					}
					queries.CreateIncident(ctx, sqlcgen.CreateIncidentParams{
						MonitorID: monitor.ID,
						Cause:     errMsg,
					})

					publishEvent(ctx, pool, monitor.ID.String(), "down", errMsg)
				}
			}
		} else if result.Status == "up" && previousStatus == "down" {
			incident, err := queries.GetOpenIncidentByMonitorID(ctx, monitor.ID)
			if err == nil {
				now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
				queries.ResolveIncident(ctx, sqlcgen.ResolveIncidentParams{
					ID:         incident.ID,
					ResolvedAt: now,
				})
			}
			publishEvent(ctx, pool, monitor.ID.String(), "up", "recovered")
		}
	}

	// Checker
	client := checker.NewClient()
	workerPool := checker.NewWorkerPool(ctx, 100, client, checkHandler)

	scheduler := checker.NewScheduler(func(m *checker.MonitorInfo) {
		workerPool.Submit(m)
	})

	// Load existing monitors
	monitors, err := queries.ListActiveMonitors(ctx)
	if err != nil {
		slog.Error("failed to load monitors", "error", err)
	}
	for _, m := range monitors {
		scheduler.Add(monitorToInfo(m))
	}
	slog.Info("loaded monitors", "count", len(monitors))

	// Monitor change callback
	onChanged := func(monitorID uuid.UUID, deleted bool) {
		if deleted {
			scheduler.Remove(monitorID)
			return
		}
		mon, err := queries.GetMonitorByID(ctx, monitorID)
		if err != nil {
			return
		}
		if mon.IsPaused {
			scheduler.Remove(monitorID)
		} else {
			scheduler.Add(monitorToInfo(mon))
		}
	}

	// Handlers
	pageHandler := handler.NewPageHandler(queries, authService, rateLimiter)
	server := handler.NewServer(queries, authService, rateLimiter, onChanged)
	webhookHandler := handler.NewWebhookHandler(queries, cfg.LemonSqueezyWebhookSecret)

	app := handler.SetupApp(authService, pageHandler, server, webhookHandler)

	// Data retention cleanup (daily)
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cutoff := time.Now().Add(-90 * 24 * time.Hour)
				deleted, err := queries.DeleteCheckResultsOlderThan(ctx, cutoff)
				if err != nil {
					slog.Error("retention cleanup failed", "error", err)
				} else if deleted > 0 {
					slog.Info("retention cleanup", "deleted_rows", deleted)
				}

				sessDeleted, err := queries.DeleteExpiredSessions(ctx)
				if err != nil {
					slog.Error("session cleanup failed", "error", err)
				} else if sessDeleted > 0 {
					slog.Info("session cleanup", "deleted", sessDeleted)
				}
			}
		}
	}()

	// Start server
	go func() {
		addr := fmt.Sprintf(":%d", cfg.Port)
		slog.Info("pingcast started", "port", cfg.Port, "monitors", len(monitors))
		if err := app.Listen(addr); err != nil {
			slog.Error("server error", "error", err)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	scheduler.Stop()
	workerPool.Stop()
	app.Shutdown()

	slog.Info("shutdown complete")
}

func monitorToInfo(m sqlcgen.Monitor) *checker.MonitorInfo {
	return &checker.MonitorInfo{
		ID:                 m.ID,
		URL:                m.Url,
		Method:             m.Method,
		IntervalSeconds:    int(m.IntervalSeconds),
		ExpectedStatus:     int(m.ExpectedStatus),
		Keyword:            m.Keyword,
		AlertAfterFailures: int(m.AlertAfterFailures),
	}
}

func derefInt32(p *int32) int32 {
	if p == nil {
		return 0
	}
	return *p
}

func publishEvent(ctx context.Context, pool *pgxpool.Pool, monitorID, event, details string) {
	payload, _ := json.Marshal(map[string]string{
		"monitor_id": monitorID,
		"event":      event,
		"details":    details,
	})
	pool.Exec(ctx, "SELECT pg_notify('monitor_events', $1)", string(payload))
}

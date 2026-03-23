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

	"github.com/go-redsync/redsync/v4"

	"github.com/kirillinakin/pingcast/internal/adapter/checker"
	natsadapter "github.com/kirillinakin/pingcast/internal/adapter/nats"
	"github.com/kirillinakin/pingcast/internal/adapter/postgres"
	redisadapter "github.com/kirillinakin/pingcast/internal/adapter/redis"
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

	cfg, err := config.LoadChecker()
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
	monitorRepo := postgres.NewMonitorRepo(pool, queries)
	checkResultRepo := postgres.NewCheckResultRepo(queries)
	incidentRepo := postgres.NewIncidentRepo(queries)
	uptimeRepo := postgres.NewUptimeRepo(queries)

	// NATS publisher
	alertPub := natsadapter.NewAlertPublisher(js)

	// Checker registry
	registry := checker.NewRegistry()
	checkTimeout := time.Duration(cfg.DefaultTimeoutSecs) * time.Second
	registry.Register(domain.MonitorHTTP, "HTTP", checker.NewHTTPCheckerWithTimeout(cfg.DefaultTimeoutSecs))
	registry.Register(domain.MonitorTCP, "TCP", checker.NewTCPChecker(checkTimeout))
	registry.Register(domain.MonitorDNS, "DNS", checker.NewDNSChecker())

	// App service (registry injected via port)
	monitoringSvc := app.NewMonitoringService(monitorRepo, nil, checkResultRepo, incidentRepo, userRepo, uptimeRepo, nil, alertPub, registry)

	// --- NATS Work Queue Architecture ---
	// Leader-elected scheduler publishes check tasks to NATS.
	// Stateless checker workers consume tasks via pull-based NATS consumer.

	// Redsync for distributed locks (Redlock algorithm)
	rs := redisadapter.NewRedsync(rdb)
	schedulerMutex := rs.NewMutex("lock:scheduler:leader", redsync.WithExpiry(30*time.Second))

	// Check task publisher (scheduler → NATS)
	checkPub := natsadapter.NewCheckPublisher(js)

	// Leader scheduler (only one instance runs at a time)
	leaderScheduler := checker.NewLeaderScheduler(schedulerMutex, checkPub)

	// Load existing monitors into scheduler
	activeMonitors, err := monitorRepo.ListActive(ctx)
	if err != nil {
		slog.Error("failed to load active monitors", "error", err)
		os.Exit(1)
	}
	for i := range activeMonitors {
		leaderScheduler.Add(&activeMonitors[i])
	}
	slog.Info("loaded monitors", "count", len(activeMonitors))

	// Start leader scheduler in background
	go leaderScheduler.Run(ctx)

	// Subscribe to monitor changes via NATS (updates scheduler)
	monitorSub := natsadapter.NewMonitorSubscriber(js)
	if err := monitorSub.Subscribe(ctx, func(ctx context.Context, action domain.MonitorAction, monitorID uuid.UUID, monitor *domain.Monitor) error {
		switch action {
		case domain.ActionCreate, domain.ActionUpdate, domain.ActionResume:
			if monitor != nil {
				leaderScheduler.Add(monitor)
			}
		case domain.ActionDelete, domain.ActionPause:
			leaderScheduler.Remove(monitorID)
		}
		slog.Info("processed monitor change", "action", action, "monitor_id", monitorID)
		return nil
	}); err != nil {
		slog.Error("failed to subscribe to monitor changes", "error", err)
		os.Exit(1)
	}

	// Check task subscriber (NATS → workers, pull-based)
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

	// Data retention cleanup (daily, with distributed lock via redsync)
	cleanupMutex := rs.NewMutex("lock:cleanup:retention", redsync.WithExpiry(1*time.Hour))
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := cleanupMutex.Lock(); err != nil {
					slog.Debug("cleanup lock held by another instance, skipping", "error", err)
					continue
				}

				cutoff := time.Now().Add(-time.Duration(cfg.RetentionDays) * 24 * time.Hour)
				deleted, err := checkResultRepo.DeleteOlderThan(ctx, cutoff)
				if err != nil {
					slog.Error("retention cleanup failed", "error", err)
				} else if deleted > 0 {
					slog.Info("retention cleanup", "deleted_rows", deleted)
				}

				if _, err := cleanupMutex.Unlock(); err != nil {
					slog.Error("failed to release cleanup lock", "error", err)
				}
			}
		}
	}()

	slog.Info("checker started", "monitors", len(activeMonitors))
	<-ctx.Done()
	slog.Info("checker shutting down")

	// Graceful shutdown with 30s timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	monitorSub.Stop()
	leaderScheduler.Stop()
	checkSub.Stop()

	// Allow in-flight check tasks to complete
	select {
	case <-time.After(5 * time.Second):
		slog.Info("checker shutdown complete")
	case <-shutdownCtx.Done():
		slog.Warn("checker shutdown timed out, force stopping")
	}
}

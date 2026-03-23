package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/kirillinakin/pingcast/internal/adapter/checker"
	natsadapter "github.com/kirillinakin/pingcast/internal/adapter/nats"
	"github.com/kirillinakin/pingcast/internal/adapter/postgres"
	redisadapter "github.com/kirillinakin/pingcast/internal/adapter/redis"
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

	slog.Info("starting", "service", "scheduler", "version", version.Version, "commit", version.Commit)

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

	// Repos (scheduler only needs monitor repo for loading active monitors)
	monitorRepo := postgres.NewMonitorRepo(pool, queries)
	checkResultRepo := postgres.NewCheckResultRepo(queries)

	// Redsync for distributed locks
	rs := redisadapter.NewRedsync(rdb)
	schedulerMutex := rs.NewMutex("lock:scheduler:leader", redsync.WithExpiry(30*time.Second))

	// Check task publisher (scheduler → NATS)
	checkPub := natsadapter.NewCheckPublisher(js)

	// Leader scheduler
	leaderScheduler := checker.NewLeaderScheduler(schedulerMutex, checkPub)

	// Load existing monitors
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

	// Subscribe to monitor changes (add/remove/pause monitors dynamically)
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

	// Data retention cleanup + partition management (daily, with distributed lock)
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

				rangeStart := time.Date(time.Now().Year(), time.Now().Month()+1, 1, 0, 0, 0, 0, time.UTC)
				rangeEnd := rangeStart.AddDate(0, 1, 0)
				safeName := pgx.Identifier{fmt.Sprintf("check_results_%d_%02d", rangeStart.Year(), rangeStart.Month())}.Sanitize()
				ddl := fmt.Sprintf(
					"CREATE TABLE IF NOT EXISTS %s PARTITION OF check_results FOR VALUES FROM ('%s') TO ('%s')",
					safeName,
					rangeStart.Format("2006-01-02"),
					rangeEnd.Format("2006-01-02"),
				)
				if _, err = pool.Exec(ctx, ddl); err != nil {
					slog.Error("partition creation failed", "partition", safeName, "error", err)
				} else {
					slog.Info("ensured partition exists", "partition", safeName)
				}

				if _, err := cleanupMutex.Unlock(); err != nil {
					slog.Error("failed to release cleanup lock", "error", err)
				}
			}
		}
	}()

	slog.Info("scheduler started", "monitors", len(activeMonitors))
	<-ctx.Done()
	slog.Info("scheduler shutting down")

	monitorSub.Stop()
	leaderScheduler.Stop()
	slog.Info("scheduler shutdown complete")
}

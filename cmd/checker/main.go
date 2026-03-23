package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

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
	monitorRepo := postgres.NewMonitorRepo(queries)
	checkResultRepo := postgres.NewCheckResultRepo(queries)
	incidentRepo := postgres.NewIncidentRepo(queries)
	uptimeRepo := postgres.NewUptimeRepo(queries)

	// NATS publisher
	alertPub := natsadapter.NewAlertPublisher(js)

	// Checker registry
	registry := checker.NewRegistry()
	registry.Register(domain.MonitorHTTP, "HTTP", checker.NewHTTPChecker())
	registry.Register(domain.MonitorTCP, "TCP", checker.NewTCPChecker(10*time.Second))
	registry.Register(domain.MonitorDNS, "DNS", checker.NewDNSChecker())

	// App service (registry injected via port)
	monitoringSvc := app.NewMonitoringService(monitorRepo, checkResultRepo, incidentRepo, userRepo, uptimeRepo, alertPub, registry)

	// Host limiter (Redis-based, distributed across replicas)
	hostLimiter := redisadapter.NewHostLimiter(rdb, 3, 30*time.Second)

	// Worker pool delegates to MonitoringService.RunCheck
	workerPool := checker.NewWorkerPool(ctx, 100, registry, hostLimiter, func(ctx context.Context, monitor *domain.Monitor) {
		if err := monitoringSvc.RunCheck(ctx, monitor); err != nil {
			slog.Error("failed to run check", "monitor_id", monitor.ID, "error", err)
		}
	})

	scheduler := checker.NewScheduler(func(m *domain.Monitor) {
		workerPool.Submit(m)
	})

	// Load existing monitors from DB
	activeMonitors, err := monitorRepo.ListActive(ctx)
	if err != nil {
		slog.Error("failed to load active monitors", "error", err)
		os.Exit(1)
	}
	for i := range activeMonitors {
		scheduler.Add(&activeMonitors[i])
	}
	slog.Info("loaded monitors", "count", len(activeMonitors))

	// Subscribe to monitor changes via NATS
	monitorSub := natsadapter.NewMonitorSubscriber(js)
	if err := monitorSub.Subscribe(ctx, func(ctx context.Context, action domain.MonitorAction, monitorID uuid.UUID, monitor *domain.Monitor) error {
		switch action {
		case domain.ActionCreate, domain.ActionUpdate, domain.ActionResume:
			if monitor != nil {
				scheduler.Add(monitor)
			}
		case domain.ActionDelete, domain.ActionPause:
			scheduler.Remove(monitorID)
		}
		slog.Info("processed monitor change", "action", action, "monitor_id", monitorID)
		return nil
	}); err != nil {
		slog.Error("failed to subscribe to monitor changes", "error", err)
		os.Exit(1)
	}

	// Data retention cleanup (daily, with distributed lock)
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				acquired, err := redisadapter.TryLock(ctx, rdb, "cleanup:retention", 1*time.Hour)
				if err != nil {
					slog.Error("failed to acquire cleanup lock", "error", err)
					continue
				}
				if !acquired {
					slog.Debug("cleanup lock held by another instance, skipping")
					continue
				}

				cutoff := time.Now().Add(-90 * 24 * time.Hour)
				deleted, err := checkResultRepo.DeleteOlderThan(ctx, cutoff)
				if err != nil {
					slog.Error("retention cleanup failed", "error", err)
				} else if deleted > 0 {
					slog.Info("retention cleanup", "deleted_rows", deleted)
				}

				if err := redisadapter.Unlock(ctx, rdb, "cleanup:retention"); err != nil {
					slog.Error("failed to release cleanup lock", "error", err)
				}
			}
		}
	}()

	slog.Info("checker started", "monitors", len(activeMonitors))
	<-ctx.Done()
	slog.Info("checker shutting down")

	monitorSub.Stop()
	scheduler.Stop()
	workerPool.Stop()
}

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

	natsadapter "github.com/kirillinakin/pingcast/internal/adapter/nats"
	"github.com/kirillinakin/pingcast/internal/adapter/postgres"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/checker"
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
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

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
	monitorRepo := postgres.NewMonitorRepo(queries)
	checkResultRepo := postgres.NewCheckResultRepo(queries)
	incidentRepo := postgres.NewIncidentRepo(queries)
	sessionRepo := postgres.NewSessionRepo(queries)

	// NATS publisher
	alertPub := natsadapter.NewAlertPublisher(js)

	// App service
	monitoringSvc := app.NewMonitoringService(monitorRepo, checkResultRepo, incidentRepo, userRepo, alertPub)

	// Check handler: converts checker types → domain types, delegates to MonitoringService
	checkHandler := func(ctx context.Context, monitor *checker.MonitorInfo, result *checker.CheckResult) {
		domainMonitor := &domain.Monitor{
			ID:                 monitor.ID,
			UserID:             monitor.UserID,
			Name:               monitor.Name,
			URL:                monitor.URL,
			Method:             domain.HTTPMethod(monitor.Method),
			IntervalSeconds:    monitor.IntervalSeconds,
			ExpectedStatus:     monitor.ExpectedStatus,
			Keyword:            monitor.Keyword,
			AlertAfterFailures: monitor.AlertAfterFailures,
		}

		var statusCode *int
		if result.StatusCode != nil {
			sc := int(*result.StatusCode)
			statusCode = &sc
		}

		domainResult := &domain.CheckResult{
			MonitorID:      result.MonitorID,
			Status:         domain.MonitorStatus(result.Status),
			StatusCode:     statusCode,
			ResponseTimeMs: result.ResponseTimeMs,
			ErrorMessage:   result.ErrorMessage,
			CheckedAt:      result.CheckedAt,
		}

		if err := monitoringSvc.ProcessCheckResult(ctx, domainMonitor, domainResult); err != nil {
			slog.Error("failed to process check result", "monitor_id", monitor.ID, "error", err)
		}
	}

	// Checker
	client := checker.NewClient()
	workerPool := checker.NewWorkerPool(ctx, 100, client, checkHandler)

	scheduler := checker.NewScheduler(func(m *checker.MonitorInfo) {
		workerPool.Submit(m)
	})

	// Load existing monitors from DB
	activeMonitors, _ := monitorRepo.ListActive(ctx)
	for _, m := range activeMonitors {
		scheduler.Add(domainToMonitorInfo(&m))
	}
	slog.Info("loaded monitors", "count", len(activeMonitors))

	// Subscribe to monitor changes via NATS
	monitorSub := natsadapter.NewMonitorSubscriber(js)
	if err := monitorSub.Subscribe(ctx, func(ctx context.Context, action domain.MonitorAction, monitorID uuid.UUID, monitor *domain.Monitor) error {
		switch action {
		case domain.ActionCreate, domain.ActionUpdate, domain.ActionResume:
			if monitor != nil {
				scheduler.Add(domainToMonitorInfo(monitor))
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
				deleted, err := checkResultRepo.DeleteOlderThan(ctx, cutoff)
				if err != nil {
					slog.Error("retention cleanup failed", "error", err)
				} else if deleted > 0 {
					slog.Info("retention cleanup", "deleted_rows", deleted)
				}
				sessDeleted, err := sessionRepo.DeleteExpired(ctx)
				if err != nil {
					slog.Error("session cleanup failed", "error", err)
				} else if sessDeleted > 0 {
					slog.Info("session cleanup", "deleted", sessDeleted)
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

func domainToMonitorInfo(m *domain.Monitor) *checker.MonitorInfo {
	return &checker.MonitorInfo{
		ID:                 m.ID,
		UserID:             m.UserID,
		Name:               m.Name,
		URL:                m.URL,
		Method:             string(m.Method),
		IntervalSeconds:    m.IntervalSeconds,
		ExpectedStatus:     m.ExpectedStatus,
		Keyword:            m.Keyword,
		AlertAfterFailures: m.AlertAfterFailures,
	}
}

package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/kirillinakin/pingcast/internal/checker"
	"github.com/kirillinakin/pingcast/internal/config"
	"github.com/kirillinakin/pingcast/internal/database"
	natsbus "github.com/kirillinakin/pingcast/internal/nats"
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

	// Publish alert helper
	publishAlert := func(ctx context.Context, event *natsbus.AlertEvent) {
		subject := "alerts." + event.Event
		data, err := json.Marshal(event)
		if err != nil {
			slog.Error("failed to marshal alert event", "error", err)
			return
		}
		if _, err := js.Publish(ctx, subject, data); err != nil {
			slog.Error("failed to publish alert", "event", event.Event, "monitor_id", event.MonitorID, "error", err)
		}
	}

	// Check handler: writes results, manages incidents, publishes fat events
	checkHandler := func(ctx context.Context, monitor *checker.MonitorInfo, result *checker.CheckResult) {
		if _, err := queries.InsertCheckResult(ctx, sqlcgen.InsertCheckResultParams{
			MonitorID:      monitor.ID,
			Status:         result.Status,
			StatusCode:     pgtype.Int4{Int32: derefInt32(result.StatusCode), Valid: result.StatusCode != nil},
			ResponseTimeMs: int32(result.ResponseTimeMs),
			ErrorMessage:   result.ErrorMessage,
			CheckedAt:      result.CheckedAt,
		}); err != nil {
			slog.Error("failed to insert check result", "monitor_id", monitor.ID, "error", err)
		}

		// Read current status from DB
		currentMon, err := queries.GetMonitorByID(ctx, monitor.ID)
		previousStatus := "unknown"
		if err == nil {
			previousStatus = currentMon.CurrentStatus
		}

		queries.UpdateMonitorStatus(ctx, sqlcgen.UpdateMonitorStatusParams{
			ID: monitor.ID, CurrentStatus: result.Status,
		})

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

					incident, dbErr := queries.CreateIncident(ctx, sqlcgen.CreateIncidentParams{
						MonitorID: monitor.ID, Cause: errMsg,
					})

					alertEvent := &natsbus.AlertEvent{
						MonitorID:   monitor.ID,
						MonitorName: monitor.Name,
						MonitorURL:  monitor.URL,
						Event:       "down",
						Cause:       errMsg,
					}
					if dbErr == nil {
						alertEvent.IncidentID = incident.ID
					}

					userInfo, uErr := queries.GetUserAlertInfo(ctx, monitor.UserID)
					if uErr == nil {
						alertEvent.Email = userInfo.Email
						alertEvent.Plan = userInfo.Plan
						if userInfo.TgChatID.Valid {
							alertEvent.TgChatID = &userInfo.TgChatID.Int64
						}
					}

					publishAlert(ctx, alertEvent)
				}
			}
		} else if result.Status == "up" && previousStatus == "down" {
			incident, err := queries.GetOpenIncidentByMonitorID(ctx, monitor.ID)
			if err == nil {
				now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
				queries.ResolveIncident(ctx, sqlcgen.ResolveIncidentParams{
					ID: incident.ID, ResolvedAt: now,
				})

				alertEvent := &natsbus.AlertEvent{
					MonitorID:   monitor.ID,
					IncidentID:  incident.ID,
					MonitorName: monitor.Name,
					MonitorURL:  monitor.URL,
					Event:       "up",
				}

				userInfo, uErr := queries.GetUserAlertInfo(ctx, monitor.UserID)
				if uErr == nil {
					alertEvent.Email = userInfo.Email
					alertEvent.Plan = userInfo.Plan
					if userInfo.TgChatID.Valid {
						alertEvent.TgChatID = &userInfo.TgChatID.Int64
					}
				}

				publishAlert(ctx, alertEvent)
			}
		}
	}

	// Checker
	client := checker.NewClient()
	workerPool := checker.NewWorkerPool(ctx, 100, client, checkHandler)

	scheduler := checker.NewScheduler(func(m *checker.MonitorInfo) {
		workerPool.Submit(m)
	})

	// Load existing monitors
	monitors, _ := queries.ListActiveMonitors(ctx)
	for _, m := range monitors {
		scheduler.Add(monitorToInfo(m))
	}
	slog.Info("loaded monitors", "count", len(monitors))

	// Subscribe to monitors.changed
	cons, err := js.CreateOrUpdateConsumer(ctx, "MONITORS", jetstream.ConsumerConfig{
		Durable:       "checker-worker",
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    10,
		AckWait:       30 * time.Second,
		FilterSubject: "monitors.changed",
	})
	if err != nil {
		slog.Error("failed to create consumer", "error", err)
		os.Exit(1)
	}

	consCtx, err := cons.Consume(func(msg jetstream.Msg) {
		var event natsbus.MonitorChangedEvent
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			slog.Error("invalid monitors.changed event", "error", err)
			msg.Nak()
			return
		}

		switch event.Action {
		case "create", "update", "resume":
			if event.Monitor != nil {
				scheduler.Add(eventToInfo(event.Monitor))
			}
		case "delete", "pause":
			scheduler.Remove(event.MonitorID)
		}

		msg.Ack()
		slog.Info("processed monitor change", "action", event.Action, "monitor_id", event.MonitorID)
	})
	if err != nil {
		slog.Error("failed to start consumer", "error", err)
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

	slog.Info("checker started", "monitors", len(monitors))
	<-ctx.Done()
	slog.Info("checker shutting down")

	consCtx.Stop()
	scheduler.Stop()
	workerPool.Stop()
}

func monitorToInfo(m sqlcgen.Monitor) *checker.MonitorInfo {
	return &checker.MonitorInfo{
		ID:                 m.ID,
		UserID:             m.UserID,
		Name:               m.Name,
		URL:                m.Url,
		Method:             m.Method,
		IntervalSeconds:    int(m.IntervalSeconds),
		ExpectedStatus:     int(m.ExpectedStatus),
		Keyword:            m.Keyword,
		AlertAfterFailures: int(m.AlertAfterFailures),
	}
}

func eventToInfo(m *natsbus.MonitorData) *checker.MonitorInfo {
	return &checker.MonitorInfo{
		ID:                 m.ID,
		UserID:             m.UserID,
		Name:               m.Name,
		URL:                m.URL,
		Method:             m.Method,
		IntervalSeconds:    m.IntervalSeconds,
		ExpectedStatus:     m.ExpectedStatus,
		Keyword:            m.Keyword,
		AlertAfterFailures: m.AlertAfterFailures,
	}
}

func derefInt32(p *int32) int32 {
	if p == nil {
		return 0
	}
	return *p
}

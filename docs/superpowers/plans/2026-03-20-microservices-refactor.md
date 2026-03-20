# PingCast Microservices Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split PingCast monolith into 3 services (api, checker, notifier) communicating via NATS JetStream.

**Architecture:** API publishes `monitors.changed` events to NATS when monitors are CRUDed. Checker subscribes, updates scheduler, executes checks, publishes fat `alerts.*` events. Notifier subscribes to alerts and sends Telegram/email. Checker and API share PostgreSQL. Notifier is stateless — NATS only.

**Tech Stack:** Go, NATS JetStream, Fiber, sqlc, PostgreSQL, Docker

**Spec:** `docs/superpowers/specs/2026-03-20-pingcast-microservices-refactor.md`

---

## Task 1: Add NATS dependency and shared event types

**Files:**
- Create: `internal/nats/client.go`
- Create: `internal/nats/events.go`

- [ ] **Step 1: Install NATS Go client**

```bash
go get github.com/nats-io/nats.go
```

- [ ] **Step 2: Create NATS client helper**

Create `internal/nats/client.go`:

```go
package natsbus

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func Connect(url string) (*nats.Conn, error) {
	nc, err := nats.Connect(url,
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.ReconnectBufSize(8*1024*1024),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			slog.Error("nats disconnected", "error", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			slog.Info("nats reconnected", "url", nc.ConnectedUrl())
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to nats: %w", err)
	}
	return nc, nil
}

func SetupStreams(ctx context.Context, js jetstream.JetStream) error {
	_, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      "MONITORS",
		Subjects:  []string{"monitors.changed"},
		Retention: jetstream.WorkQueuePolicy,
		Storage:   jetstream.FileStorage,
		MaxAge:    24 * time.Hour,
	})
	if err != nil {
		return fmt.Errorf("create MONITORS stream: %w", err)
	}

	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      "ALERTS",
		Subjects:  []string{"alerts.>"},
		Retention: jetstream.WorkQueuePolicy,
		Storage:   jetstream.FileStorage,
		MaxAge:    24 * time.Hour,
	})
	if err != nil {
		return fmt.Errorf("create ALERTS stream: %w", err)
	}

	return nil
}
```

Note: Add `"context"` to imports.

- [ ] **Step 3: Create shared event types**

Create `internal/nats/events.go`:

```go
package natsbus

import "github.com/google/uuid"

// MonitorChangedEvent is published by API when a monitor is created/updated/deleted/paused/resumed.
type MonitorChangedEvent struct {
	Action    string       `json:"action"` // create, update, delete, pause, resume
	MonitorID uuid.UUID    `json:"monitor_id"`
	Monitor   *MonitorData `json:"monitor,omitempty"`
}

// MonitorData carries full monitor info for create/update/resume actions.
type MonitorData struct {
	ID                 uuid.UUID `json:"id"`
	Name               string    `json:"name"`
	URL                string    `json:"url"`
	Method             string    `json:"method"`
	IntervalSeconds    int       `json:"interval_seconds"`
	ExpectedStatus     int       `json:"expected_status"`
	Keyword            *string   `json:"keyword,omitempty"`
	AlertAfterFailures int       `json:"alert_after_failures"`
	UserID             uuid.UUID `json:"user_id"`
}

// AlertEvent is published by Checker when a monitor goes down or recovers.
// Fat event — contains all data needed for notification delivery.
type AlertEvent struct {
	MonitorID   uuid.UUID `json:"monitor_id"`
	IncidentID  int64     `json:"incident_id"`
	MonitorName string    `json:"monitor_name"`
	MonitorURL  string    `json:"monitor_url"`
	Event       string    `json:"event"` // "down" or "up"
	Cause       string    `json:"cause,omitempty"`
	TgChatID    *int64    `json:"tg_chat_id,omitempty"`
	Email       string    `json:"email"`
	Plan        string    `json:"plan"`
}
```

- [ ] **Step 4: Verify compilation**

```bash
go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add internal/nats/ go.mod go.sum
git commit -m "feat: NATS client helper and shared event types"
```

---

## Task 2: Config, MonitorInfo, handler callback, delete monolith, new sqlc query

Tasks 2-4 from the original ordering are merged into one atomic task. This avoids intermediate compilation failures since `config.Load()` removal and `onChanged` signature change both break `cmd/pingcast/main.go`.

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/checker/client.go`
- Modify: `internal/handler/server.go`
- Modify: `internal/sqlc/queries/users.sql`
- Delete: `cmd/pingcast/main.go`
- Delete: `internal/notifier/listener.go`
- Regenerate: `internal/sqlc/gen/`

- [ ] **Step 1: Delete old monolith files first**

```bash
rm cmd/pingcast/main.go internal/notifier/listener.go
rmdir cmd/pingcast
```

- [ ] **Step 2: Replace single Load() with per-service functions**

Rewrite `internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"
	"strconv"
)

type APIConfig struct {
	Port                       int
	DatabaseURL                string
	NatsURL                    string
	LemonSqueezyWebhookSecret string
	BaseURL                    string
}

type CheckerConfig struct {
	DatabaseURL string
	NatsURL     string
}

type NotifierConfig struct {
	NatsURL       string
	TelegramToken string
	SMTPHost      string
	SMTPPort      int
	SMTPUser      string
	SMTPPass      string
	SMTPFrom      string
}

func LoadAPI() (*APIConfig, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	port, _ := strconv.Atoi(getEnv("PORT", "8080"))

	return &APIConfig{
		Port:                       port,
		DatabaseURL:                dbURL,
		NatsURL:                    getEnv("NATS_URL", "nats://localhost:4222"),
		LemonSqueezyWebhookSecret: os.Getenv("LEMONSQUEEZY_WEBHOOK_SECRET"),
		BaseURL:                    getEnv("BASE_URL", "http://localhost:8080"),
	}, nil
}

func LoadChecker() (*CheckerConfig, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return &CheckerConfig{
		DatabaseURL: dbURL,
		NatsURL:     getEnv("NATS_URL", "nats://localhost:4222"),
	}, nil
}

func LoadNotifier() (*NotifierConfig, error) {
	smtpPort, _ := strconv.Atoi(getEnv("SMTP_PORT", "587"))

	return &NotifierConfig{
		NatsURL:       getEnv("NATS_URL", "nats://localhost:4222"),
		TelegramToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		SMTPHost:      os.Getenv("SMTP_HOST"),
		SMTPPort:      smtpPort,
		SMTPUser:      os.Getenv("SMTP_USER"),
		SMTPPass:      os.Getenv("SMTP_PASS"),
		SMTPFrom:      getEnv("SMTP_FROM", "noreply@pingcast.io"),
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 2: Add GetUserAlertInfo sqlc query**

Append to `internal/sqlc/queries/users.sql`:

```sql
-- name: GetUserAlertInfo :one
SELECT tg_chat_id, email, plan
FROM users WHERE id = $1;
```

- [ ] **Step 3: Regenerate sqlc**

```bash
cd internal/sqlc && sqlc generate && cd ../..
```

- [ ] **Step 4: Add UserID and Name to MonitorInfo**

In `internal/checker/client.go`, change:

```go
type MonitorInfo struct {
	ID                 uuid.UUID
	URL                string
	Method             string
	IntervalSeconds    int
	ExpectedStatus     int
	Keyword            *string
	AlertAfterFailures int
}
```

to:

```go
type MonitorInfo struct {
	ID                 uuid.UUID
	UserID             uuid.UUID
	Name               string
	URL                string
	Method             string
	IntervalSeconds    int
	ExpectedStatus     int
	Keyword            *string
	AlertAfterFailures int
}
```

- [ ] **Step 2: Change onChanged callback in server.go**

In `internal/handler/server.go`, change the `Server` struct and `NewServer`:

```go
type Server struct {
	queries     *gen.Queries
	authService *auth.Service
	rateLimiter *auth.RateLimiter
	onChanged   func(action string, monitorID uuid.UUID, monitor *natsbus.MonitorData)
}

func NewServer(
	queries *gen.Queries,
	authService *auth.Service,
	rateLimiter *auth.RateLimiter,
	onChanged func(action string, monitorID uuid.UUID, monitor *natsbus.MonitorData),
) *Server {
```

Add import: `natsbus "github.com/kirillinakin/pingcast/internal/nats"`

- [ ] **Step 6: Update all onChanged call sites in server.go**

`CreateMonitor` (currently `s.onChanged(mon.ID, false)`) → change to:
```go
s.onChanged("create", mon.ID, &natsbus.MonitorData{
    ID: mon.ID, Name: mon.Name, URL: mon.Url, Method: mon.Method,
    IntervalSeconds: int(mon.IntervalSeconds), ExpectedStatus: int(mon.ExpectedStatus),
    Keyword: mon.Keyword, AlertAfterFailures: int(mon.AlertAfterFailures), UserID: mon.UserID,
})
```

`UpdateMonitor` (currently `s.onChanged(mon.ID, false)`) → change to:
```go
s.onChanged("update", mon.ID, &natsbus.MonitorData{...})  // same fields from updated monitor
```

`DeleteMonitor` (currently `s.onChanged(uuid.UUID(id), true)`) → change to:
```go
s.onChanged("delete", uuid.UUID(id), nil)
```

`ToggleMonitorPause` (currently `s.onChanged(mon.ID, newPaused)`) → change to:
```go
if newPaused {
    s.onChanged("pause", mon.ID, nil)
} else {
    s.onChanged("resume", mon.ID, &natsbus.MonitorData{...})
}
```

- [ ] **Step 7: Verify compilation**

```bash
go build ./internal/...
```

- [ ] **Step 8: Commit**

```bash
git add -A cmd/pingcast/ internal/notifier/listener.go internal/config/ internal/sqlc/ internal/checker/client.go internal/handler/server.go
git commit -m "refactor: split config, update MonitorInfo/onChanged, delete monolith, add GetUserAlertInfo"
```

---

## Task 3: Create cmd/api/main.go

**Files:**
- Create: `cmd/api/main.go`

- [ ] **Step 1: Create API entry point**

```go
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
		data, _ := json.Marshal(event)
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
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./cmd/api/
```

- [ ] **Step 3: Commit**

```bash
git add cmd/api/
git commit -m "feat: API service entry point with NATS publish"
```

---

## Task 4: Create cmd/checker/main.go

**Files:**
- Create: `cmd/checker/main.go`

- [ ] **Step 1: Create Checker entry point**

```go
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

	// Ensure streams exist (idempotent — safe if API already created them)
	if err := natsbus.SetupStreams(ctx, js); err != nil {
		slog.Error("failed to setup nats streams", "error", err)
		os.Exit(1)
	}

	// Publish alert helper
	publishAlert := func(ctx context.Context, event *natsbus.AlertEvent) {
		subject := "alerts." + event.Event
		data, _ := json.Marshal(event)
		if _, err := js.Publish(ctx, subject, data); err != nil {
			slog.Error("failed to publish alert", "event", event.Event, "monitor_id", event.MonitorID, "error", err)
		}
	}

	// Check handler
	checkHandler := func(ctx context.Context, monitor *checker.MonitorInfo, result *checker.CheckResult) {
		// Write check result
		queries.InsertCheckResult(ctx, sqlcgen.InsertCheckResultParams{
			MonitorID:      monitor.ID,
			Status:         result.Status,
			StatusCode:     pgtype.Int4{Int32: derefInt32(result.StatusCode), Valid: result.StatusCode != nil},
			ResponseTimeMs: int32(result.ResponseTimeMs),
			ErrorMessage:   result.ErrorMessage,
			CheckedAt:      result.CheckedAt,
		})

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

					// Build fat event
					alertEvent := &natsbus.AlertEvent{
						MonitorID:   monitor.ID,
						MonitorName: monitor.Name,
						MonitorURL:  monitor.URL,
						Event:       "down",
						Cause:       errMsg,
						Email:       "",
						Plan:        "free",
					}
					if dbErr == nil {
						alertEvent.IncidentID = incident.ID
					}

					// Fetch user alert info
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
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./cmd/checker/
```

- [ ] **Step 3: Commit**

```bash
git add cmd/checker/
git commit -m "feat: Checker service with NATS subscribe and fat event publish"
```

---

## Task 5: Create cmd/notifier/main.go

**Files:**
- Create: `cmd/notifier/main.go`

- [ ] **Step 1: Create Notifier entry point**

```go
package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/kirillinakin/pingcast/internal/config"
	natsbus "github.com/kirillinakin/pingcast/internal/nats"
	"github.com/kirillinakin/pingcast/internal/notifier"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.LoadNotifier()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

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

	// Ensure streams exist (idempotent)
	if err := natsbus.SetupStreams(ctx, js); err != nil {
		slog.Error("failed to setup nats streams", "error", err)
		os.Exit(1)
	}

	// Senders
	var tgSender *notifier.TelegramSender
	if cfg.TelegramToken != "" {
		tgSender = notifier.NewTelegramSender(cfg.TelegramToken)
	}

	var emailSender *notifier.EmailSender
	if cfg.SMTPHost != "" {
		emailSender = notifier.NewEmailSender(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPFrom)
	}

	// Subscribe to alerts
	cons, err := js.CreateOrUpdateConsumer(ctx, "ALERTS", jetstream.ConsumerConfig{
		Durable:    "notifier-alerts",
		AckPolicy:  jetstream.AckExplicitPolicy,
		MaxDeliver: 5,
		AckWait:    60 * time.Second,
		BackOff:    []time.Duration{1 * time.Second, 5 * time.Second, 30 * time.Second, 2 * time.Minute, 10 * time.Minute},
	})
	if err != nil {
		slog.Error("failed to create consumer", "error", err)
		os.Exit(1)
	}

	consCtx, err := cons.Consume(func(msg jetstream.Msg) {
		var event natsbus.AlertEvent
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			slog.Error("invalid alert event", "error", err)
			msg.Term()
			return
		}

		if err := handleAlert(&event, tgSender, emailSender); err != nil {
			slog.Error("alert delivery failed, will retry", "event", event.Event, "monitor_id", event.MonitorID, "error", err)
			msg.Nak()
			return
		}

		msg.Ack()
		slog.Info("alert delivered", "event", event.Event, "monitor_id", event.MonitorID)
	})
	if err != nil {
		slog.Error("failed to start consumer", "error", err)
		os.Exit(1)
	}

	slog.Info("notifier started")
	<-ctx.Done()
	slog.Info("notifier shutting down")
	consCtx.Stop()
}

func handleAlert(event *natsbus.AlertEvent, tg *notifier.TelegramSender, email *notifier.EmailSender) error {
	// Telegram
	if event.TgChatID != nil && tg != nil {
		var err error
		switch event.Event {
		case "down":
			err = tg.SendDown(*event.TgChatID, event.MonitorName, event.MonitorURL, event.Cause)
		case "up":
			err = tg.SendUp(*event.TgChatID, event.MonitorName, event.MonitorURL)
		}
		if err != nil {
			return err
		}
	}

	// Email (Pro only)
	if event.Plan == "pro" && event.Email != "" && email != nil {
		var err error
		switch event.Event {
		case "down":
			err = email.SendDown(event.Email, event.MonitorName, event.MonitorURL, event.Cause)
		case "up":
			err = email.SendUp(event.Email, event.MonitorName, event.MonitorURL)
		}
		if err != nil {
			return err
		}
	}

	return nil
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./cmd/notifier/
```

- [ ] **Step 3: Commit**

```bash
git add cmd/notifier/
git commit -m "feat: Notifier service — stateless alert sender via NATS"
```

---

## Task 6: Update Dockerfile, docker-compose, CI

**Files:**
- Modify: `Dockerfile`
- Modify: `docker-compose.yml`
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Rewrite Dockerfile for multi-target**

```dockerfile
FROM golang:1.25-alpine AS builder
ARG SERVICE
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /service ./cmd/${SERVICE}/

FROM alpine:3.21
RUN apk --no-cache add ca-certificates
COPY --from=builder /service /service
CMD ["/service"]
```

- [ ] **Step 2: Rewrite docker-compose.yml**

Use the docker-compose from the spec (4 services: api, checker, notifier, nats, db).

- [ ] **Step 3: Update CI to build all 3 binaries**

In `.github/workflows/ci.yml`, replace the Build step:

```yaml
      - name: Build all services
        run: |
          go build ./cmd/api/
          go build ./cmd/checker/
          go build ./cmd/notifier/
```

- [ ] **Step 4: Verify all builds**

```bash
go build ./cmd/api/ && go build ./cmd/checker/ && go build ./cmd/notifier/
```

- [ ] **Step 5: Run all tests**

```bash
go test ./... -count=1 -timeout=30s
```

- [ ] **Step 6: Commit**

```bash
git add Dockerfile docker-compose.yml .github/
git commit -m "feat: multi-service Docker setup with NATS JetStream"
```

---

## Summary

6 tasks, ordered by dependency. Each task produces a working commit.

**Task order:**
1. NATS dependency + shared event types
2. Config + MonitorInfo + handler callback + delete monolith + new sqlc query (atomic)
3. Create cmd/api/main.go
4. Create cmd/checker/main.go
5. Create cmd/notifier/main.go
6. Update Dockerfile, docker-compose, CI (Go version 1.25 to match go.mod)

**Note:** All 3 services call `natsbus.SetupStreams()` on startup (idempotent) so they can start in any order.

**What stays unchanged:** auth, web templates, database, sqlc queries (except new one), checker client/scheduler/hostlimit, telegram/email senders.

**What gets deleted:** `cmd/pingcast/main.go`, `internal/notifier/listener.go`

**Key behavioral changes:**
- API publishes `monitors.changed` to NATS instead of calling scheduler directly
- Checker subscribes to NATS, manages scheduler, builds fat events with user data
- Notifier is pure NATS consumer — no DB, no PG LISTEN/NOTIFY
- Each service has its own config loader and graceful shutdown sequence

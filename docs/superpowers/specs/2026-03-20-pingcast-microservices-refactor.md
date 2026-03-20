# PingCast Microservices Refactor — Design Spec

## Overview

Refactor PingCast from a single Go binary (monolith) into 3 independent services communicating via NATS JetStream. This replaces the PG LISTEN/NOTIFY mechanism and enables independent deployment, scaling, and failure isolation.

## Architecture

```
┌──────────────┐       NATS JetStream        ┌──────────────┐
│              │  monitors.changed            │              │
│   API        │──────────────────────────────►│   Checker    │
│  (Fiber)     │                              │  (scheduler  │
│              │◄──── PostgreSQL ────────────►│   + workers) │
└──────────────┘                              └──────┬───────┘
                                                     │
                                              alerts.down/up
                                              (fat events)
                                                     │
                                                     ▼
                                              ┌──────────────┐
                                              │   Notifier   │
                                              │  (TG/email)  │
                                              │  stateless   │
                                              └──────────────┘
```

## Services

### API (`cmd/api/main.go`)

**Responsibility:** HTTP API (oapi-codegen), HTML pages (HTMX), authentication, Lemon Squeezy webhooks, Telegram bot /start webhook.

**Connections:** PostgreSQL (read/write), NATS (publish only).

**Behavior on monitor CRUD:**
- Creates/updates/deletes monitor in PostgreSQL
- Publishes `monitors.changed` event to NATS so checker updates its scheduler
- No longer calls scheduler directly (scheduler lives in checker process)

**NATS publish interface:** `server.go`'s `onChanged` callback is replaced with a new signature: `func(action string, monitorID uuid.UUID, monitor *natsevents.MonitorData)`. Each handler method determines the action:
- `CreateMonitor` → action `"create"`, full monitor data
- `UpdateMonitor` → action `"update"`, full monitor data
- `DeleteMonitor` → action `"delete"`, monitor data nil
- `ToggleMonitorPause` → action `"pause"` or `"resume"`, monitor data nil for pause, full data for resume (checker needs data to re-add to scheduler)

The callback is injected into `NewServer()` and internally publishes to NATS. The `Server` struct does not hold a NATS client directly.

**Runs migrations** on startup. Only service that runs migrations.

### Checker (`cmd/checker/main.go`)

**Responsibility:** Scheduler (timer-per-monitor), worker pool, HTTP check execution, check result persistence, incident state machine.

**Connections:** PostgreSQL (read/write), NATS (subscribe to `monitors.changed`, publish to `alerts.*`).

**Startup:**
1. Connects to PostgreSQL, NATS
2. Loads all active monitors from DB (`ListActiveMonitors`)
3. Starts scheduler with loaded monitors
4. Subscribes to `monitors.changed` — updates scheduler on create/update/delete/pause/resume

**On check result:**
1. Writes check result to `check_results` table
2. Reads current monitor status from DB (avoids stale in-memory state)
3. Updates `monitors.current_status`
4. If status transition detected:
   - Down: checks consecutive failures, cooldown, creates incident, publishes fat `alerts.down` event
   - Up (was down): resolves incident, publishes fat `alerts.up` event

**Fat event construction:** Checker needs `user_id`, `monitor_name` to build fat events. Changes required:
- Add `UserID uuid.UUID` and `Name string` fields to `MonitorInfo` struct in `internal/checker/client.go`
- Add new sqlc query `GetUserAlertInfo` that returns only `tg_chat_id`, `email`, `plan` by user ID (minimal data for fat event)
- The check handler (in `cmd/checker/main.go`) calls `GetUserAlertInfo(monitor.UserID)` and constructs the fat event with all required fields

**Data retention cleanup:** Runs daily goroutine to delete old check_results and expired sessions.

**Singleton:** Checker is designed as a single instance. The NATS WorkQueue retention ensures each `monitors.changed` event is delivered to exactly one consumer. Horizontal scaling of checkers is not supported in this design and would require a different approach (e.g., partitioning monitors across checker instances).

### Notifier (`cmd/notifier/main.go`)

**Responsibility:** Receives alert events from NATS, sends Telegram messages and emails.

**Connections:** NATS only (subscribe to `alerts.*`). No PostgreSQL access.

**Completely stateless.** Receives a fat event with all data needed to send the alert. No DB lookups.

**On `alerts.down`:**
- If `tg_chat_id` is present → send Telegram "DOWN" message
- If `plan == "pro"` and `email` is present → send email

**On `alerts.up`:**
- Same logic but "UP" / "recovered" message

**Error handling:** If send fails → nack the NATS message → JetStream retries with backoff.

## NATS Configuration

### Streams

**Stream `MONITORS`:**
- Subjects: `monitors.changed`
- Retention: WorkQueue
- Storage: File (persist across restarts)
- Max age: 24h (discard stale events)

**Stream `ALERTS`:**
- Subjects: `alerts.>`
- Retention: WorkQueue
- Storage: File
- Max age: 24h

### Consumers

**Consumer `checker-worker` on stream `MONITORS`:**
- Durable name: `checker-worker`
- Ack policy: Explicit
- Max deliver: 10
- Ack wait: 30s

**Consumer `notifier-alerts` on stream `ALERTS`:**
- Durable name: `notifier-alerts`
- Ack policy: Explicit
- Max deliver: 5
- Ack wait: 60s (Telegram/email can be slow)
- Backoff: 1s, 5s, 30s, 2m, 10m

### Connection

All services connect to NATS via `NATS_URL` env var (default: `nats://nats:4222`).

## Event Contracts

### `monitors.changed` (API → Checker)

```json
{
  "action": "create | update | delete | pause | resume",
  "monitor_id": "uuid",
  "monitor": {
    "id": "uuid",
    "name": "My API",
    "url": "https://example.com",
    "method": "GET",
    "interval_seconds": 300,
    "expected_status": 200,
    "keyword": null,
    "alert_after_failures": 3,
    "user_id": "uuid"
  }
}
```

- `monitor` is null when `action` is `delete` or `pause` (checker uses `monitor_id` to remove from scheduler)
- `monitor` is required for `create`, `update`, and `resume` (checker needs full data to add/update scheduler entry)

### `alerts.down` (Checker → Notifier)

```json
{
  "monitor_id": "uuid",
  "incident_id": 42,
  "monitor_name": "My API",
  "monitor_url": "https://api.example.com",
  "event": "down",
  "cause": "connection timeout",
  "tg_chat_id": 12345,
  "email": "user@example.com",
  "plan": "pro"
}
```

### `alerts.up` (Checker → Notifier)

```json
{
  "monitor_id": "uuid",
  "incident_id": 42,
  "monitor_name": "My API",
  "monitor_url": "https://api.example.com",
  "event": "up",
  "cause": "",
  "tg_chat_id": 12345,
  "email": "user@example.com",
  "plan": "pro"
}
```

## File Structure Changes

### New files
- `cmd/api/main.go` — API entry point
- `cmd/checker/main.go` — Checker entry point
- `cmd/notifier/main.go` — Notifier entry point
- `internal/nats/client.go` — NATS connection, stream/consumer creation
- `internal/nats/events.go` — Shared event types (MonitorChangedEvent, AlertEvent, MonitorData)

### Modified files
- `internal/handler/server.go` — `onChanged` callback signature changed to `func(action string, monitorID uuid.UUID, monitor *natsevents.MonitorData)`
- `internal/checker/client.go` — Add `UserID` and `Name` fields to `MonitorInfo`
- `internal/checker/worker.go` — `CheckHandler` builds fat events, publishes to NATS
- `internal/config/config.go` — Replace single `Load()` with per-service `LoadAPI()`, `LoadChecker()`, `LoadNotifier()`
- `internal/sqlc/queries/users.sql` — Add `GetUserAlertInfo` query (returns `tg_chat_id`, `email`, `plan` by user ID)
- `docker-compose.yml` — 4 services (api, checker, notifier, nats)
- `Dockerfile` — Multi-target with `SERVICE` build arg
- `.github/workflows/ci.yml` — Build all 3 binaries

### Deleted files
- `cmd/pingcast/main.go` — Replaced by 3 separate entry points
- `internal/notifier/listener.go` — PG LISTEN/NOTIFY removed

### Unchanged files
- `internal/auth/` — No changes
- `internal/handler/pages.go`, `setup.go`, `webhook.go` — No changes
- `internal/web/` — No changes
- `internal/sqlc/` — No changes
- `internal/database/` — No changes
- `internal/checker/client.go`, `hostlimit.go`, `scheduler.go`, `scheduler_test.go` — No changes
- `internal/notifier/telegram.go`, `email.go` — No changes

## Docker Compose

```yaml
services:
  api:
    build:
      context: .
      args:
        SERVICE: api
    ports: ["8080:8080"]
    environment:
      DATABASE_URL: postgres://pingcast:pingcast@db:5432/pingcast?sslmode=disable
      NATS_URL: nats://nats:4222
      PORT: "8080"
    depends_on:
      db: { condition: service_healthy }
      nats: { condition: service_started }

  checker:
    build:
      context: .
      args:
        SERVICE: checker
    environment:
      DATABASE_URL: postgres://pingcast:pingcast@db:5432/pingcast?sslmode=disable
      NATS_URL: nats://nats:4222
    depends_on:
      db: { condition: service_healthy }
      nats: { condition: service_started }

  notifier:
    build:
      context: .
      args:
        SERVICE: notifier
    environment:
      NATS_URL: nats://nats:4222
      TELEGRAM_BOT_TOKEN: ""
    depends_on:
      nats: { condition: service_started }

  nats:
    image: nats:2-alpine
    command: ["--jetstream", "--store_dir=/data"]
    ports: ["4222:4222", "8222:8222"]
    volumes: [nats-data:/data]

  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: pingcast
      POSTGRES_PASSWORD: pingcast
      POSTGRES_DB: pingcast
    ports: ["5432:5432"]
    volumes: [pgdata:/var/lib/postgresql/data]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U pingcast"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  pgdata:
  nats-data:
```

## Dockerfile (multi-target)

```dockerfile
FROM golang:1.24-alpine AS builder
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

## Config Changes

Replace single `config.Load()` with per-service config functions:

```go
// Shared
NatsURL string  // env: NATS_URL, default: nats://localhost:4222

// config.LoadAPI() — requires DATABASE_URL, NATS_URL, PORT
// config.LoadChecker() — requires DATABASE_URL, NATS_URL
// config.LoadNotifier() — requires NATS_URL, reads TELEGRAM_BOT_TOKEN and SMTP_* optionally
```

Each function validates only the env vars its service needs. `DATABASE_URL` is no longer globally required.

## Graceful Shutdown

**API:**
1. Stop accepting new HTTP connections (`app.Shutdown()`)
2. Wait for in-flight requests to complete
3. Disconnect from NATS
4. Close PostgreSQL pool

**Checker:**
1. Drain NATS subscription (stop receiving new `monitors.changed` events)
2. Stop scheduler (no new checks dispatched)
3. Wait for in-flight workers to complete (`workerPool.Stop()`)
4. Disconnect from NATS
5. Close PostgreSQL pool

**Notifier:**
1. Drain NATS subscription (stop receiving new alerts)
2. Wait for in-flight sends to complete (respect `ack_wait` timeout)
3. Disconnect from NATS

## NATS Client Configuration

All services use the same connection options:

```go
nats.Connect(url,
    nats.MaxReconnects(-1),          // reconnect forever
    nats.ReconnectWait(2*time.Second),
    nats.ReconnectBufSize(8*1024*1024), // 8MB buffer during reconnect
    nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
        slog.Error("nats disconnected", "error", err)
    }),
    nats.ReconnectHandler(func(nc *nats.Conn) {
        slog.Info("nats reconnected", "url", nc.ConnectedUrl())
    }),
)
```

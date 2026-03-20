# Notification Channels Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace hardcoded Telegram/Email factories with user-configurable notification channels (Telegram, Email, Webhook) stored in DB, bound to monitors or used globally.

**Architecture:** New domain entity `NotificationChannel` with raw JSON config. `ChannelRegistry` (like `CheckerRegistry`) maps type → factory. `AlertService` rewired to look up channels from DB. Notifier gains PostgreSQL access. `AlertSender.Send(ctx, event)` replaces `NotifyDown`/`NotifyUp`.

**Tech Stack:** Go 1.26, PostgreSQL JSONB, sqlc, NATS JetStream, HTMX, Fiber

**Spec:** `docs/superpowers/specs/2026-03-20-notification-channels.md`

---

## Task Order

1. Domain (NotificationChannel, simplify AlertEvent)
2. Ports (ChannelRegistry, ChannelRepo, simplify AlertSender)
3. Database migration + sqlc queries
4. Channel registry adapter + webhook adapter
5. Rewrite telegram + smtp adapters (ChannelSenderFactory + Send)
6. App layer (AlertService rewrite, MonitoringService simplify)
7. Postgres adapter (channel_repo)
8. NATS adapter (updated AlertEvent)
9. HTTP adapter + frontend (channel CRUD, schema-driven forms)
10. Rewire cmd/ (notifier gets DB, channel registry everywhere)
11. Cleanup + tests

---

## Task 1: Domain Changes

**Files:**
- Create: `internal/domain/channel.go`
- Modify: `internal/domain/alert.go`

- [ ] **Step 1: Create domain/channel.go**

```go
package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type ChannelType string

const (
	ChannelTelegram ChannelType = "telegram"
	ChannelEmail    ChannelType = "email"
	ChannelWebhook  ChannelType = "webhook"
)

func (t ChannelType) Valid() bool {
	switch t {
	case ChannelTelegram, ChannelEmail, ChannelWebhook:
		return true
	}
	return false
}

type NotificationChannel struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Name      string
	Type      ChannelType
	Config    json.RawMessage
	IsEnabled bool
	CreatedAt time.Time
}
```

- [ ] **Step 2: Simplify domain/alert.go**

Remove `TgChatID`, `Email`, `Plan`. Add `UserID`:

```go
type AlertEvent struct {
	MonitorID     uuid.UUID
	UserID        uuid.UUID
	IncidentID    int64
	MonitorName   string
	MonitorTarget string
	Event         AlertEventType
	Cause         string
}
```

- [ ] **Step 3: Verify domain compiles**

```bash
go build ./internal/domain/
```

- [ ] **Step 4: Commit**

```bash
git add internal/domain/
git commit -m "feat: domain — NotificationChannel entity, simplify AlertEvent"
```

---

## Task 2: Port Changes

**Files:**
- Create: `internal/port/channel.go`
- Modify: `internal/port/alerter.go`

- [ ] **Step 1: Create port/channel.go**

```go
package port

import (
	"encoding/json"

	"github.com/kirillinakin/pingcast/internal/domain"
)

type ChannelSenderFactory interface {
	CreateSender(config json.RawMessage) (AlertSender, error)
	ValidateConfig(config json.RawMessage) error
	ConfigSchema() ConfigSchema
}

type ChannelRegistry interface {
	Get(channelType domain.ChannelType) (ChannelSenderFactory, error)
	Types() []ChannelTypeInfo
	ValidateConfig(channelType domain.ChannelType, config json.RawMessage) error
}

type ChannelTypeInfo struct {
	Type   domain.ChannelType `json:"type"`
	Label  string             `json:"label"`
	Schema ConfigSchema       `json:"schema"`
}
```

Reuses `ConfigSchema`, `ConfigField`, `Option` from `port/checker.go`.

- [ ] **Step 2: Simplify port/alerter.go**

```go
type AlertSender interface {
	Send(ctx context.Context, event *domain.AlertEvent) error
}
```

- [ ] **Step 3: Add ChannelRepo to port/repository.go**

```go
type ChannelRepo interface {
	Create(ctx context.Context, ch *domain.NotificationChannel) (*domain.NotificationChannel, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.NotificationChannel, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.NotificationChannel, error)
	ListForMonitor(ctx context.Context, monitorID uuid.UUID) ([]domain.NotificationChannel, error)
	Update(ctx context.Context, ch *domain.NotificationChannel) error
	Delete(ctx context.Context, id, userID uuid.UUID) error
	BindToMonitor(ctx context.Context, monitorID, channelID uuid.UUID) error
	UnbindFromMonitor(ctx context.Context, monitorID, channelID uuid.UUID) error
}
```

- [ ] **Step 4: Verify and commit**

```bash
go build ./internal/port/
git add internal/port/
git commit -m "feat: ports — ChannelRegistry, ChannelRepo, simplified AlertSender"
```

---

## Task 3: Database Migration + sqlc

**Files:**
- Create: `internal/database/migrations/008_create_notification_channels.sql`
- Create: `internal/sqlc/queries/channels.sql`
- Regenerate: `internal/sqlc/gen/`

- [ ] **Step 1: Create migration**

```sql
CREATE TABLE notification_channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(20) NOT NULL,
    config JSONB NOT NULL DEFAULT '{}',
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_channels_user_id ON notification_channels (user_id);

CREATE TABLE monitor_channels (
    monitor_id UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    channel_id UUID NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    PRIMARY KEY (monitor_id, channel_id)
);
```

- [ ] **Step 2: Create sqlc queries**

All queries from spec with explicit column lists.

- [ ] **Step 3: Regenerate sqlc**

```bash
cd internal/sqlc && sqlc generate
```

- [ ] **Step 4: Commit**

```bash
git add internal/database/migrations/ internal/sqlc/
git commit -m "feat: notification channels migration + sqlc queries"
```

---

## Task 4: Channel Registry + Webhook Adapter

**Files:**
- Create: `internal/adapter/channel/registry.go`
- Create: `internal/adapter/webhook/sender.go`

- [ ] **Step 1: Create channel registry** (same pattern as checker registry)

- [ ] **Step 2: Create webhook adapter** — Factory, ValidateConfig (SSRF protection), ConfigSchema, sender with `http.Client{Timeout: 10s}`

- [ ] **Step 3: Verify and commit**

```bash
go build ./internal/adapter/channel/ ./internal/adapter/webhook/
git add internal/adapter/channel/ internal/adapter/webhook/
git commit -m "feat: channel registry + webhook adapter"
```

---

## Task 5: Rewrite Telegram + SMTP Adapters

**Files:**
- Modify: `internal/adapter/telegram/sender.go`
- Modify: `internal/adapter/smtp/sender.go`

- [ ] **Step 1: Rewrite telegram** — `Factory` implements `ChannelSenderFactory`, `sender.Send(ctx, event)` replaces `NotifyDown`/`NotifyUp`

- [ ] **Step 2: Rewrite smtp** — same pattern

- [ ] **Step 3: Delete old ForChat/ForRecipient/chatAlert/recipientAlert code**

- [ ] **Step 4: Verify and commit**

```bash
go build ./internal/adapter/telegram/ ./internal/adapter/smtp/
git add internal/adapter/telegram/ internal/adapter/smtp/
git commit -m "feat: telegram + smtp implement ChannelSenderFactory + Send"
```

---

## Task 6: App Layer Rewrite

**Files:**
- Modify: `internal/app/alert.go`
- Modify: `internal/app/monitoring.go`

- [ ] **Step 1: Rewrite alert.go** — `AlertService` with `ChannelRepo` + `ChannelRegistry`. Handle (best-effort), CRUD methods, BindChannel/UnbindChannel with auth checks.

- [ ] **Step 2: Simplify monitoring.go publishAlert** — remove user lookup, set `UserID` from monitor

- [ ] **Step 3: Verify and commit**

```bash
go build ./internal/app/
git add internal/app/
git commit -m "feat: AlertService with channel CRUD, best-effort delivery"
```

---

## Task 7: Postgres Adapter — ChannelRepo

**Files:**
- Create: `internal/adapter/postgres/channel_repo.go`
- Modify: `internal/adapter/postgres/mapper.go`

- [ ] **Step 1: Create channel_repo.go** — implements `port.ChannelRepo`, uses sqlc queries + mapper

- [ ] **Step 2: Add channel mappers** — sqlc row types → `domain.NotificationChannel`

- [ ] **Step 3: Verify and commit**

```bash
go build ./internal/adapter/postgres/
git add internal/adapter/postgres/
git commit -m "feat: postgres channel_repo with mapper"
```

---

## Task 8: NATS Adapter Updates

**Files:**
- Modify: `internal/adapter/nats/publisher.go`
- Modify: `internal/adapter/nats/subscriber.go`

- [ ] **Step 1: Update AlertEvent serialization** — remove TgChatID/Email/Plan, add UserID

- [ ] **Step 2: Verify and commit**

```bash
go build ./internal/adapter/nats/
git add internal/adapter/nats/
git commit -m "refactor: NATS alert events — simplified, no routing data"
```

---

## Task 9: HTTP Adapter + Frontend

**Files:**
- Modify: `internal/adapter/http/server.go` — channel CRUD API handlers
- Modify: `internal/adapter/http/pages.go` — channel HTML pages
- Modify: `internal/adapter/http/setup.go` — new routes
- Create: `internal/web/templates/channels.html`
- Create: `internal/web/templates/channel_form.html`
- Modify: `internal/web/templates/monitor_form.html` — channel checkboxes
- Modify: `api/openapi.yaml` — channel schemas + endpoints
- Regenerate: `internal/api/gen/server.go`

- [ ] **Step 1: Update OpenAPI, regenerate**
- [ ] **Step 2: Channel API handlers (CRUD + bind/unbind)**
- [ ] **Step 3: Channel HTML pages (list, create form with schema-driven fields)**
- [ ] **Step 4: Monitor form — channel binding checkboxes**
- [ ] **Step 5: New routes in setup.go**
- [ ] **Step 6: Verify and commit**

```bash
go build ./...
git add internal/adapter/http/ internal/web/ api/ internal/api/
git commit -m "feat: channel CRUD API + schema-driven frontend"
```

---

## Task 10: Rewire cmd/

**Files:**
- Modify: `cmd/api/main.go`
- Modify: `cmd/notifier/main.go`
- Modify: `cmd/checker/main.go`
- Modify: `internal/config/config.go`
- Modify: `docker-compose.yml`

- [ ] **Step 1: Update config** — `LoadNotifier` gets `DatabaseURL`

- [ ] **Step 2: Rewrite cmd/notifier/main.go** — DB + channel registry + AlertService

- [ ] **Step 3: Update cmd/api/main.go** — channel registry + AlertService for CRUD

- [ ] **Step 4: Update cmd/checker/main.go** — remove old alert-related code if any

- [ ] **Step 5: Update docker-compose** — notifier gets DATABASE_URL

- [ ] **Step 6: Build all 3 services + test**

```bash
go build -o /tmp/api ./cmd/api/
go build -o /tmp/checker ./cmd/checker/
go build -o /tmp/notifier ./cmd/notifier/
go test ./... -count=1 -timeout=30s
```

- [ ] **Step 7: Commit**

```bash
git add cmd/ internal/config/ docker-compose.yml
git commit -m "feat: rewire cmd/ — notifier with DB, channel registry everywhere"
```

---

## Task 11: Cleanup + Tests

**Files:**
- Delete: old telegram `sender_test.go`
- Create: new adapter tests (telegram, webhook, channel registry)
- Create: `internal/app/alert_test.go`

- [ ] **Step 1: Rewrite telegram test** — test `Factory.CreateSender` + `sender.Send`
- [ ] **Step 2: Add webhook test** — mock HTTP server
- [ ] **Step 3: Add alert service test** — mock registry + mock channel repo
- [ ] **Step 4: Final verification**

```bash
go build ./...
go vet ./...
go test ./... -count=1 -timeout=30s
```

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "test: notification channel tests — telegram, webhook, alert service"
```

---

## Summary

11 tasks, hex arch bottom-up.

1. Domain (NotificationChannel, simplified AlertEvent)
2. Ports (ChannelRegistry, ChannelRepo, AlertSender.Send)
3. Database (migration + sqlc)
4. Channel registry + webhook adapter
5. Rewrite telegram + smtp
6. App (AlertService rewrite + CRUD + MonitoringService simplify)
7. Postgres (channel_repo)
8. NATS (simplified events)
9. HTTP + frontend (CRUD, schema-driven forms)
10. cmd/ (notifier gains DB, registry everywhere)
11. Tests

**Adding a new channel after this = 2 steps:**
1. `adapter/newchannel/sender.go` — implement `ChannelSenderFactory`
2. `cmd/*/main.go` — `channelReg.Register(domain.ChannelNew, "Label", newchannel.NewFactory())`

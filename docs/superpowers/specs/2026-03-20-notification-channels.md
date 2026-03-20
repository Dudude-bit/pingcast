# User-Configurable Notification Channels

## Overview

Replace hardcoded Telegram/Email alert factories with a user-configurable notification channel system. Users create channels (Telegram, Email, Webhook), bind them to monitors or use them globally. Channel registry pattern mirrors CheckerRegistry. AlertSender interface simplified to `Send(ctx, event)`.

## Domain

### New entity `NotificationChannel` in `domain/channel.go`:

```go
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

Config structs live in adapters:
- `adapter/telegram/` — `TelegramChannelConfig { ChatID int64 }`
- `adapter/smtp/` — `EmailChannelConfig { Address string }`
- `adapter/webhook/` — `WebhookChannelConfig { URL string, Headers map[string]string }`

### `AlertEvent` simplified in `domain/alert.go`:

```go
type AlertEvent struct {
    MonitorID     uuid.UUID
    UserID        uuid.UUID       // for fallback to global channels
    IncidentID    int64
    MonitorName   string
    MonitorTarget string
    Event         AlertEventType
    Cause         string
}
```

Removed: `TgChatID`, `Email`, `Plan` — routing is now via channel repo, not event data.

### `AlertSender` simplified in `port/alerter.go`:

```go
type AlertSender interface {
    Send(ctx context.Context, event *domain.AlertEvent) error
}
```

One method. Each adapter formats the message from AlertEvent and delivers to its preconfigured destination.

## Ports

### `port/channel.go` — Channel Registry:

```go
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
    Type   domain.ChannelType
    Label  string
    Schema ConfigSchema
}
```

Same pattern as `CheckerRegistry`. Adding a new channel = one adapter file + registration.

`ConfigSchema`, `ConfigField`, `Option` types are **reused** from `port/checker.go` — NOT redefined. Both checkers and channels share the same schema types for form rendering.

### `port/repository.go` — ChannelRepo:

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

`ListForMonitor` returns channels bound to a specific monitor. Empty result = caller falls back to all user channels.

## App — AlertService

```go
type AlertService struct {
    channels port.ChannelRepo
    registry port.ChannelRegistry
}

func NewAlertService(channels port.ChannelRepo, registry port.ChannelRegistry) *AlertService

// Handle delivers an alert to all relevant channels.
// Best-effort delivery: logs errors per channel but does NOT return error
// to avoid NATS retry causing duplicate notifications on already-delivered channels.
func (s *AlertService) Handle(ctx context.Context, event *domain.AlertEvent) error {
    // 1. Get channels for this monitor
    channels, _ := s.channels.ListForMonitor(ctx, event.MonitorID)

    // 2. If none bound → fall back to all user's channels
    if len(channels) == 0 {
        channels, _ = s.channels.ListByUserID(ctx, event.UserID)
    }

    // 3. Best-effort delivery to all enabled channels
    for _, ch := range channels {
        if !ch.IsEnabled {
            continue
        }
        factory, err := s.registry.Get(ch.Type)
        if err != nil {
            slog.Error("unknown channel type", "type", ch.Type, "channel_id", ch.ID)
            continue
        }
        sender, err := factory.CreateSender(ch.Config)
        if err != nil {
            slog.Error("failed to create sender", "channel_id", ch.ID, "error", err)
            continue
        }
        if err := sender.Send(ctx, event); err != nil {
            slog.Error("channel delivery failed", "channel_id", ch.ID, "type", ch.Type, "error", err)
            // continue to next channel — don't abort on single failure
        }
    }
    return nil // always ack — no retry duplication
}

// --- Channel CRUD ---

type CreateChannelInput struct {
    Name   string
    Type   domain.ChannelType
    Config json.RawMessage
}

func (s *AlertService) CreateChannel(ctx context.Context, userID uuid.UUID, input CreateChannelInput) (*domain.NotificationChannel, error) {
    if err := s.registry.ValidateConfig(input.Type, input.Config); err != nil {
        return nil, fmt.Errorf("invalid channel config: %w", err)
    }
    ch := &domain.NotificationChannel{
        UserID: userID,
        Name:   input.Name,
        Type:   input.Type,
        Config: input.Config,
    }
    return s.channels.Create(ctx, ch)
}

func (s *AlertService) UpdateChannel(ctx context.Context, userID, channelID uuid.UUID, name string, config json.RawMessage, isEnabled bool) (*domain.NotificationChannel, error) {
    ch, err := s.channels.GetByID(ctx, channelID)
    if err != nil || ch.UserID != userID {
        return nil, fmt.Errorf("channel not found")
    }
    if config != nil {
        if err := s.registry.ValidateConfig(ch.Type, config); err != nil {
            return nil, fmt.Errorf("invalid channel config: %w", err)
        }
        ch.Config = config
    }
    ch.Name = name
    ch.IsEnabled = isEnabled
    if err := s.channels.Update(ctx, ch); err != nil {
        return nil, err
    }
    return ch, nil
}

func (s *AlertService) DeleteChannel(ctx context.Context, userID, channelID uuid.UUID) error {
    return s.channels.Delete(ctx, channelID, userID)
}

func (s *AlertService) ListChannels(ctx context.Context, userID uuid.UUID) ([]domain.NotificationChannel, error) {
    return s.channels.ListByUserID(ctx, userID)
}

// BindChannel binds a channel to a monitor. Both must belong to the same user.
func (s *AlertService) BindChannel(ctx context.Context, userID, monitorID, channelID uuid.UUID, monitors port.MonitorRepo) error {
    // Verify ownership of monitor
    mon, err := monitors.GetByID(ctx, monitorID)
    if err != nil || mon.UserID != userID {
        return fmt.Errorf("monitor not found")
    }
    // Verify ownership of channel
    ch, err := s.channels.GetByID(ctx, channelID)
    if err != nil || ch.UserID != userID {
        return fmt.Errorf("channel not found")
    }
    return s.channels.BindToMonitor(ctx, monitorID, channelID)
}

func (s *AlertService) UnbindChannel(ctx context.Context, userID, monitorID, channelID uuid.UUID, monitors port.MonitorRepo) error {
    mon, err := monitors.GetByID(ctx, monitorID)
    if err != nil || mon.UserID != userID {
        return fmt.Errorf("monitor not found")
    }
    return s.channels.UnbindFromMonitor(ctx, monitorID, channelID)
}
```

**Design decisions:**
- **Best-effort delivery** — logs per-channel errors, always acks NATS message. No retry duplication.
- **Channel CRUD with authorization** — `CreateChannel`, `UpdateChannel`, `DeleteChannel` verify `userID` ownership.
- **`BindChannel`/`UnbindChannel`** — verify ownership of BOTH monitor and channel via `userID`.
- **Type immutable** — `UpdateChannel` does not allow changing `Type` (would invalidate config). Explicit: not in params.

### Changes to `MonitoringService.publishAlert`:

No longer reads user data for `TgChatID`/`Email`/`Plan`. Simplified:

```go
func (s *MonitoringService) publishAlert(ctx context.Context, monitor *domain.Monitor, eventType domain.AlertEventType, cause string, incidentID int64) error {
    event := &domain.AlertEvent{
        MonitorID:     monitor.ID,
        UserID:        monitor.UserID,
        IncidentID:    incidentID,
        MonitorName:   monitor.Name,
        MonitorTarget: s.registry.Target(monitor.Type, monitor.CheckConfig),
        Event:         eventType,
        Cause:         cause,
    }
    return s.alerts.PublishAlert(ctx, event)
}
```

`MonitoringService` no longer depends on `port.UserRepo` for alert publishing. The `users` field in `MonitoringService` can be removed if it was only used for alert data — but it's still needed for `GetStatusPage` (user plan check) and `CreateMonitor` validation.

### Deleted from `app/alert.go`:
- `TelegramFactory`, `EmailFactory` types
- Old `NewAlertService(tg TelegramFactory, email EmailFactory)`

## Adapters

### `adapter/telegram/sender.go`:

```go
type TelegramChannelConfig struct {
    ChatID int64 `json:"chat_id"`
}

type Factory struct {
    token string
}

func NewFactory(token string) *Factory

func (f *Factory) CreateSender(config json.RawMessage) (port.AlertSender, error) {
    var cfg TelegramChannelConfig
    if err := json.Unmarshal(config, &cfg); err != nil {
        return nil, err
    }
    return &sender{token: f.token, chatID: cfg.ChatID}, nil
}

func (f *Factory) ValidateConfig(raw json.RawMessage) error {
    var cfg TelegramChannelConfig
    if err := json.Unmarshal(raw, &cfg); err != nil {
        return err
    }
    if cfg.ChatID == 0 {
        return fmt.Errorf("chat_id required")
    }
    return nil
}

func (f *Factory) ConfigSchema() port.ConfigSchema {
    return port.ConfigSchema{Fields: []port.ConfigField{
        {Name: "chat_id", Label: "Chat ID", Type: "number", Required: true, Placeholder: "Get from @PingCastBot /start"},
    }}
}

type sender struct {
    token  string
    chatID int64
}

func (s *sender) Send(ctx context.Context, event *domain.AlertEvent) error {
    var text string
    switch event.Event {
    case domain.AlertDown:
        text = fmt.Sprintf("🔴 *%s* is DOWN\n\nTarget: `%s`\nCause: %s", event.MonitorName, event.MonitorTarget, event.Cause)
    case domain.AlertUp:
        text = fmt.Sprintf("🟢 *%s* is back UP\n\nTarget: `%s`", event.MonitorName, event.MonitorTarget)
    }
    // POST to Telegram API with token + chatID + text
}
```

### `adapter/smtp/sender.go`:

Same pattern. `EmailChannelConfig { Address string }`. `sender.Send` formats email from event.

### `adapter/webhook/sender.go` — NEW:

```go
type WebhookChannelConfig struct {
    URL     string            `json:"url"`
    Headers map[string]string `json:"headers,omitempty"`
}

type Factory struct {
    client *http.Client // with 10s timeout
}

func NewFactory() *Factory {
    return &Factory{
        client: &http.Client{Timeout: 10 * time.Second},
    }
}

func (f *Factory) CreateSender(config json.RawMessage) (port.AlertSender, error) {
    var cfg WebhookChannelConfig
    if err := json.Unmarshal(config, &cfg); err != nil {
        return nil, err
    }
    return &sender{client: f.client, url: cfg.URL, headers: cfg.Headers}, nil
}

func (f *Factory) ValidateConfig(raw json.RawMessage) error {
    var cfg WebhookChannelConfig
    if err := json.Unmarshal(raw, &cfg); err != nil {
        return err
    }
    if cfg.URL == "" {
        return fmt.Errorf("url required")
    }
    u, err := url.Parse(cfg.URL)
    if err != nil {
        return fmt.Errorf("invalid url: %w", err)
    }
    if u.Scheme != "http" && u.Scheme != "https" {
        return fmt.Errorf("url must use http or https scheme")
    }
    // Block private/internal IP ranges (SSRF protection)
    // Resolved at send time, not validation — but reject obvious localhost
    if u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1" {
        return fmt.Errorf("webhook url cannot point to localhost")
    }
    return nil
}

func (f *Factory) ConfigSchema() port.ConfigSchema {
    return port.ConfigSchema{Fields: []port.ConfigField{
        {Name: "url", Label: "Webhook URL", Type: "text", Required: true, Placeholder: "https://hooks.slack.com/..."},
        {Name: "headers", Label: "Headers (JSON)", Type: "text", Placeholder: `{"Authorization": "Bearer ..."}`},
    }}
}

type sender struct {
    client  *http.Client
    url     string
    headers map[string]string
}

func (s *sender) Send(ctx context.Context, event *domain.AlertEvent) error {
    body, _ := json.Marshal(event)
    req, _ := http.NewRequestWithContext(ctx, "POST", s.url, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    for k, v := range s.headers {
        req.Header.Set(k, v)
    }
    resp, err := s.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode >= 400 {
        return fmt.Errorf("webhook returned status %d", resp.StatusCode)
    }
    return nil
}
```

### `adapter/channel/registry.go` — NEW:

```go
type registryEntry struct {
    label   string
    factory port.ChannelSenderFactory
}

type Registry struct {
    entries map[domain.ChannelType]registryEntry
}

func NewRegistry() *Registry
func (r *Registry) Register(t domain.ChannelType, label string, f port.ChannelSenderFactory)
func (r *Registry) Get(t domain.ChannelType) (port.ChannelSenderFactory, error)
func (r *Registry) Types() []port.ChannelTypeInfo
func (r *Registry) ValidateConfig(t domain.ChannelType, raw json.RawMessage) error
```

`var _ port.ChannelRegistry = (*Registry)(nil)`

## Database

### Migration `008_create_notification_channels.sql`:

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

### sqlc queries `queries/channels.sql`:

```sql
-- name: CreateChannel :one
INSERT INTO notification_channels (user_id, name, type, config)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, name, type, config, is_enabled, created_at;

-- name: GetChannelByID :one
SELECT id, user_id, name, type, config, is_enabled, created_at
FROM notification_channels WHERE id = $1;

-- name: ListChannelsByUserID :many
SELECT id, user_id, name, type, config, is_enabled, created_at
FROM notification_channels WHERE user_id = $1 ORDER BY created_at;

-- name: ListChannelsForMonitor :many
SELECT c.id, c.user_id, c.name, c.type, c.config, c.is_enabled, c.created_at
FROM notification_channels c
JOIN monitor_channels mc ON c.id = mc.channel_id
WHERE mc.monitor_id = $1
ORDER BY c.name;

-- name: UpdateChannel :exec
UPDATE notification_channels
SET name = $2, config = $3, is_enabled = $4
WHERE id = $1 AND user_id = $5;

-- name: DeleteChannel :exec
DELETE FROM notification_channels WHERE id = $1 AND user_id = $2;

-- name: BindChannelToMonitor :exec
INSERT INTO monitor_channels (monitor_id, channel_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: UnbindChannelFromMonitor :exec
DELETE FROM monitor_channels WHERE monitor_id = $1 AND channel_id = $2;

-- name: ListMonitorChannelIDs :many
SELECT channel_id FROM monitor_channels WHERE monitor_id = $1;
```

## Notifier Service Changes

Notifier gains PostgreSQL access (was NATS-only):

```
cmd/notifier/main.go:
    DB → postgres repos (ChannelRepo — read only)
    NATS → AlertSubscriber
    Channel registry (Telegram factory, SMTP factory, Webhook factory)
    AlertService(channelRepo, channelRegistry)
    Subscribe → alertService.Handle
```

`docker-compose.yml` — notifier gets `DATABASE_URL` env var.
`config.LoadNotifier()` — add `DatabaseURL` (required now — for notification channel lookup).

## Deployment Order

**Deploy notifier first**, then checker, then API. Reason: the new notifier reads channels from DB and ignores old `TgChatID`/`Email` fields in events (they're simply absent). Old publisher still sends events — new notifier handles them correctly by falling back to user's global channels. If API is deployed first, it sends events without routing data while old notifier expects it → silent alert loss.

## Composition Root Changes

### `cmd/notifier/main.go`:

```go
// DB (read-only for channel lookup)
pool, _ := database.Connect(ctx, cfg.DatabaseURL)
queries := sqlcgen.New(pool)
channelRepo := postgres.NewChannelRepo(queries)

// Channel registry
channelReg := channel.NewRegistry()
if cfg.TelegramToken != "" {
    channelReg.Register(domain.ChannelTelegram, "Telegram", telegram.NewFactory(cfg.TelegramToken))
}
if cfg.SMTPHost != "" {
    channelReg.Register(domain.ChannelEmail, "Email", smtpadapter.NewFactory(...))
}
channelReg.Register(domain.ChannelWebhook, "Webhook", webhook.NewFactory())

// Alert service
alertSvc := app.NewAlertService(channelRepo, channelReg)

// Subscribe
alertSub.Subscribe(ctx, alertSvc.Handle)
```

### `cmd/api/main.go`:

Same channel registry — needed for channel CRUD validation + `/api/channel-types`.

## API Endpoints

```
GET    /api/channel-types                      — available types with ConfigSchema
GET    /api/channels                           — user's channels
POST   /api/channels                           — create channel
PUT    /api/channels/{id}                      — update channel
DELETE /api/channels/{id}                      — delete channel
POST   /api/monitors/{id}/channels             — bind channel
DELETE /api/monitors/{id}/channels/{channelId} — unbind
```

OpenAPI schema additions: `NotificationChannel`, `CreateChannelRequest`, `UpdateChannelRequest`, `ChannelTypeInfo`.

## Frontend

- `/channels` page — list of user's channels with enable/disable toggle, delete
- `/channels/new` — create form with type selector + schema-driven config fields (same HTMX pattern as monitors)
- Monitor form — new section "Notification Channels" with checkboxes for user's channels. Unchecked = global behavior.
- Dashboard info bar — shows channel count
- Reuse `monitor_config_fields.html` template for channel config fields (or rename to `config_fields.html`)

## Files Changed

### New files:
- `internal/domain/channel.go`
- `internal/port/channel.go`
- `internal/adapter/channel/registry.go`
- `internal/adapter/webhook/sender.go`
- `internal/adapter/postgres/channel_repo.go`
- `internal/database/migrations/008_create_notification_channels.sql`
- `internal/sqlc/queries/channels.sql`
- `internal/web/templates/channels.html`
- `internal/web/templates/channel_form.html`

### Modified files:
- `internal/domain/alert.go` — remove TgChatID/Email/Plan, add UserID
- `internal/port/alerter.go` — `Send(ctx, event)` replaces `NotifyDown`/`NotifyUp`
- `internal/app/alert.go` — rewrite with ChannelRepo + ChannelRegistry
- `internal/app/monitoring.go` — simplify publishAlert (no user lookup for routing)
- `internal/adapter/telegram/sender.go` — implement ChannelSenderFactory + Send
- `internal/adapter/smtp/sender.go` — same
- `internal/adapter/http/server.go` — channel CRUD handlers
- `internal/adapter/http/pages.go` — channel pages + monitor form channel checkboxes
- `internal/adapter/http/setup.go` — new routes
- `internal/adapter/nats/publisher.go` — updated AlertEvent serialization (no TgChatID/Email)
- `internal/adapter/nats/subscriber.go` — updated AlertEvent deserialization
- `internal/config/config.go` — LoadNotifier gets DatabaseURL
- `cmd/notifier/main.go` — DB access + channel registry + AlertService
- `cmd/api/main.go` — channel registry + channel CRUD
- `api/openapi.yaml` — channel endpoints + schemas
- `docker-compose.yml` — notifier gets DATABASE_URL

### Deleted:
- Old `TelegramFactory`/`EmailFactory` types
- `ForChat`/`ForRecipient` factory methods on telegram/smtp senders
- `sender_test.go` for telegram (needs rewrite for new interface)

### Unchanged:
- `internal/domain/monitor.go`, `internal/domain/user.go`, `internal/domain/incident.go`
- `internal/adapter/checker/` — all checker code
- `cmd/checker/main.go`

## Adding a new channel type:
1. `adapter/newchannel/sender.go` — implement `ChannelSenderFactory` with `CreateSender`, `ValidateConfig`, `ConfigSchema`
2. `cmd/*/main.go` — `channelReg.Register(domain.ChannelNewType, "Label", newchannel.NewFactory())`
3. No domain/port/frontend changes — form renders from ConfigSchema

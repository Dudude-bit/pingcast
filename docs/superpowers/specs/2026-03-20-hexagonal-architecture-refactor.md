# PingCast Hexagonal Architecture Refactor

## Overview

Refactor PingCast from a flat package structure with tight coupling to sqlc/pgtype types into a strict Hexagonal Architecture (Ports & Adapters). The domain layer becomes pure Go with zero external dependencies. Business logic moves from closures and handlers into testable application services.

## Problems Solved

1. **No domain layer** — business logic (incident state machine, consecutive failures, plan limits) scattered across `cmd/checker/main.go` closure and `handler/server.go`
2. **Handler → sqlc coupling** — HTTP handlers work directly with `gen.Queries` and sqlc types (`gen.Monitor`, `pgtype.Int8`)
3. **Auth depends on sqlc** — `auth.Service` accepts `*gen.Queries`, returns sqlc row types
4. **Untestable check handler** — 80-line closure in `cmd/checker/main.go`
5. **No repository interfaces** — impossible to substitute storage for tests
6. **Duplicate logic** — uptime queries, monitor validation repeated across handlers

## Dependency Rule

```
domain ← port ← app ← adapter ← cmd/
```

- **domain** imports only stdlib + `github.com/google/uuid`
- **port** imports only domain
- **app** imports domain + port
- **adapter** imports domain + port + external libraries (sqlc, pgtype, fiber, nats, etc.)
- **cmd/** wires everything via dependency injection

No layer may import a layer to its right. Adapters NEVER leak into app or domain.

## Package Structure

```
internal/
├── domain/
│   ├── monitor.go          # Monitor, MonitorStatus, HTTPMethod, CheckResult
│   ├── user.go             # User, Plan, helper methods (MonitorLimit, MinInterval, CanUseEmail)
│   ├── incident.go         # Incident, IsResolved
│   └── alert.go            # AlertEvent, AlertEventType
│
├── port/
│   ├── repository.go       # UserRepo, SessionRepo, MonitorRepo, CheckResultRepo, IncidentRepo
│   ├── eventbus.go         # MonitorEventPublisher, AlertEventPublisher, MonitorEventSubscriber, AlertEventSubscriber
│   └── alerter.go          # AlertSender
│
├── app/
│   ├── auth.go             # AuthService (register, login, session validation)
│   ├── monitoring.go       # MonitoringService (CRUD, check processing, incidents, status page)
│   └── alert.go            # AlertService (dispatch notifications)
│
├── adapter/
│   ├── postgres/
│   │   ├── user_repo.go
│   │   ├── session_repo.go
│   │   ├── monitor_repo.go
│   │   ├── check_result_repo.go
│   │   ├── incident_repo.go
│   │   └── mapper.go       # sqlc types ↔ domain types (pgtype.Int8 → *int64, etc.)
│   ├── nats/
│   │   ├── publisher.go    # MonitorEventPublisher, AlertEventPublisher
│   │   └── subscriber.go   # MonitorEventSubscriber, AlertEventSubscriber
│   ├── telegram/
│   │   └── sender.go       # AlertSender for Telegram
│   ├── smtp/
│   │   └── sender.go       # AlertSender for email
│   └── http/
│       ├── server.go       # oapi-codegen ServerInterface (thin: parse → app service → respond)
│       ├── pages.go        # HTML page handlers
│       ├── webhook.go      # Lemon Squeezy + Telegram bot
│       ├── middleware.go    # Auth middleware (Fiber)
│       ├── setup.go        # Fiber app wiring
│       └── ratelimit.go    # Login rate limiter
│
├── config/                 # Per-service configs (unchanged)
├── database/               # Connection + migrations (unchanged)
├── sqlc/                   # sqlc config + generated code (unchanged)
└── web/                    # Templates + static (unchanged)
```

## Domain Layer (`internal/domain/`)

Zero external dependencies except stdlib and uuid. All types are pure Go structs with typed enums.

### `domain/user.go`

```go
type Plan string
const (
    PlanFree Plan = "free"
    PlanPro  Plan = "pro"
)

type User struct {
    ID        uuid.UUID
    Email     string
    Slug      string
    Plan      Plan
    TgChatID  *int64
    CreatedAt time.Time
}

func (u User) MonitorLimit() int   // PlanFree: 5, PlanPro: 50
func (u User) MinInterval() int    // PlanFree: 300, PlanPro: 30
func (u User) CanUseEmail() bool   // plan == PlanPro
```

### `domain/monitor.go`

```go
type MonitorStatus string
const (
    StatusUp      MonitorStatus = "up"
    StatusDown    MonitorStatus = "down"
    StatusUnknown MonitorStatus = "unknown"
)

type HTTPMethod string
const (
    MethodGET  HTTPMethod = "GET"
    MethodPOST HTTPMethod = "POST"
)

type Monitor struct {
    ID                 uuid.UUID
    UserID             uuid.UUID
    Name               string
    URL                string
    Method             HTTPMethod
    IntervalSeconds    int
    ExpectedStatus     int
    Keyword            *string
    AlertAfterFailures int
    IsPaused           bool
    IsPublic           bool
    CurrentStatus      MonitorStatus
    CreatedAt          time.Time
}

type CheckResult struct {
    ID             int64
    MonitorID      uuid.UUID
    Status         MonitorStatus
    StatusCode     *int
    ResponseTimeMs int
    ErrorMessage   *string
    CheckedAt      time.Time
}
```

### `domain/incident.go`

```go
type Incident struct {
    ID         int64
    MonitorID  uuid.UUID
    StartedAt  time.Time
    ResolvedAt *time.Time
    Cause      string
}

func (i Incident) IsResolved() bool { return i.ResolvedAt != nil }
```

### `domain/alert.go`

```go
type MonitorAction string
const (
    ActionCreate MonitorAction = "create"
    ActionUpdate MonitorAction = "update"
    ActionDelete MonitorAction = "delete"
    ActionPause  MonitorAction = "pause"
    ActionResume MonitorAction = "resume"
)

type AlertEventType string
const (
    AlertDown AlertEventType = "down"
    AlertUp   AlertEventType = "up"
)

type AlertEvent struct {
    MonitorID   uuid.UUID
    IncidentID  int64
    MonitorName string
    MonitorURL  string
    Event       AlertEventType
    Cause       string
    TgChatID    *int64
    Email       string
    Plan        Plan
}
```

## Ports (`internal/port/`)

Interfaces defined as contracts. Depend only on domain types.

### `port/repository.go`

```go
type UserRepo interface {
    Create(ctx context.Context, email, slug, passwordHash string) (*domain.User, error)
    GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
    GetByEmail(ctx context.Context, email string) (user *domain.User, passwordHash string, err error)
    GetBySlug(ctx context.Context, slug string) (*domain.User, error)
    UpdatePlan(ctx context.Context, id uuid.UUID, plan domain.Plan) error
    UpdateTelegramChatID(ctx context.Context, id uuid.UUID, chatID int64) error
    UpdateLemonSqueezy(ctx context.Context, id uuid.UUID, customerID, subscriptionID string) error
}

type SessionRepo interface {
    Create(ctx context.Context, sessionID string, userID uuid.UUID, expiresAt time.Time) error
    GetUserID(ctx context.Context, sessionID string) (uuid.UUID, error)
    Touch(ctx context.Context, sessionID string, expiresAt time.Time) error
    Delete(ctx context.Context, sessionID string) error
    DeleteExpired(ctx context.Context) (int64, error)
}

// Token generation (crypto/rand) and session duration (30 days) remain in app/auth.go.
// SessionRepo is a pure storage adapter — it does not generate tokens or compute expiry.

type MonitorRepo interface {
    Create(ctx context.Context, m *domain.Monitor) (*domain.Monitor, error)
    GetByID(ctx context.Context, id uuid.UUID) (*domain.Monitor, error)
    ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Monitor, error)
    ListPublicBySlug(ctx context.Context, slug string) ([]domain.Monitor, error)
    ListActive(ctx context.Context) ([]domain.Monitor, error)
    CountByUserID(ctx context.Context, userID uuid.UUID) (int, error)
    Update(ctx context.Context, m *domain.Monitor) error
    UpdateStatus(ctx context.Context, id uuid.UUID, status domain.MonitorStatus) error
    Delete(ctx context.Context, id, userID uuid.UUID) error
}

type CheckResultRepo interface {
    Insert(ctx context.Context, cr *domain.CheckResult) error
    GetUptime(ctx context.Context, monitorID uuid.UUID, since time.Time) (float64, error)
    ConsecutiveFailures(ctx context.Context, monitorID uuid.UUID) (int, error)
    DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

type IncidentRepo interface {
    Create(ctx context.Context, monitorID uuid.UUID, cause string) (*domain.Incident, error)
    Resolve(ctx context.Context, id int64, resolvedAt time.Time) error
    GetOpen(ctx context.Context, monitorID uuid.UUID) (*domain.Incident, error)
    IsInCooldown(ctx context.Context, monitorID uuid.UUID) (bool, error)
    ListByMonitorID(ctx context.Context, monitorID uuid.UUID, limit int) ([]domain.Incident, error)
}
```

### `port/eventbus.go`

```go
type MonitorEventPublisher interface {
    PublishMonitorChanged(ctx context.Context, action domain.MonitorAction, monitorID uuid.UUID, monitor *domain.Monitor) error
}

type AlertEventPublisher interface {
    PublishAlert(ctx context.Context, event *domain.AlertEvent) error
}

type MonitorEventSubscriber interface {
    Subscribe(ctx context.Context, handler func(ctx context.Context, action domain.MonitorAction, monitorID uuid.UUID, monitor *domain.Monitor) error) error
    Stop()
}

type AlertEventSubscriber interface {
    Subscribe(ctx context.Context, handler func(ctx context.Context, event *domain.AlertEvent) error) error
    Stop()
}
```

### `port/alerter.go`

```go
type AlertSender interface {
    NotifyDown(ctx context.Context, monitorName, monitorURL, cause string) error
    NotifyUp(ctx context.Context, monitorName, monitorURL string) error
}
```

## Application Services (`internal/app/`)

### `app/auth.go`

```go
type AuthService struct {
    users    port.UserRepo
    sessions port.SessionRepo
}

func NewAuthService(users port.UserRepo, sessions port.SessionRepo) *AuthService

func (s *AuthService) Register(ctx context.Context, email, slug, password string) (*domain.User, string, error)
    // 1. ValidateSlug(slug)
    // 2. ValidatePassword(password)
    // 3. HashPassword(password)
    // 4. users.Create(ctx, email, slug, hash)
    // 5. sessions.Create(ctx, user.ID)
    // Returns: user, sessionID, error

func (s *AuthService) Login(ctx context.Context, email, password string) (*domain.User, string, error)
    // 1. users.GetByEmail(ctx, email) → user, hash
    // 2. CheckPassword(hash, password)
    // 3. sessions.Create(ctx, user.ID)

func (s *AuthService) ValidateSession(ctx context.Context, sessionID string) (*domain.User, error)
    // 1. sessions.GetUserID(ctx, sessionID)
    // 2. users.GetByID(ctx, userID)
    // 3. sessions.Touch(ctx, sessionID)

func (s *AuthService) Logout(ctx context.Context, sessionID string) error

func (s *AuthService) LinkTelegram(ctx context.Context, userID uuid.UUID, chatID int64) error
    // users.UpdateTelegramChatID

func (s *AuthService) UpgradeToPro(ctx context.Context, userID uuid.UUID, customerID, subscriptionID string) error
    // users.UpdatePlan(ctx, id, PlanPro) + users.UpdateLemonSqueezy(ctx, id, customerID, subscriptionID)

func (s *AuthService) DowngradeToFree(ctx context.Context, userID uuid.UUID) error
    // users.UpdatePlan(ctx, id, PlanFree)

// Pure functions (not methods):
func ValidateSlug(slug string) error
func ValidatePassword(password string) error  // len >= 8
func HashPassword(password string) (string, error)
func CheckPassword(hash, password string) bool
```

### `app/monitoring.go`

```go
type MonitoringService struct {
    monitors     port.MonitorRepo
    checkResults port.CheckResultRepo
    incidents    port.IncidentRepo
    users        port.UserRepo
    alerts       port.AlertEventPublisher
}

func NewMonitoringService(...) *MonitoringService

// --- CRUD ---

type CreateMonitorInput struct {
    Name               string
    URL                string
    Method             domain.HTTPMethod
    IntervalSeconds    int
    ExpectedStatus     int
    Keyword            *string
    AlertAfterFailures int
    IsPublic           bool
}

func (s *MonitoringService) CreateMonitor(ctx context.Context, user *domain.User, input CreateMonitorInput) (*domain.Monitor, error)
    // 1. Check user.MonitorLimit() vs monitors.CountByUserID
    // 2. Enforce user.MinInterval()
    // 3. monitors.Create

type UpdateMonitorInput struct {
    Name               *string
    URL                *string
    Method             *domain.HTTPMethod
    IntervalSeconds    *int
    ExpectedStatus     *int
    Keyword            *string
    AlertAfterFailures *int
    IsPaused           *bool
    IsPublic           *bool
}

func (s *MonitoringService) UpdateMonitor(ctx context.Context, user *domain.User, id uuid.UUID, input UpdateMonitorInput) (*domain.Monitor, error)
    // 1. monitors.GetByID → existing
    // 2. Merge non-nil fields from input into existing
    // 3. Enforce user.MinInterval()
    // 4. monitors.Update

func (s *MonitoringService) DeleteMonitor(ctx context.Context, userID, monitorID uuid.UUID) error

func (s *MonitoringService) TogglePause(ctx context.Context, user *domain.User, monitorID uuid.UUID) (*domain.Monitor, error)

// --- Check processing (core business logic) ---

func (s *MonitoringService) ProcessCheckResult(ctx context.Context, monitor *domain.Monitor, result *domain.CheckResult) error
    // 1. checkResults.Insert(result)
    // 2. monitors.GetByID → previousStatus
    // 3. monitors.UpdateStatus(result.Status)
    // 4. If no transition → return
    // 5. If down:
    //    a. checkResults.ConsecutiveFailures
    //    b. if >= threshold: incidents.IsInCooldown
    //    c. if not cooldown: incidents.Create
    //    d. users.GetByID → build AlertEvent
    //    e. alerts.PublishAlert
    // 6. If up (was down):
    //    a. incidents.GetOpen → Resolve
    //    b. Build AlertEvent → alerts.PublishAlert

// --- Query helpers (deduplicated) ---

type MonitorDetail struct {
    Monitor   domain.Monitor
    Uptime24h float64
    Uptime7d  float64
    Uptime30d float64
    Incidents []domain.Incident
}

func (s *MonitoringService) GetMonitorDetail(ctx context.Context, monitorID uuid.UUID) (*MonitorDetail, error)

type StatusPageData struct {
    Slug         string
    AllUp        bool
    ShowBranding bool
    Monitors     []StatusMonitor
    Incidents    []domain.Incident
}

type StatusMonitor struct {
    Name          string
    CurrentStatus domain.MonitorStatus
    Uptime90d     float64
}

func (s *MonitoringService) GetStatusPage(ctx context.Context, slug string) (*StatusPageData, error)

func (s *MonitoringService) ListMonitorsWithUptime(ctx context.Context, userID uuid.UUID) ([]MonitorWithUptime, error)

type MonitorWithUptime struct {
    Monitor domain.Monitor
    Uptime  float64
}
```

### `app/alert.go`

```go
type TelegramFactory func(chatID int64) port.AlertSender
type EmailFactory func(to string) port.AlertSender

type AlertService struct {
    telegramFactory TelegramFactory
    emailFactory    EmailFactory
}

func NewAlertService(tg TelegramFactory, email EmailFactory) *AlertService

func (s *AlertService) Handle(ctx context.Context, event *domain.AlertEvent) error
    // 1. Build []port.AlertSender from event:
    //    - if telegramFactory != nil && event.TgChatID != nil → append telegramFactory(*event.TgChatID)
    //    - if emailFactory != nil && event.Plan == PlanPro && event.Email != "" → append emailFactory(event.Email)
    // 2. For each sender: NotifyDown or NotifyUp
    // 3. Return first error (nack triggers NATS retry)

// Factories are nil-safe: if Telegram/SMTP is not configured, the corresponding factory is nil and skipped.
```

## Adapters

### `adapter/postgres/`

Each file implements one port interface. All use `*gen.Queries` internally.

`mapper.go` centralizes all type conversions:
- `pgtype.Int8` → `*int64`
- `pgtype.Timestamptz` → `*time.Time`
- `gen.Monitor` → `*domain.Monitor`
- `gen.GetUserByIDRow` → `*domain.User`
- Domain enums (`domain.Plan`, `domain.MonitorStatus`) ↔ strings

Repository methods: call sqlc query → mapper → return domain type.

### `adapter/nats/`

`publisher.go`:
- `MonitorPublisher` implements `port.MonitorEventPublisher`
- `AlertPublisher` implements `port.AlertEventPublisher`
- JSON marshal domain types → js.Publish

`subscriber.go`:
- `MonitorSubscriber` implements `port.MonitorEventSubscriber`
- `AlertSubscriber` implements `port.AlertEventSubscriber`
- js.CreateOrUpdateConsumer → Consume → JSON unmarshal → handler callback
- Ack on success, Nack on error

`client.go`:
- `Connect(url) (*nats.Conn, error)` — connection with reconnect options (moved from current `internal/nats/client.go`)
- `SetupStreams(ctx, js)` — creates MONITORS and ALERTS streams (idempotent, called from all cmd/ on startup)

`events.go` is deleted — domain types (`AlertEvent`, `MonitorAction`) replace the NATS-specific event structs. JSON serialization of domain types happens inside the adapter.

### `adapter/telegram/` and `adapter/smtp/`

Current `TelegramSender` and `EmailSender` code moves here. They don't implement `port.AlertSender` directly (different signatures: need chatID/email). Instead, factory functions create bound senders:

```go
// adapter/telegram/sender.go
type Sender struct { ... }
func New(token string) *Sender
func (s *Sender) ForChat(chatID int64) port.AlertSender  // returns bound AlertSender
```

```go
// adapter/smtp/sender.go
type Sender struct { ... }
func New(host, port, user, pass, from) *Sender
func (s *Sender) ForRecipient(email string) port.AlertSender  // returns bound AlertSender
```

### `adapter/http/`

Current handler code moves here. Key change: handlers call app services instead of sqlc directly.

```go
type Server struct {
    auth       *app.AuthService
    monitoring *app.MonitoringService
    events     port.MonitorEventPublisher
    rateLimiter *RateLimiter
}
```

Handlers become thin: parse request → call service → map domain type to API type → respond.

`pages.go`: HTML page handlers. Same structure as `Server`:

```go
type PageHandler struct {
    auth       *app.AuthService
    monitoring *app.MonitoringService
    templates  map[string]*template.Template
}
```

Handlers call app services, receive domain types, pass them to templates:
- `Dashboard` → `monitoring.ListMonitorsWithUptime(ctx, user.ID)`
- `MonitorDetail` → `monitoring.GetMonitorDetail(ctx, monitorID)`
- `StatusPage` → `monitoring.GetStatusPage(ctx, slug)`
- `MonitorNewForm` / `MonitorCreate` → `monitoring.CreateMonitor(ctx, user, input)`
- `LoginSubmit` → `auth.Login(ctx, email, password)`

No sqlc types ever reach templates — only domain types.

`middleware.go`: auth middleware calls `auth.ValidateSession`, stores `*domain.User` in fiber.Locals.

`ratelimit.go`: moved from `internal/auth/` — it's an HTTP concern, not domain.

## What Gets Deleted

| Old path | New location |
|---|---|
| `internal/auth/service.go` | `internal/app/auth.go` |
| `internal/auth/middleware.go` | `internal/adapter/http/middleware.go` |
| `internal/auth/ratelimit.go` | `internal/adapter/http/ratelimit.go` |
| `internal/handler/server.go` | `internal/adapter/http/server.go` |
| `internal/handler/pages.go` | `internal/adapter/http/pages.go` |
| `internal/handler/webhook.go` | `internal/adapter/http/webhook.go` |
| `internal/handler/setup.go` | `internal/adapter/http/setup.go` |
| `internal/notifier/telegram.go` | `internal/adapter/telegram/sender.go` |
| `internal/notifier/email.go` | `internal/adapter/smtp/sender.go` |
| `internal/notifier/sender.go` | `internal/port/alerter.go` |
| `internal/nats/client.go` | `internal/adapter/nats/client.go` |
| `internal/nats/events.go` | Deleted (domain types replace it) |
| `internal/checker/client.go` | Stays (checker is infrastructure, used by adapter/checker or cmd/) |
| `internal/checker/scheduler.go` | Stays |
| `internal/checker/worker.go` | Stays |
| `internal/checker/hostlimit.go` | Stays |

## What Stays Unchanged

- `internal/config/` — per-service configs
- `internal/database/` — connection + migrations
- `internal/sqlc/` — sqlc config + generated code (used only by adapter/postgres/)
- `internal/web/` — templates + static
- `internal/checker/` — HTTP client, scheduler, worker pool (infrastructure, not domain)
- `api/openapi.yaml` — OpenAPI spec
- `cmd/api/`, `cmd/checker/`, `cmd/notifier/` — rewired to use app services

## Data Retention

Retention cleanup (delete old check_results + expired sessions) stays in `cmd/checker/main.go` as a composition-root goroutine. It calls repo methods directly (`checkResultRepo.DeleteOlderThan`, `sessionRepo.DeleteExpired`). This is acceptable — it's a cron-like infrastructure concern, not domain logic. No app service needed.

## cmd/ Changes

Each `cmd/*/main.go` becomes a composition root:

**`cmd/api/main.go`:**
```go
// Create adapters
userRepo := postgres.NewUserRepo(queries)
sessionRepo := postgres.NewSessionRepo(queries)
monitorRepo := postgres.NewMonitorRepo(queries)
// ...
monitorPublisher := natspub.NewMonitorPublisher(js)

// Create app services
authService := app.NewAuthService(userRepo, sessionRepo)
monitoringService := app.NewMonitoringService(monitorRepo, checkResultRepo, incidentRepo, userRepo, alertPublisher)

// Create HTTP adapter
server := httpadapter.NewServer(authService, monitoringService, monitorPublisher, rateLimiter)
```

**`cmd/checker/main.go`:**
```go
// Create adapters
monitorRepo := postgres.NewMonitorRepo(queries)
// ...
alertPublisher := natspub.NewAlertPublisher(js)
monitorSubscriber := natssub.NewMonitorSubscriber(js)

// Create app service
monitoringService := app.NewMonitoringService(...)

// Check handler uses app service
checkHandler := func(ctx, monitor, result) {
    monitoringService.ProcessCheckResult(ctx, monitor, result)
}

// Subscribe to monitor changes
monitorSubscriber.Subscribe(ctx, func(action, id, monitor) {
    // update scheduler
})
```

**`cmd/notifier/main.go`:**
```go
// Create adapters
tgSender := telegram.New(token)
emailSender := smtp.New(...)
alertSubscriber := natssub.NewAlertSubscriber(js)

// Create app service
alertService := app.NewAlertService(tgSender.ForChat, emailSender.ForRecipient)

// Subscribe to alerts
alertSubscriber.Subscribe(ctx, alertService.Handle)
```

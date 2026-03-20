# PingCast Hexagonal Architecture Refactor — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor PingCast into strict Hexagonal Architecture — domain, port, app, adapter layers with clean dependency rule.

**Architecture:** Build new layers bottom-up (domain → port → app → adapters), then rewire cmd/ entry points and delete old packages. Each task compiles independently. Old and new code coexist during migration.

**Tech Stack:** Go 1.26, Fiber, sqlc, oapi-codegen, NATS JetStream, PostgreSQL

**Spec:** `docs/superpowers/specs/2026-03-20-hexagonal-architecture-refactor.md`

---

## Migration Strategy

Build new layers alongside old code. Old packages (`internal/auth/`, `internal/handler/`, `internal/notifier/`, `internal/nats/`) are NOT deleted until the new adapters are wired. This avoids broken intermediate states.

**Order:**
1. Domain types (pure Go, no deps)
2. Port interfaces (depend on domain)
3. App services + tests (depend on domain + port)
4. Adapter: postgres (implement repos using sqlc)
5. Adapter: nats (implement eventbus)
6. Adapter: telegram + smtp (implement AlertSender)
7. Adapter: http (implement handlers using app services)
8. Rewire cmd/ to use new layers
9. Delete old packages

---

## Task 1: Domain Layer

**Files:**
- Create: `internal/domain/user.go`
- Create: `internal/domain/monitor.go`
- Create: `internal/domain/incident.go`
- Create: `internal/domain/alert.go`

- [ ] **Step 1: Create domain types**

Create `internal/domain/user.go`:

```go
package domain

import (
	"time"

	"github.com/google/uuid"
)

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

func (u User) MonitorLimit() int {
	if u.Plan == PlanPro {
		return 50
	}
	return 5
}

func (u User) MinInterval() int {
	if u.Plan == PlanPro {
		return 30
	}
	return 300
}

func (u User) CanUseEmail() bool {
	return u.Plan == PlanPro
}
```

Create `internal/domain/monitor.go`:

```go
package domain

import (
	"time"

	"github.com/google/uuid"
)

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

Create `internal/domain/incident.go`:

```go
package domain

import (
	"time"

	"github.com/google/uuid"
)

type Incident struct {
	ID         int64
	MonitorID  uuid.UUID
	StartedAt  time.Time
	ResolvedAt *time.Time
	Cause      string
}

func (i Incident) IsResolved() bool {
	return i.ResolvedAt != nil
}
```

Create `internal/domain/alert.go`:

```go
package domain

import "github.com/google/uuid"

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

- [ ] **Step 2: Verify domain has zero external deps**

```bash
go build ./internal/domain/
```

Verify imports: only `time`, `github.com/google/uuid`. No pgtype, sqlc, fiber, nats.

- [ ] **Step 3: Commit**

```bash
git add internal/domain/
git commit -m "feat: domain layer — pure Go types with zero external dependencies"
```

---

## Task 2: Port Interfaces

**Files:**
- Create: `internal/port/repository.go`
- Create: `internal/port/eventbus.go`
- Create: `internal/port/alerter.go`

- [ ] **Step 1: Create port interfaces**

Create `internal/port/repository.go`:

```go
package port

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/domain"
)

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

Create `internal/port/eventbus.go`:

```go
package port

import (
	"context"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/domain"
)

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

Create `internal/port/alerter.go`:

```go
package port

import "context"

type AlertSender interface {
	NotifyDown(ctx context.Context, monitorName, monitorURL, cause string) error
	NotifyUp(ctx context.Context, monitorName, monitorURL string) error
}
```

- [ ] **Step 2: Verify ports import only domain**

```bash
go build ./internal/port/
```

- [ ] **Step 3: Commit**

```bash
git add internal/port/
git commit -m "feat: port interfaces — repository, eventbus, alerter contracts"
```

---

## Task 3: Application Services

**Files:**
- Create: `internal/app/auth.go`
- Create: `internal/app/auth_test.go`
- Create: `internal/app/monitoring.go`
- Create: `internal/app/monitoring_test.go`
- Create: `internal/app/alert.go`
- Create: `internal/app/alert_test.go`

- [ ] **Step 1: Create AuthService**

Create `internal/app/auth.go`:

```go
package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
	"golang.org/x/crypto/bcrypt"
)

const sessionDuration = 30 * 24 * time.Hour

var (
	slugRegex     = regexp.MustCompile(`^[a-z0-9-]{3,30}$`)
	reservedSlugs = map[string]bool{
		"login": true, "logout": true, "register": true, "api": true,
		"admin": true, "status": true, "health": true, "webhook": true,
		"pricing": true, "docs": true, "app": true, "dashboard": true,
		"settings": true, "billing": true,
	}
)

type AuthService struct {
	users    port.UserRepo
	sessions port.SessionRepo
}

func NewAuthService(users port.UserRepo, sessions port.SessionRepo) *AuthService {
	return &AuthService{users: users, sessions: sessions}
}

func (s *AuthService) Register(ctx context.Context, email, slug, password string) (*domain.User, string, error) {
	if err := ValidateSlug(slug); err != nil {
		return nil, "", err
	}
	if err := ValidatePassword(password); err != nil {
		return nil, "", err
	}

	hash, err := HashPassword(password)
	if err != nil {
		return nil, "", err
	}

	user, err := s.users.Create(ctx, email, slug, hash)
	if err != nil {
		return nil, "", fmt.Errorf("create user: %w", err)
	}

	sessionID, err := s.createSession(ctx, user.ID)
	if err != nil {
		return nil, "", err
	}

	return user, sessionID, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*domain.User, string, error) {
	user, hash, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, "", fmt.Errorf("invalid email or password")
	}

	if !CheckPassword(hash, password) {
		return nil, "", fmt.Errorf("invalid email or password")
	}

	sessionID, err := s.createSession(ctx, user.ID)
	if err != nil {
		return nil, "", err
	}

	return user, sessionID, nil
}

func (s *AuthService) ValidateSession(ctx context.Context, sessionID string) (*domain.User, error) {
	userID, err := s.sessions.GetUserID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("invalid session")
	}

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	_ = s.sessions.Touch(ctx, sessionID, time.Now().Add(sessionDuration))

	return user, nil
}

func (s *AuthService) Logout(ctx context.Context, sessionID string) error {
	return s.sessions.Delete(ctx, sessionID)
}

func (s *AuthService) LinkTelegram(ctx context.Context, userID uuid.UUID, chatID int64) error {
	return s.users.UpdateTelegramChatID(ctx, userID, chatID)
}

func (s *AuthService) UpgradeToPro(ctx context.Context, userID uuid.UUID, customerID, subscriptionID string) error {
	if err := s.users.UpdatePlan(ctx, userID, domain.PlanPro); err != nil {
		return err
	}
	return s.users.UpdateLemonSqueezy(ctx, userID, customerID, subscriptionID)
}

func (s *AuthService) DowngradeToFree(ctx context.Context, userID uuid.UUID) error {
	return s.users.UpdatePlan(ctx, userID, domain.PlanFree)
}

func (s *AuthService) createSession(ctx context.Context, userID uuid.UUID) (string, error) {
	token, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	if err := s.sessions.Create(ctx, token, userID, time.Now().Add(sessionDuration)); err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return token, nil
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func ValidateSlug(slug string) error {
	if !slugRegex.MatchString(slug) {
		return fmt.Errorf("slug must be 3-30 characters, lowercase alphanumeric and hyphens only")
	}
	if reservedSlugs[slug] {
		return fmt.Errorf("slug %q is reserved", slug)
	}
	return nil
}

func ValidatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	return nil
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
```

- [ ] **Step 2: Create MonitoringService**

Create `internal/app/monitoring.go`:

```go
package app

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

type MonitoringService struct {
	monitors     port.MonitorRepo
	checkResults port.CheckResultRepo
	incidents    port.IncidentRepo
	users        port.UserRepo
	alerts       port.AlertEventPublisher
}

func NewMonitoringService(
	monitors port.MonitorRepo,
	checkResults port.CheckResultRepo,
	incidents port.IncidentRepo,
	users port.UserRepo,
	alerts port.AlertEventPublisher,
) *MonitoringService {
	return &MonitoringService{
		monitors:     monitors,
		checkResults: checkResults,
		incidents:    incidents,
		users:        users,
		alerts:       alerts,
	}
}

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

func (s *MonitoringService) CreateMonitor(ctx context.Context, user *domain.User, input CreateMonitorInput) (*domain.Monitor, error) {
	count, err := s.monitors.CountByUserID(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("count monitors: %w", err)
	}
	if count >= user.MonitorLimit() {
		return nil, fmt.Errorf("monitor limit reached")
	}

	interval := input.IntervalSeconds
	if interval < user.MinInterval() {
		interval = user.MinInterval()
	}

	alertAfter := input.AlertAfterFailures
	if alertAfter <= 0 {
		alertAfter = 3
	}

	method := input.Method
	if method == "" {
		method = domain.MethodGET
	}

	expectedStatus := input.ExpectedStatus
	if expectedStatus == 0 {
		expectedStatus = 200
	}

	mon := &domain.Monitor{
		UserID:             user.ID,
		Name:               input.Name,
		URL:                input.URL,
		Method:             method,
		IntervalSeconds:    interval,
		ExpectedStatus:     expectedStatus,
		Keyword:            input.Keyword,
		AlertAfterFailures: alertAfter,
		IsPublic:           input.IsPublic,
		CurrentStatus:      domain.StatusUnknown,
	}

	return s.monitors.Create(ctx, mon)
}

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

func (s *MonitoringService) UpdateMonitor(ctx context.Context, user *domain.User, id uuid.UUID, input UpdateMonitorInput) (*domain.Monitor, error) {
	mon, err := s.monitors.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("monitor not found: %w", err)
	}
	if mon.UserID != user.ID {
		return nil, fmt.Errorf("monitor not found")
	}

	if input.Name != nil {
		mon.Name = *input.Name
	}
	if input.URL != nil {
		mon.URL = *input.URL
	}
	if input.Method != nil {
		mon.Method = *input.Method
	}
	if input.IntervalSeconds != nil {
		mon.IntervalSeconds = *input.IntervalSeconds
	}
	if input.ExpectedStatus != nil {
		mon.ExpectedStatus = *input.ExpectedStatus
	}
	if input.Keyword != nil {
		mon.Keyword = input.Keyword
	}
	if input.AlertAfterFailures != nil {
		mon.AlertAfterFailures = *input.AlertAfterFailures
	}
	if input.IsPaused != nil {
		mon.IsPaused = *input.IsPaused
	}
	if input.IsPublic != nil {
		mon.IsPublic = *input.IsPublic
	}

	if mon.IntervalSeconds < user.MinInterval() {
		mon.IntervalSeconds = user.MinInterval()
	}

	if err := s.monitors.Update(ctx, mon); err != nil {
		return nil, fmt.Errorf("update monitor: %w", err)
	}

	return mon, nil
}

func (s *MonitoringService) DeleteMonitor(ctx context.Context, userID, monitorID uuid.UUID) error {
	return s.monitors.Delete(ctx, monitorID, userID)
}

func (s *MonitoringService) TogglePause(ctx context.Context, user *domain.User, monitorID uuid.UUID) (*domain.Monitor, error) {
	mon, err := s.monitors.GetByID(ctx, monitorID)
	if err != nil || mon.UserID != user.ID {
		return nil, fmt.Errorf("monitor not found")
	}

	mon.IsPaused = !mon.IsPaused
	if err := s.monitors.Update(ctx, mon); err != nil {
		return nil, fmt.Errorf("update monitor: %w", err)
	}

	return mon, nil
}

// ProcessCheckResult is the core business logic.
// It writes the check result, detects status transitions, creates/resolves incidents,
// and publishes fat alert events.
func (s *MonitoringService) ProcessCheckResult(ctx context.Context, monitor *domain.Monitor, result *domain.CheckResult) error {
	if err := s.checkResults.Insert(ctx, result); err != nil {
		return fmt.Errorf("insert check result: %w", err)
	}

	current, err := s.monitors.GetByID(ctx, monitor.ID)
	previousStatus := domain.StatusUnknown
	if err == nil {
		previousStatus = current.CurrentStatus
	}

	if err := s.monitors.UpdateStatus(ctx, monitor.ID, result.Status); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	if previousStatus == result.Status {
		return nil
	}

	if result.Status == domain.StatusDown {
		return s.handleDown(ctx, monitor, result)
	}

	if result.Status == domain.StatusUp && previousStatus == domain.StatusDown {
		return s.handleRecovery(ctx, monitor)
	}

	return nil
}

func (s *MonitoringService) handleDown(ctx context.Context, monitor *domain.Monitor, result *domain.CheckResult) error {
	failures, err := s.checkResults.ConsecutiveFailures(ctx, monitor.ID)
	if err != nil {
		return fmt.Errorf("consecutive failures: %w", err)
	}

	if failures < monitor.AlertAfterFailures {
		return nil
	}

	inCooldown, err := s.incidents.IsInCooldown(ctx, monitor.ID)
	if err != nil {
		return fmt.Errorf("cooldown check: %w", err)
	}
	if inCooldown {
		return nil
	}

	cause := ""
	if result.ErrorMessage != nil {
		cause = *result.ErrorMessage
	}

	incident, err := s.incidents.Create(ctx, monitor.ID, cause)
	if err != nil {
		return fmt.Errorf("create incident: %w", err)
	}

	return s.publishAlert(ctx, monitor, domain.AlertDown, cause, incident.ID)
}

func (s *MonitoringService) handleRecovery(ctx context.Context, monitor *domain.Monitor) error {
	incident, err := s.incidents.GetOpen(ctx, monitor.ID)
	if err != nil {
		return nil // no open incident
	}

	if err := s.incidents.Resolve(ctx, incident.ID, time.Now()); err != nil {
		return fmt.Errorf("resolve incident: %w", err)
	}

	return s.publishAlert(ctx, monitor, domain.AlertUp, "", incident.ID)
}

func (s *MonitoringService) publishAlert(ctx context.Context, monitor *domain.Monitor, eventType domain.AlertEventType, cause string, incidentID int64) error {
	user, err := s.users.GetByID(ctx, monitor.UserID)
	if err != nil {
		return fmt.Errorf("get user for alert: %w", err)
	}

	event := &domain.AlertEvent{
		MonitorID:   monitor.ID,
		IncidentID:  incidentID,
		MonitorName: monitor.Name,
		MonitorURL:  monitor.URL,
		Event:       eventType,
		Cause:       cause,
		TgChatID:    user.TgChatID,
		Email:       user.Email,
		Plan:        user.Plan,
	}

	return s.alerts.PublishAlert(ctx, event)
}

// --- Query helpers ---

type MonitorWithUptime struct {
	Monitor domain.Monitor
	Uptime  float64
}

func (s *MonitoringService) ListMonitorsWithUptime(ctx context.Context, userID uuid.UUID) ([]MonitorWithUptime, error) {
	monitors, err := s.monitors.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]MonitorWithUptime, 0, len(monitors))
	for _, m := range monitors {
		uptime, _ := s.checkResults.GetUptime(ctx, m.ID, time.Now().Add(-24*time.Hour))
		result = append(result, MonitorWithUptime{Monitor: m, Uptime: uptime})
	}
	return result, nil
}

type MonitorDetail struct {
	Monitor   domain.Monitor
	Uptime24h float64
	Uptime7d  float64
	Uptime30d float64
	Incidents []domain.Incident
}

func (s *MonitoringService) GetMonitorDetail(ctx context.Context, monitorID uuid.UUID) (*MonitorDetail, error) {
	mon, err := s.monitors.GetByID(ctx, monitorID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	u24, _ := s.checkResults.GetUptime(ctx, monitorID, now.Add(-24*time.Hour))
	u7, _ := s.checkResults.GetUptime(ctx, monitorID, now.Add(-7*24*time.Hour))
	u30, _ := s.checkResults.GetUptime(ctx, monitorID, now.Add(-30*24*time.Hour))

	incidents, _ := s.incidents.ListByMonitorID(ctx, monitorID, 10)

	return &MonitorDetail{
		Monitor:   *mon,
		Uptime24h: u24,
		Uptime7d:  u7,
		Uptime30d: u30,
		Incidents: incidents,
	}, nil
}

type StatusMonitor struct {
	Name          string
	CurrentStatus domain.MonitorStatus
	Uptime90d     float64
}

type StatusPageData struct {
	Slug         string
	AllUp        bool
	ShowBranding bool
	Monitors     []StatusMonitor
	Incidents    []domain.Incident
}

func (s *MonitoringService) GetStatusPage(ctx context.Context, slug string) (*StatusPageData, error) {
	user, err := s.users.GetBySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	monitors, err := s.monitors.ListPublicBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	allUp := true
	statusMons := make([]StatusMonitor, 0, len(monitors))
	var incidents []domain.Incident

	for _, m := range monitors {
		uptime, _ := s.checkResults.GetUptime(ctx, m.ID, time.Now().Add(-90*24*time.Hour))
		if m.CurrentStatus != domain.StatusUp {
			allUp = false
		}
		statusMons = append(statusMons, StatusMonitor{
			Name:          m.Name,
			CurrentStatus: m.CurrentStatus,
			Uptime90d:     uptime,
		})

		monIncidents, _ := s.incidents.ListByMonitorID(ctx, m.ID, 5)
		incidents = append(incidents, monIncidents...)
	}

	return &StatusPageData{
		Slug:         slug,
		AllUp:        allUp,
		ShowBranding: user.Plan == domain.PlanFree,
		Monitors:     statusMons,
		Incidents:    incidents,
	}, nil
}
```

- [ ] **Step 3: Create AlertService**

Create `internal/app/alert.go`:

```go
package app

import (
	"context"
	"fmt"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

type TelegramFactory func(chatID int64) port.AlertSender
type EmailFactory func(to string) port.AlertSender

type AlertService struct {
	telegramFactory TelegramFactory
	emailFactory    EmailFactory
}

func NewAlertService(tg TelegramFactory, email EmailFactory) *AlertService {
	return &AlertService{telegramFactory: tg, emailFactory: email}
}

func (s *AlertService) Handle(ctx context.Context, event *domain.AlertEvent) error {
	var senders []port.AlertSender

	if s.telegramFactory != nil && event.TgChatID != nil {
		senders = append(senders, s.telegramFactory(*event.TgChatID))
	}

	if s.emailFactory != nil && event.Plan == domain.PlanPro && event.Email != "" {
		senders = append(senders, s.emailFactory(event.Email))
	}

	for _, sender := range senders {
		var err error
		switch event.Event {
		case domain.AlertDown:
			err = sender.NotifyDown(ctx, event.MonitorName, event.MonitorURL, event.Cause)
		case domain.AlertUp:
			err = sender.NotifyUp(ctx, event.MonitorName, event.MonitorURL)
		}
		if err != nil {
			return fmt.Errorf("alert delivery failed: %w", err)
		}
	}

	return nil
}
```

- [ ] **Step 4: Verify app imports only domain + port**

```bash
go build ./internal/app/
```

- [ ] **Step 5: Commit**

```bash
git add internal/app/
git commit -m "feat: application services — AuthService, MonitoringService, AlertService"
```

---

## Task 4: Adapter — postgres repositories + mapper

**Files:**
- Create: `internal/adapter/postgres/mapper.go`
- Create: `internal/adapter/postgres/user_repo.go`
- Create: `internal/adapter/postgres/session_repo.go`
- Create: `internal/adapter/postgres/monitor_repo.go`
- Create: `internal/adapter/postgres/check_result_repo.go`
- Create: `internal/adapter/postgres/incident_repo.go`

- [ ] **Step 1: Create mapper**

Create `internal/adapter/postgres/mapper.go` — centralizes all sqlc ↔ domain conversions:

```go
package postgres

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

func toDomainUser(u gen.GetUserByIDRow) *domain.User {
	return &domain.User{
		ID:        u.ID,
		Email:     u.Email,
		Slug:      u.Slug,
		Plan:      domain.Plan(u.Plan),
		TgChatID:  pgtypeInt8ToPtr(u.TgChatID),
		CreatedAt: u.CreatedAt,
	}
}

func toDomainUserFromCreate(u gen.User) *domain.User {
	return &domain.User{
		ID:        u.ID,
		Email:     u.Email,
		Slug:      u.Slug,
		Plan:      domain.Plan(u.Plan),
		TgChatID:  pgtypeInt8ToPtr(u.TgChatID),
		CreatedAt: u.CreatedAt,
	}
}

func toDomainUserFromSlug(u gen.GetUserBySlugRow) *domain.User {
	return &domain.User{
		ID:        u.ID,
		Email:     u.Email,
		Slug:      u.Slug,
		Plan:      domain.Plan(u.Plan),
		TgChatID:  pgtypeInt8ToPtr(u.TgChatID),
		CreatedAt: u.CreatedAt,
	}
}

func toDomainMonitor(m gen.Monitor) *domain.Monitor {
	return &domain.Monitor{
		ID:                 m.ID,
		UserID:             m.UserID,
		Name:               m.Name,
		URL:                m.Url,
		Method:             domain.HTTPMethod(m.Method),
		IntervalSeconds:    int(m.IntervalSeconds),
		ExpectedStatus:     int(m.ExpectedStatus),
		Keyword:            m.Keyword,
		AlertAfterFailures: int(m.AlertAfterFailures),
		IsPaused:           m.IsPaused,
		IsPublic:           m.IsPublic,
		CurrentStatus:      domain.MonitorStatus(m.CurrentStatus),
		CreatedAt:          m.CreatedAt,
	}
}

func toDomainMonitors(ms []gen.Monitor) []domain.Monitor {
	result := make([]domain.Monitor, len(ms))
	for i, m := range ms {
		result[i] = *toDomainMonitor(m)
	}
	return result
}

func toDomainIncident(i gen.Incident) *domain.Incident {
	return &domain.Incident{
		ID:         i.ID,
		MonitorID:  i.MonitorID,
		StartedAt:  i.StartedAt,
		ResolvedAt: pgtypeTimestamptzToPtr(i.ResolvedAt),
		Cause:      i.Cause,
	}
}

func toDomainIncidents(is []gen.Incident) []domain.Incident {
	result := make([]domain.Incident, len(is))
	for i, inc := range is {
		result[i] = *toDomainIncident(inc)
	}
	return result
}

func pgtypeInt8ToPtr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}

func pgtypeTimestamptzToPtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	return &v.Time
}

func timeToPgtypeTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}
```

- [ ] **Step 2: Create all 5 repo implementations**

Each repo is a thin wrapper: call sqlc query → mapper → return domain type. Create `user_repo.go`, `session_repo.go`, `monitor_repo.go`, `check_result_repo.go`, `incident_repo.go`.

Example pattern (user_repo.go):

```go
package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

type UserRepo struct {
	q *gen.Queries
}

func NewUserRepo(q *gen.Queries) *UserRepo {
	return &UserRepo{q: q}
}

func (r *UserRepo) Create(ctx context.Context, email, slug, passwordHash string) (*domain.User, error) {
	u, err := r.q.CreateUser(ctx, gen.CreateUserParams{
		Email: email, Slug: slug, PasswordHash: passwordHash,
	})
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return toDomainUserFromCreate(u), nil
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	u, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return toDomainUser(u), nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, string, error) {
	u, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, "", fmt.Errorf("get user by email: %w", err)
	}
	user := &domain.User{
		ID: u.ID, Email: u.Email, Slug: u.Slug,
		Plan: domain.Plan(u.Plan), TgChatID: pgtypeInt8ToPtr(u.TgChatID),
		CreatedAt: u.CreatedAt,
	}
	return user, u.PasswordHash, nil
}

func (r *UserRepo) GetBySlug(ctx context.Context, slug string) (*domain.User, error) {
	u, err := r.q.GetUserBySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("get user by slug: %w", err)
	}
	return toDomainUserFromSlug(u), nil
}

func (r *UserRepo) UpdatePlan(ctx context.Context, id uuid.UUID, plan domain.Plan) error {
	return r.q.UpdateUserPlan(ctx, gen.UpdateUserPlanParams{ID: id, Plan: string(plan)})
}

func (r *UserRepo) UpdateTelegramChatID(ctx context.Context, id uuid.UUID, chatID int64) error {
	return r.q.UpdateUserTelegramChatID(ctx, gen.UpdateUserTelegramChatIDParams{
		ID: id, TgChatID: pgtype.Int8{Int64: chatID, Valid: true},
	})
}

func (r *UserRepo) UpdateLemonSqueezy(ctx context.Context, id uuid.UUID, customerID, subscriptionID string) error {
	return r.q.UpdateUserLemonSqueezy(ctx, gen.UpdateUserLemonSqueezyParams{
		ID: id, LemonSqueezyCustomerID: &customerID, LemonSqueezySubscriptionID: &subscriptionID,
	})
}
```

Follow the same pattern for session_repo.go, monitor_repo.go, check_result_repo.go, incident_repo.go. Each implements the corresponding port interface using sqlc queries + mapper.

- [ ] **Step 3: Add compile-time interface checks**

At the bottom of each repo file, add:
```go
var _ port.UserRepo = (*UserRepo)(nil)
```

- [ ] **Step 4: Verify compilation**

```bash
go build ./internal/adapter/postgres/
```

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/postgres/
git commit -m "feat: postgres adapter — repository implementations with mapper"
```

---

## Task 5: Adapter — nats (publisher + subscriber + client)

**Files:**
- Create: `internal/adapter/nats/client.go`
- Create: `internal/adapter/nats/publisher.go`
- Create: `internal/adapter/nats/subscriber.go`

- [ ] **Step 1: Move and adapt NATS client**

Move `internal/nats/client.go` → `internal/adapter/nats/client.go`. Change package name from `natsbus` to `natsadapter`. Keep `Connect()` and `SetupStreams()` functions.

- [ ] **Step 2: Create publishers**

Create `internal/adapter/nats/publisher.go`:

```go
package natsadapter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/kirillinakin/pingcast/internal/domain"
)

type MonitorPublisher struct {
	js jetstream.JetStream
}

func NewMonitorPublisher(js jetstream.JetStream) *MonitorPublisher {
	return &MonitorPublisher{js: js}
}

type monitorChangedMsg struct {
	Action    domain.MonitorAction `json:"action"`
	MonitorID uuid.UUID            `json:"monitor_id"`
	Monitor   *domain.Monitor      `json:"monitor,omitempty"`
}

func (p *MonitorPublisher) PublishMonitorChanged(ctx context.Context, action domain.MonitorAction, monitorID uuid.UUID, monitor *domain.Monitor) error {
	data, err := json.Marshal(monitorChangedMsg{Action: action, MonitorID: monitorID, Monitor: monitor})
	if err != nil {
		return fmt.Errorf("marshal monitor changed: %w", err)
	}
	_, err = p.js.Publish(ctx, "monitors.changed", data)
	return err
}

type AlertPublisher struct {
	js jetstream.JetStream
}

func NewAlertPublisher(js jetstream.JetStream) *AlertPublisher {
	return &AlertPublisher{js: js}
}

func (p *AlertPublisher) PublishAlert(ctx context.Context, event *domain.AlertEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal alert: %w", err)
	}
	subject := "alerts." + string(event.Event)
	_, err = p.js.Publish(ctx, subject, data)
	return err
}
```

- [ ] **Step 3: Create subscribers**

Create `internal/adapter/nats/subscriber.go` with `MonitorSubscriber` and `AlertSubscriber`. Each implements the port interface, creating a JetStream consumer and calling the handler callback with domain types.

- [ ] **Step 4: Verify and commit**

```bash
go build ./internal/adapter/nats/
git add internal/adapter/nats/
git commit -m "feat: nats adapter — publisher and subscriber implementations"
```

---

## Task 6: Adapter — telegram + smtp

**Files:**
- Create: `internal/adapter/telegram/sender.go`
- Create: `internal/adapter/smtp/sender.go`

- [ ] **Step 1: Create Telegram adapter with factory**

Move logic from `internal/notifier/telegram.go` to `internal/adapter/telegram/sender.go`. Add `ForChat(chatID) port.AlertSender` factory method.

- [ ] **Step 2: Create SMTP adapter with factory**

Move logic from `internal/notifier/email.go` to `internal/adapter/smtp/sender.go`. Add `ForRecipient(email) port.AlertSender` factory method.

- [ ] **Step 3: Verify and commit**

```bash
go build ./internal/adapter/telegram/ ./internal/adapter/smtp/
git add internal/adapter/telegram/ internal/adapter/smtp/
git commit -m "feat: telegram and smtp adapters with AlertSender factory"
```

---

## Task 7: Adapter — HTTP (server, pages, webhook, middleware)

**Files:**
- Create: `internal/adapter/http/server.go`
- Create: `internal/adapter/http/pages.go`
- Create: `internal/adapter/http/webhook.go`
- Create: `internal/adapter/http/middleware.go`
- Create: `internal/adapter/http/ratelimit.go`
- Create: `internal/adapter/http/setup.go`

- [ ] **Step 1: Create middleware**

Move from `internal/auth/middleware.go`. Change: depends on `*app.AuthService`, stores `*domain.User` in Locals.

- [ ] **Step 2: Create ratelimit**

Move from `internal/auth/ratelimit.go` unchanged.

- [ ] **Step 3: Create server.go**

Rewrite `internal/handler/server.go`. Key change: Server struct holds `*app.AuthService`, `*app.MonitoringService`, `port.MonitorEventPublisher`. Handlers become thin:
- Parse request → call app service → map domain type to oapi-codegen type → respond
- No sqlc imports, no `gen.Queries`

- [ ] **Step 4: Create pages.go**

Rewrite `internal/handler/pages.go`. PageHandler holds `*app.AuthService`, `*app.MonitoringService`. All queries go through app services. Templates receive `domain.Monitor`, `domain.User`, etc.

- [ ] **Step 5: Create webhook.go**

Rewrite `internal/handler/webhook.go`. Lemon Squeezy calls `auth.UpgradeToPro`/`auth.DowngradeToFree`. Telegram bot calls `auth.LinkTelegram`. No direct sqlc access.

- [ ] **Step 6: Create setup.go**

Move from `internal/handler/setup.go`. Update import paths.

- [ ] **Step 7: Verify and commit**

```bash
go build ./internal/adapter/http/
git add internal/adapter/http/
git commit -m "feat: http adapter — thin handlers using app services"
```

---

## Task 8: Rewire cmd/ entry points

**Files:**
- Modify: `cmd/api/main.go`
- Modify: `cmd/checker/main.go`
- Modify: `cmd/notifier/main.go`

- [ ] **Step 1: Rewrite cmd/api/main.go**

Composition root: create postgres repos → create app services → create http adapter → wire Fiber.

- [ ] **Step 2: Rewrite cmd/checker/main.go**

Composition root: create postgres repos → create MonitoringService → checker uses `monitoringService.ProcessCheckResult` → NATS subscriber updates scheduler.

- [ ] **Step 3: Rewrite cmd/notifier/main.go**

Composition root: create telegram/smtp senders → create AlertService → NATS subscriber calls `alertService.Handle`.

- [ ] **Step 4: Verify all 3 services compile**

```bash
go build -o /tmp/api ./cmd/api/
go build -o /tmp/checker ./cmd/checker/
go build -o /tmp/notifier ./cmd/notifier/
```

- [ ] **Step 5: Run all tests**

```bash
go test ./... -count=1 -timeout=30s
```

- [ ] **Step 6: Commit**

```bash
git add cmd/
git commit -m "refactor: rewire cmd/ to use hex arch layers"
```

---

## Task 9: Delete old packages

**Files:**
- Delete: `internal/auth/` (entire directory)
- Delete: `internal/handler/` (entire directory)
- Delete: `internal/notifier/` (entire directory)
- Delete: `internal/nats/` (entire directory)

- [ ] **Step 1: Delete old packages**

```bash
rm -rf internal/auth/ internal/handler/ internal/notifier/ internal/nats/
```

- [ ] **Step 2: Verify everything still compiles**

```bash
go build ./...
go test ./... -count=1 -timeout=30s
```

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "refactor: delete old packages — migration to hex arch complete"
```

---

## Summary

9 tasks, ordered by dependency. New layers are built alongside old code, then cmd/ is rewired, then old code is deleted.

**Task order:**
1. Domain layer (pure Go types)
2. Port interfaces (contracts)
3. Application services (business logic)
4. Adapter: postgres (sqlc repos + mapper)
5. Adapter: nats (publisher + subscriber)
6. Adapter: telegram + smtp (AlertSender)
7. Adapter: http (Fiber handlers)
8. Rewire cmd/ entry points
9. Delete old packages

**Dependency rule verified at each step:**
- Task 1: domain imports only stdlib + uuid
- Task 2: port imports only domain
- Task 3: app imports only domain + port
- Tasks 4-7: adapters import domain + port + external libs
- Task 8: cmd/ imports everything, wires via DI
- Task 9: cleanup

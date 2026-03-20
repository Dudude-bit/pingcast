# Multi-Type Monitors Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add HTTP/TCP/DNS check types with a checker registry, schema-driven forms, and raw JSON config storage.

**Architecture:** `MonitorType` + `json.RawMessage` in domain. Config structs live in adapter/checker/. Registry maps type → checker. Each checker implements Check + ValidateConfig + ConfigSchema + Target + Host. Frontend renders forms dynamically from ConfigSchema via one universal template.

**Tech Stack:** Go 1.26, PostgreSQL JSONB, sqlc, HTMX, Fiber

**Spec:** `docs/superpowers/specs/2026-03-20-multi-type-monitors.md`

---

## Task Order

Build bottom-up following hex arch layers. Each task compiles independently.

1. Domain changes (MonitorType, raw JSON config, remove old fields)
2. Port changes (CheckerRegistry, expanded MonitorChecker)
3. Database migrations + sqlc regeneration
4. Checker registry + HTTP/TCP/DNS adapters
5. App service updates (registry, RunCheck, validation)
6. Postgres adapter updates (mapper for new fields)
7. Alert chain (MonitorURL → MonitorTarget)
8. HTTP adapter + frontend (API types, schema-driven forms, templates)
9. Rewire cmd/ entry points
10. Cleanup (delete HTTPMethod from domain, update tests)

---

## Task 1: Domain Changes

**Files:**
- Modify: `internal/domain/monitor.go`
- Modify: `internal/domain/alert.go`

- [ ] **Step 1: Update domain/monitor.go**

Replace the entire file:

```go
package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type MonitorStatus string

const (
	StatusUp      MonitorStatus = "up"
	StatusDown    MonitorStatus = "down"
	StatusUnknown MonitorStatus = "unknown"
)

type MonitorType string

const (
	MonitorHTTP MonitorType = "http"
	MonitorTCP  MonitorType = "tcp"
	MonitorDNS  MonitorType = "dns"
)

func (t MonitorType) Valid() bool {
	switch t {
	case MonitorHTTP, MonitorTCP, MonitorDNS:
		return true
	}
	return false
}

type Monitor struct {
	ID                 uuid.UUID
	UserID             uuid.UUID
	Name               string
	Type               MonitorType
	CheckConfig        json.RawMessage
	IntervalSeconds    int
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

Removed: `URL`, `Method`, `ExpectedStatus`, `Keyword`, `HTTPMethod` type. Added: `MonitorType`, `CheckConfig json.RawMessage`.

- [ ] **Step 2: Update domain/alert.go — MonitorURL → MonitorTarget**

```go
type AlertEvent struct {
	MonitorID     uuid.UUID
	IncidentID    int64
	MonitorName   string
	MonitorTarget string         // was MonitorURL
	Event         AlertEventType
	Cause         string
	TgChatID      *int64
	Email         string
	Plan          Plan
}
```

- [ ] **Step 3: Verify domain compiles**

```bash
go build ./internal/domain/
```

Note: Other packages will break — that's expected. We fix them in subsequent tasks.

- [ ] **Step 4: Commit**

```bash
git add internal/domain/
git commit -m "feat: domain — MonitorType, raw JSON config, remove HTTP-specific fields"
```

---

## Task 2: Port Changes

**Files:**
- Modify: `internal/port/checker.go`
- Modify: `internal/port/alerter.go`

- [ ] **Step 1: Rewrite port/checker.go**

```go
package port

import (
	"context"
	"encoding/json"

	"github.com/kirillinakin/pingcast/internal/domain"
)

type MonitorChecker interface {
	Check(ctx context.Context, monitor *domain.Monitor) *domain.CheckResult
	ValidateConfig(raw json.RawMessage) error
	ConfigSchema() ConfigSchema
	Target(raw json.RawMessage) string
	Host(raw json.RawMessage) string
}

type CheckerRegistry interface {
	Get(monitorType domain.MonitorType) (MonitorChecker, error)
	Types() []MonitorTypeInfo
	ValidateConfig(monitorType domain.MonitorType, raw json.RawMessage) error
	Target(monitorType domain.MonitorType, raw json.RawMessage) string
	Host(monitorType domain.MonitorType, raw json.RawMessage) string
}

type MonitorTypeInfo struct {
	Type   domain.MonitorType `json:"type"`
	Label  string             `json:"label"`
	Schema ConfigSchema       `json:"schema"`
}

type ConfigSchema struct {
	Fields []ConfigField `json:"fields"`
}

type ConfigField struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Default     any      `json:"default,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Options     []Option `json:"options,omitempty"`
}

type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}
```

- [ ] **Step 2: Update port/alerter.go — monitorURL → monitorTarget**

```go
type AlertSender interface {
	NotifyDown(ctx context.Context, monitorName, monitorTarget, cause string) error
	NotifyUp(ctx context.Context, monitorName, monitorTarget string) error
}
```

- [ ] **Step 3: Verify ports compile**

```bash
go build ./internal/port/
```

- [ ] **Step 4: Commit**

```bash
git add internal/port/
git commit -m "feat: ports — CheckerRegistry, ConfigSchema, MonitorTarget in AlertSender"
```

---

## Task 3: Database Migrations + sqlc

**Files:**
- Create: `internal/database/migrations/006_add_monitor_type_and_config.sql`
- Create: `internal/database/migrations/007_drop_old_monitor_columns.sql`
- Modify: `internal/sqlc/queries/monitors.sql`
- Regenerate: `internal/sqlc/gen/`

- [ ] **Step 1: Create migration 006**

```sql
ALTER TABLE monitors
    ADD COLUMN type VARCHAR(10) NOT NULL DEFAULT 'http',
    ADD COLUMN check_config JSONB NOT NULL DEFAULT '{}';

UPDATE monitors SET check_config = jsonb_strip_nulls(jsonb_build_object(
    'url', url,
    'method', method,
    'expected_status', expected_status,
    'keyword', keyword
));
```

- [ ] **Step 2: Create migration 007**

```sql
ALTER TABLE monitors
    DROP COLUMN url,
    DROP COLUMN method,
    DROP COLUMN expected_status,
    DROP COLUMN keyword;
```

- [ ] **Step 3: Update sqlc queries**

Update `internal/sqlc/queries/monitors.sql` — all queries replace `url, method, expected_status, keyword` with `type, check_config`. Example for CreateMonitor:

```sql
-- name: CreateMonitor :one
INSERT INTO monitors (user_id, name, type, check_config, interval_seconds, alert_after_failures, is_paused, is_public)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, user_id, name, type, check_config, interval_seconds, alert_after_failures, is_paused, is_public, current_status, created_at;
```

Update all SELECT queries similarly — replace old columns with `type, check_config`.

Update `UpdateMonitor`:
```sql
-- name: UpdateMonitor :exec
UPDATE monitors
SET name = $2, check_config = $3, interval_seconds = $4, alert_after_failures = $5, is_paused = $6, is_public = $7
WHERE id = $1 AND user_id = $8;
```

- [ ] **Step 4: Regenerate sqlc**

```bash
cd internal/sqlc && sqlc generate && cd ../..
```

- [ ] **Step 5: Commit**

```bash
git add internal/database/migrations/ internal/sqlc/
git commit -m "feat: migrations for monitor type + JSONB check_config, updated sqlc queries"
```

---

## Task 4: Checker Registry + HTTP/TCP/DNS Adapters

**Files:**
- Create: `internal/adapter/checker/registry.go`
- Rename+Modify: `internal/adapter/checker/client.go` → `internal/adapter/checker/http.go`
- Create: `internal/adapter/checker/tcp.go`
- Create: `internal/adapter/checker/dns.go`
- Modify: `internal/adapter/checker/worker.go`

- [ ] **Step 1: Create registry**

Create `internal/adapter/checker/registry.go`:

```go
package checker

import (
	"encoding/json"
	"fmt"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

var _ port.CheckerRegistry = (*Registry)(nil)

type registryEntry struct {
	label   string
	checker port.MonitorChecker
}

type Registry struct {
	entries map[domain.MonitorType]registryEntry
}

func NewRegistry() *Registry {
	return &Registry{entries: make(map[domain.MonitorType]registryEntry)}
}

func (r *Registry) Register(t domain.MonitorType, label string, c port.MonitorChecker) {
	r.entries[t] = registryEntry{label: label, checker: c}
}

func (r *Registry) Get(t domain.MonitorType) (port.MonitorChecker, error) {
	entry, ok := r.entries[t]
	if !ok {
		return nil, fmt.Errorf("unknown monitor type: %s", t)
	}
	return entry.checker, nil
}

func (r *Registry) Types() []port.MonitorTypeInfo {
	types := make([]port.MonitorTypeInfo, 0, len(r.entries))
	for t, entry := range r.entries {
		types = append(types, port.MonitorTypeInfo{
			Type:   t,
			Label:  entry.label,
			Schema: entry.checker.ConfigSchema(),
		})
	}
	return types
}

func (r *Registry) ValidateConfig(t domain.MonitorType, raw json.RawMessage) error {
	checker, err := r.Get(t)
	if err != nil {
		return err
	}
	return checker.ValidateConfig(raw)
}

func (r *Registry) Target(t domain.MonitorType, raw json.RawMessage) string {
	checker, err := r.Get(t)
	if err != nil {
		return ""
	}
	return checker.Target(raw)
}

func (r *Registry) Host(t domain.MonitorType, raw json.RawMessage) string {
	checker, err := r.Get(t)
	if err != nil {
		return ""
	}
	return checker.Host(raw)
}
```

- [ ] **Step 2: Rename client.go → http.go and rewrite**

Rename: `mv internal/adapter/checker/client.go internal/adapter/checker/http.go`

Rewrite `http.go` — HTTPChecker reads config from `monitor.CheckConfig` JSON instead of Monitor fields. Add `ValidateConfig`, `ConfigSchema`, `Target`, `Host` methods. Move `HTTPMethod` type here from domain.

Full implementation: deserialize `json.RawMessage` → `HTTPCheckConfig`, use `cfg.URL`, `cfg.Method`, etc. for the HTTP request. Same timeout/redirect/TLS/keyword logic.

- [ ] **Step 3: Create tcp.go**

```go
package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

var _ port.MonitorChecker = (*TCPChecker)(nil)

type TCPCheckConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type TCPChecker struct {
	timeout time.Duration
}

func NewTCPChecker(timeout time.Duration) *TCPChecker {
	return &TCPChecker{timeout: timeout}
}

func (c *TCPChecker) Check(ctx context.Context, monitor *domain.Monitor) *domain.CheckResult {
	var cfg TCPCheckConfig
	start := time.Now()
	result := &domain.CheckResult{MonitorID: monitor.ID, CheckedAt: start}

	if err := json.Unmarshal(monitor.CheckConfig, &cfg); err != nil {
		errMsg := fmt.Sprintf("invalid tcp config: %s", err)
		result.Status = domain.StatusDown
		result.ErrorMessage = &errMsg
		return result
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	conn, err := net.DialTimeout("tcp", addr, c.timeout)
	result.ResponseTimeMs = int(time.Since(start).Milliseconds())

	if err != nil {
		errMsg := fmt.Sprintf("tcp connect failed: %s", err)
		result.Status = domain.StatusDown
		result.ErrorMessage = &errMsg
		return result
	}
	conn.Close()

	result.Status = domain.StatusUp
	return result
}

func (c *TCPChecker) ValidateConfig(raw json.RawMessage) error {
	var cfg TCPCheckConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("invalid tcp config: %w", err)
	}
	if cfg.Host == "" {
		return fmt.Errorf("host required")
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("port must be 1-65535")
	}
	return nil
}

func (c *TCPChecker) ConfigSchema() port.ConfigSchema {
	return port.ConfigSchema{Fields: []port.ConfigField{
		{Name: "host", Label: "Host", Type: "text", Required: true, Placeholder: "db.example.com"},
		{Name: "port", Label: "Port", Type: "number", Required: true, Placeholder: "5432"},
	}}
}

func (c *TCPChecker) Target(raw json.RawMessage) string {
	var cfg TCPCheckConfig
	if json.Unmarshal(raw, &cfg) != nil {
		return ""
	}
	return fmt.Sprintf("tcp://%s:%d", cfg.Host, cfg.Port)
}

func (c *TCPChecker) Host(raw json.RawMessage) string {
	var cfg TCPCheckConfig
	if json.Unmarshal(raw, &cfg) != nil {
		return ""
	}
	return cfg.Host
}
```

- [ ] **Step 4: Create dns.go**

Same pattern: `DNSCheckConfig`, `DNSChecker` with `Check` (uses `net.Resolver.LookupHost`), `ValidateConfig`, `ConfigSchema`, `Target`, `Host`.

- [ ] **Step 5: Update worker.go — remove checker dependency**

WorkerPool no longer holds a checker. It dispatches monitors to handler callback. The handler (in cmd/) calls `monitoringSvc.RunCheck`. Also update host limiter to use `registry.Host`.

```go
type WorkerPool struct {
	registry    port.CheckerRegistry  // for host extraction only
	hostLimiter *HostLimiter
	jobs        chan *domain.Monitor
	handler     CheckHandler
	// ...
}
```

`worker()` uses `wp.registry.Host(m.Type, m.CheckConfig)` instead of `extractHost(m.URL)`.

- [ ] **Step 6: Verify adapter/checker compiles**

```bash
go build ./internal/adapter/checker/
```

- [ ] **Step 7: Commit**

```bash
git add internal/adapter/checker/
git commit -m "feat: checker registry + HTTP/TCP/DNS adapters with schema-driven config"
```

---

## Task 5: App Service Updates

**Files:**
- Modify: `internal/app/monitoring.go`

- [ ] **Step 1: Update MonitoringService**

- Replace `checker port.MonitorChecker` with `registry port.CheckerRegistry`
- Update `NewMonitoringService` constructor
- Update `RunCheck` to use registry lookup
- Update `CreateMonitorInput` — remove old fields, add `Type` + `CheckConfig json.RawMessage`
- Update `UpdateMonitorInput` — remove `Type`, `CheckConfig` as `json.RawMessage`
- `CreateMonitor` validates config via `s.registry.ValidateConfig(input.Type, input.CheckConfig)`
- `UpdateMonitor` validates config if provided
- `publishAlert` uses `registry.Target` instead of `monitor.URL`
- Add `Registry() port.CheckerRegistry` getter for HTTP adapter to call `Types()`

- [ ] **Step 2: Verify app compiles**

```bash
go build ./internal/app/
```

- [ ] **Step 3: Commit**

```bash
git add internal/app/
git commit -m "feat: MonitoringService with CheckerRegistry, RunCheck, config validation"
```

---

## Task 6: Postgres Adapter Updates

**Files:**
- Modify: `internal/adapter/postgres/mapper.go`
- Modify: `internal/adapter/postgres/monitor_repo.go`

- [ ] **Step 1: Update mapper.go**

- `toDomainMonitor` reads `Type` (string → `domain.MonitorType`) and `CheckConfig` (raw bytes → `json.RawMessage`)
- Remove old field mappings (URL, Method, ExpectedStatus, Keyword)
- Repo methods pass `json.RawMessage` directly to sqlc (JSONB ↔ `[]byte`)

- [ ] **Step 2: Update monitor_repo.go**

- `Create` passes `Type` and `CheckConfig` to sqlc
- `Update` passes `CheckConfig` (no more URL/Method/etc)
- All read methods map new columns

- [ ] **Step 3: Verify and commit**

```bash
go build ./internal/adapter/postgres/
git add internal/adapter/postgres/
git commit -m "feat: postgres adapter — type + check_config JSONB mapping"
```

---

## Task 7: Alert Chain Updates

**Files:**
- Modify: `internal/adapter/telegram/sender.go`
- Modify: `internal/adapter/smtp/sender.go`
- Modify: `internal/adapter/nats/publisher.go`

- [ ] **Step 1: Update senders — monitorURL → monitorTarget**

Telegram message: `"URL: \`%s\`"` → `"Target: \`%s\`"`
SMTP message: `"URL: %s"` → `"Target: %s"`

Method parameter names: `url` → `target`.

- [ ] **Step 2: Update NATS publisher**

`AlertEvent` JSON serialization uses `MonitorTarget` instead of `MonitorURL`.

- [ ] **Step 3: Verify and commit**

```bash
go build ./internal/adapter/...
git add internal/adapter/telegram/ internal/adapter/smtp/ internal/adapter/nats/
git commit -m "refactor: alert chain — MonitorURL → MonitorTarget"
```

---

## Task 8: HTTP Adapter + Frontend

**Files:**
- Modify: `internal/adapter/http/server.go`
- Modify: `internal/adapter/http/pages.go`
- Modify: `internal/adapter/http/setup.go`
- Create: `internal/web/templates/monitor_config_fields.html`
- Modify: `internal/web/templates/monitor_form.html`
- Modify: `internal/web/templates/dashboard.html`
- Modify: `internal/web/templates/monitor_detail.html`
- Modify: `api/openapi.yaml`
- Regenerate: `internal/api/gen/server.go`

- [ ] **Step 1: Update OpenAPI spec + regenerate**

Add `CheckConfig`, `MonitorType`, `target` to schemas. Remove old fields. Add `GET /api/monitor-types` endpoint. Regenerate oapi-codegen.

- [ ] **Step 2: Update server.go**

- `domainMonitorToAPI` maps `Type`, `CheckConfig`, `Target` (via registry)
- `CreateMonitor` reads new input format
- Add `ListMonitorTypes` handler → calls `registry.Types()`
- All monitor response types include `target` field

- [ ] **Step 3: Update pages.go**

- `MonitorNewForm` passes `MonitorTypes` to template
- `MonitorConfigFields` — new handler, returns schema-driven HTML fragment
- `MonitorCreate` reads `type` + constructs `json.RawMessage` from `config.*` form fields
- Dashboard/detail pass `.Target` computed via registry

- [ ] **Step 4: Create universal config fields template**

`internal/web/templates/monitor_config_fields.html` — renders from `ConfigSchema.Fields`.

- [ ] **Step 5: Update monitor_form.html**

Type selector populated from `.MonitorTypes`, HTMX loads config fields dynamically.

- [ ] **Step 6: Update dashboard.html and monitor_detail.html**

`.Monitor.URL` → `.Target`

- [ ] **Step 7: Verify and commit**

```bash
go build ./...
git add internal/adapter/http/ internal/web/ api/ internal/api/
git commit -m "feat: HTTP adapter + schema-driven frontend for multi-type monitors"
```

---

## Task 9: Rewire cmd/ Entry Points

**Files:**
- Modify: `cmd/api/main.go`
- Modify: `cmd/checker/main.go`

- [ ] **Step 1: Update cmd/api/main.go**

Create registry, register all checkers, pass to MonitoringService. No more nil checker.

- [ ] **Step 2: Update cmd/checker/main.go**

Same registry. Worker pool handler calls `monitoringSvc.RunCheck`. NATS subscriber passes full `domain.Monitor` to scheduler.

- [ ] **Step 3: Build + test**

```bash
go build -o /tmp/api ./cmd/api/
go build -o /tmp/checker ./cmd/checker/
go build -o /tmp/notifier ./cmd/notifier/
go test ./... -count=1 -timeout=30s
```

- [ ] **Step 4: Commit**

```bash
git add cmd/
git commit -m "feat: cmd/ composition roots with checker registry"
```

---

## Task 10: Cleanup + Tests

**Files:**
- Modify: `internal/adapter/checker/client_test.go` (renamed to `http_test.go`)
- Delete: unused `HTTPMethod` from domain if still present

- [ ] **Step 1: Update checker tests for new Monitor structure**

Tests create `domain.Monitor` with `Type: domain.MonitorHTTP` and `CheckConfig` as JSON.

- [ ] **Step 2: Add TCP + DNS checker tests**

Test each checker with valid/invalid configs, up/down results.

- [ ] **Step 3: Add registry test**

Test `Register`, `Get`, `Types`, `ValidateConfig`.

- [ ] **Step 4: Final verification**

```bash
go build ./...
go vet ./...
go test ./... -count=1 -timeout=30s
```

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "test: checker tests for HTTP/TCP/DNS + registry"
```

---

## Summary

10 tasks, hex arch layered bottom-up.

**Task order:**
1. Domain (MonitorType, raw JSON config)
2. Ports (CheckerRegistry, ConfigSchema, AlertSender rename)
3. Database migrations + sqlc
4. Checker registry + HTTP/TCP/DNS adapters
5. App service (registry, RunCheck, validation)
6. Postgres adapter (mapper for new fields)
7. Alert chain (MonitorURL → MonitorTarget)
8. HTTP adapter + schema-driven frontend
9. Rewire cmd/
10. Cleanup + tests

**Adding a new checker after this = 3 steps:**
1. `adapter/checker/newtype.go` — config + Check + Schema + Validate + Target + Host
2. `domain/monitor.go` — add `MonitorNewType` const
3. `cmd/*/main.go` — `registry.Register(domain.MonitorNewType, "Label", checker.NewX())`

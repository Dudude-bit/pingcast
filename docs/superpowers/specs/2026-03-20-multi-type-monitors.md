# Multi-Type Monitor Support

## Overview

Add support for multiple monitor check types (HTTP, TCP, DNS) with a checker registry pattern. Users select the check type when creating a monitor, and type-specific parameters are stored in a JSONB `check_config` field. Each checker is an adapter behind `port.MonitorChecker`, selected at runtime via `port.CheckerRegistry`.

## Domain Changes

### New type `MonitorType` in `domain/monitor.go`:

```go
type MonitorType string

const (
    MonitorHTTP MonitorType = "http"
    MonitorTCP  MonitorType = "tcp"
    MonitorDNS  MonitorType = "dns"
)
```

### CheckConfig — type-safe union in `domain/monitor.go`:

```go
type CheckConfig struct {
    HTTP *HTTPCheckConfig `json:"http,omitempty"`
    TCP  *TCPCheckConfig  `json:"tcp,omitempty"`
    DNS  *DNSCheckConfig  `json:"dns,omitempty"`
}

type HTTPCheckConfig struct {
    URL            string  `json:"url"`
    Method         string  `json:"method"`
    ExpectedStatus int     `json:"expected_status"`
    Keyword        *string `json:"keyword,omitempty"`
}

type TCPCheckConfig struct {
    Host string `json:"host"`
    Port int    `json:"port"`
}

type DNSCheckConfig struct {
    Hostname   string  `json:"hostname"`
    ExpectedIP *string `json:"expected_ip,omitempty"`
    DNSServer  *string `json:"dns_server,omitempty"`
}
```

### Monitor struct — cleaned up:

```go
type Monitor struct {
    ID                 uuid.UUID
    UserID             uuid.UUID
    Name               string
    Type               MonitorType
    CheckConfig        CheckConfig
    IntervalSeconds    int
    AlertAfterFailures int
    IsPaused           bool
    IsPublic           bool
    CurrentStatus      MonitorStatus
    CreatedAt          time.Time
}
```

**Removed fields:** `URL`, `Method`, `ExpectedStatus`, `Keyword` — all moved into `CheckConfig.HTTP`.

### Helper method for display:

```go
func (m Monitor) Target() string {
    switch m.Type {
    case MonitorHTTP:
        if m.CheckConfig.HTTP != nil {
            return m.CheckConfig.HTTP.Method + " " + m.CheckConfig.HTTP.URL
        }
    case MonitorTCP:
        if m.CheckConfig.TCP != nil {
            return fmt.Sprintf("tcp://%s:%d", m.CheckConfig.TCP.Host, m.CheckConfig.TCP.Port)
        }
    case MonitorDNS:
        if m.CheckConfig.DNS != nil {
            return "dns://" + m.CheckConfig.DNS.Hostname
        }
    }
    return ""
}
```

## Database Migration

Expand-contract pattern — two migrations to avoid destructive single-step change:

`internal/database/migrations/006_add_monitor_type_and_config.sql`:

```sql
-- Step 1: Add new columns (non-destructive)
ALTER TABLE monitors
    ADD COLUMN type VARCHAR(10) NOT NULL DEFAULT 'http',
    ADD COLUMN check_config JSONB NOT NULL DEFAULT '{}';

-- Step 2: Backfill existing HTTP monitors
UPDATE monitors SET check_config = jsonb_strip_nulls(jsonb_build_object(
    'http', jsonb_build_object(
        'url', url,
        'method', method,
        'expected_status', expected_status,
        'keyword', keyword
    )
));
```

`internal/database/migrations/007_drop_old_monitor_columns.sql`:

```sql
-- Step 3: Drop old columns (deployed after verifying backfill)
ALTER TABLE monitors
    DROP COLUMN url,
    DROP COLUMN method,
    DROP COLUMN expected_status,
    DROP COLUMN keyword;
```

Uses `jsonb_strip_nulls` to avoid `"keyword": null` entries for monitors without keywords.

### sqlc handling

`check_config` column is `JSONB` in PostgreSQL. sqlc generates it as `[]byte`. Deserialization to `domain.CheckConfig` happens in `adapter/postgres/mapper.go` via `json.Unmarshal`. Serialization via `json.Marshal` when writing.

### Updated sqlc queries

`queries/monitors.sql` — all queries updated:
- Remove `url`, `method`, `expected_status`, `keyword` from column lists
- Add `type`, `check_config`
- `CreateMonitor` params: `type`, `check_config` (as JSONB)
- `UpdateMonitor` params: same changes

## Port Changes

### `port/checker.go` — registry replaces single checker:

```go
type CheckerRegistry interface {
    Get(monitorType domain.MonitorType) (MonitorChecker, error)
}

type MonitorChecker interface {
    Check(ctx context.Context, monitor *domain.Monitor) *domain.CheckResult
}
```

### `MonitoringService` changes:

- Field: `checker port.MonitorChecker` → `registry port.CheckerRegistry`
- Constructor: `NewMonitoringService(..., registry port.CheckerRegistry)`
- `RunCheck` does registry lookup:

```go
func (s *MonitoringService) RunCheck(ctx context.Context, monitor *domain.Monitor) error {
    checker, err := s.registry.Get(monitor.Type)
    if err != nil {
        return fmt.Errorf("no checker for type %q: %w", monitor.Type, err)
    }
    result := checker.Check(ctx, monitor)
    return s.ProcessCheckResult(ctx, monitor, result)
}
```

- `CreateMonitor` validates that a checker exists for the requested type via `s.registry.Get(input.Type)`
- No more `nil` checker — all services receive the registry

### `CreateMonitorInput` changes:

```go
type CreateMonitorInput struct {
    Name               string
    Type               domain.MonitorType
    CheckConfig        domain.CheckConfig
    IntervalSeconds    int
    AlertAfterFailures int
    IsPublic           bool
}
```

Removed: `URL`, `Method`, `ExpectedStatus`, `Keyword`. Added: `Type`, `CheckConfig`.

### `UpdateMonitorInput` changes:

```go
type UpdateMonitorInput struct {
    Name               *string
    CheckConfig        *domain.CheckConfig  // must match existing Type
    IntervalSeconds    *int
    AlertAfterFailures *int
    IsPaused           *bool
    IsPublic           *bool
}
```

`Type` is not in UpdateMonitorInput — type is immutable after creation.

## Adapter — Checker Registry + Three Checkers

### `adapter/checker/registry.go`:

```go
type Registry struct {
    checkers map[domain.MonitorType]port.MonitorChecker
}

func NewRegistry() *Registry
func (r *Registry) Register(t domain.MonitorType, c port.MonitorChecker)
func (r *Registry) Get(t domain.MonitorType) (port.MonitorChecker, error)
```

`var _ port.CheckerRegistry = (*Registry)(nil)`

### `adapter/checker/http.go` (renamed from `client.go`):

Current HTTP checker logic. Changes:
- Reads URL, Method, ExpectedStatus, Keyword from `monitor.CheckConfig.HTTP`
- No longer reads from Monitor fields directly

### `adapter/checker/tcp.go` — new:

```go
type TCPChecker struct {
    timeout time.Duration
}

func NewTCPChecker() *TCPChecker

func (c *TCPChecker) Check(ctx context.Context, monitor *domain.Monitor) *domain.CheckResult {
    cfg := monitor.CheckConfig.TCP
    addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
    // net.DialTimeout → success = up, error = down
    // Record response_time_ms
}
```

### `adapter/checker/dns.go` — new:

```go
type DNSChecker struct{}

func NewDNSChecker() *DNSChecker

func (c *DNSChecker) Check(ctx context.Context, monitor *domain.Monitor) *domain.CheckResult {
    cfg := monitor.CheckConfig.DNS
    // If cfg.DNSServer != nil → custom resolver
    // net.Resolver.LookupHost(ctx, cfg.Hostname)
    // If cfg.ExpectedIP != nil → verify IP in results
    // Record response_time_ms
}
```

### Modified adapter files:

**`worker.go`** — two changes:
1. Replace `port.MonitorChecker` field with `port.CheckerRegistry`. Worker does registry lookup per check: `checker, err := wp.registry.Get(m.Type)`.
2. Replace `extractHost(m.URL)` with `monitor.Host()` — new domain helper (see below).

Alternatively (simpler): WorkerPool no longer calls checker directly. It just calls `CheckHandler` which delegates to `MonitoringService.RunCheck`. WorkerPool becomes a pure scheduler-to-handler dispatcher, no checker dependency at all.

**Recommended approach:** Remove checker from WorkerPool entirely. WorkerPool dispatches monitors to handler. Handler calls `monitoringSvc.RunCheck(ctx, monitor)` which does registry lookup + check + process result. WorkerPool only needs domain types and a handler callback.

**`hostlimit.go`** — unchanged, but `extractHost` needs a type-aware replacement.

### New domain helper `Monitor.Host()`:

```go
func (m Monitor) Host() string {
    switch m.Type {
    case MonitorHTTP:
        if m.CheckConfig.HTTP != nil {
            u, err := url.Parse(m.CheckConfig.HTTP.URL)
            if err == nil {
                return u.Host
            }
        }
    case MonitorTCP:
        if m.CheckConfig.TCP != nil {
            return m.CheckConfig.TCP.Host
        }
    case MonitorDNS:
        if m.CheckConfig.DNS != nil {
            return m.CheckConfig.DNS.Hostname
        }
    }
    return ""
}
```

Note: This adds `net/url` import to domain — only stdlib, still hex-compliant.

**`scheduler.go`** — unchanged (type-agnostic, works with `*domain.Monitor`)

## Composition Root Changes

### `cmd/checker/main.go`:

```go
registry := checker.NewRegistry()
registry.Register(domain.MonitorHTTP, checker.NewHTTPChecker())
registry.Register(domain.MonitorTCP, checker.NewTCPChecker())
registry.Register(domain.MonitorDNS, checker.NewDNSChecker())

monitoringSvc := app.NewMonitoringService(..., registry)
```

### `cmd/api/main.go`:

Same registry — needed for `CreateMonitor` validation (reject unknown types).

```go
registry := checker.NewRegistry()
registry.Register(domain.MonitorHTTP, checker.NewHTTPChecker())
registry.Register(domain.MonitorTCP, checker.NewTCPChecker())
registry.Register(domain.MonitorDNS, checker.NewDNSChecker())

monitoringSvc := app.NewMonitoringService(..., registry)
```

### `cmd/notifier/main.go`:

No changes — notifier doesn't know about checker types.

## Postgres Adapter Changes

### `adapter/postgres/mapper.go`:

New functions:
- `checkConfigToJSON(cfg domain.CheckConfig) ([]byte, error)` — marshal for write
- `checkConfigFromJSON(data []byte) (domain.CheckConfig, error)` — unmarshal for read
- Update `toDomainMonitor` — reads `type` and `check_config`, drops old fields

### `adapter/postgres/monitor_repo.go`:

- `Create` — marshals `CheckConfig` to JSON before passing to sqlc
- `Update` — same
- All read methods — unmarshal `check_config` bytes to `domain.CheckConfig`

## OpenAPI Changes

### `api/openapi.yaml`:

```yaml
CreateMonitorRequest:
  required: [name, type, check_config]
  properties:
    name:
      type: string
    type:
      type: string
      enum: [http, tcp, dns]
    check_config:
      $ref: "#/components/schemas/CheckConfig"
    interval_seconds:
      type: integer
    alert_after_failures:
      type: integer
    is_public:
      type: boolean

CheckConfig:
  type: object
  properties:
    http:
      $ref: "#/components/schemas/HTTPCheckConfig"
    tcp:
      $ref: "#/components/schemas/TCPCheckConfig"
    dns:
      $ref: "#/components/schemas/DNSCheckConfig"

HTTPCheckConfig:
  type: object
  required: [url]
  properties:
    url:
      type: string
    method:
      type: string
      enum: [GET, POST]
      default: GET
    expected_status:
      type: integer
      default: 200
    keyword:
      type: string
      nullable: true

TCPCheckConfig:
  type: object
  required: [host, port]
  properties:
    host:
      type: string
    port:
      type: integer

DNSCheckConfig:
  type: object
  required: [hostname]
  properties:
    hostname:
      type: string
    expected_ip:
      type: string
      nullable: true
    dns_server:
      type: string
      nullable: true

Monitor:
  type: object
  properties:
    id:
      type: string
      format: uuid
    name:
      type: string
    type:
      type: string
      enum: [http, tcp, dns]
    check_config:
      $ref: "#/components/schemas/CheckConfig"
    target:
      type: string
      description: "Display string: 'GET https://...', 'tcp://host:port', 'dns://hostname'"
    interval_seconds:
      type: integer
    alert_after_failures:
      type: integer
    is_paused:
      type: boolean
    is_public:
      type: boolean
    current_status:
      type: string
      enum: [up, down, unknown]
    created_at:
      type: string
      format: date-time
```

Removed from Monitor: `url`, `method`, `expected_status`, `keyword`. Added: `type`, `check_config`, `target`.

## Alert Chain Changes

`domain.AlertEvent.MonitorURL` → `MonitorTarget`. Set via `monitor.Target()` in `publishAlert`.

`port.AlertSender` signature: `NotifyDown(ctx, monitorName, monitorTarget, cause)` — parameter renamed from `monitorURL` to `monitorTarget`.

Telegram/SMTP message templates: "URL:" → "Target:" to correctly label TCP/DNS targets.

## Validation

### `CheckConfig.Validate(monitorType)` — domain method:

```go
func (c CheckConfig) Validate(t MonitorType) error {
    switch t {
    case MonitorHTTP:
        if c.HTTP == nil { return errors.New("http config required") }
        if c.HTTP.URL == "" { return errors.New("url required") }
    case MonitorTCP:
        if c.TCP == nil { return errors.New("tcp config required") }
        if c.TCP.Host == "" { return errors.New("host required") }
        if c.TCP.Port <= 0 || c.TCP.Port > 65535 { return errors.New("invalid port") }
    case MonitorDNS:
        if c.DNS == nil { return errors.New("dns config required") }
        if c.DNS.Hostname == "" { return errors.New("hostname required") }
    default:
        return fmt.Errorf("unknown monitor type: %s", t)
    }
    return nil
}
```

Called in `MonitoringService.CreateMonitor` and `UpdateMonitor`.

### Type changes on update:

Changing `Type` on an existing monitor is **disallowed**. To change type: delete and re-create. This avoids CheckConfig/Type mismatch bugs.

`UpdateMonitorInput.Type` is removed — type is immutable after creation.

### `MonitorType.Valid()` helper:

```go
func (t MonitorType) Valid() bool {
    switch t {
    case MonitorHTTP, MonitorTCP, MonitorDNS:
        return true
    }
    return false
}
```

## `HTTPCheckConfig.Method` type

Uses `domain.HTTPMethod` (existing typed enum) instead of plain string:

```go
type HTTPCheckConfig struct {
    URL            string     `json:"url"`
    Method         HTTPMethod `json:"method"`
    ExpectedStatus int        `json:"expected_status"`
    Keyword        *string    `json:"keyword,omitempty"`
}
```

## MonitorConfigFields input validation

```go
func (h *PageHandler) MonitorConfigFields(c *fiber.Ctx) error {
    monitorType := domain.MonitorType(c.Query("type", "http"))
    if !monitorType.Valid() {
        return c.Status(400).SendString("invalid monitor type")
    }
    return h.render(c, "monitor_config_"+string(monitorType)+".html", nil)
}
```

Validates against known enum before template lookup — prevents path traversal.

## Frontend Changes

### Monitor form — dynamic fields via HTMX:

```html
<select name="type" hx-get="/monitors/config-fields" hx-target="#config-fields" hx-include="[name='type']">
    <option value="http">HTTP</option>
    <option value="tcp">TCP</option>
    <option value="dns">DNS</option>
</select>

<div id="config-fields">
    <!-- Loaded via HTMX based on selected type -->
</div>
```

New endpoint `GET /monitors/config-fields?type=tcp` returns HTML fragment with type-specific form fields.

### Dashboard — shows `monitor.Target()` instead of URL:

```html
<div class="url">{{.Monitor.Target}}</div>
```

### Templates updated:
- `monitor_form.html` — type selector + dynamic config fields
- `dashboard.html` — `.Monitor.URL` → `.Monitor.Target`
- `monitor_detail.html` — `.Monitor.URL` → `.Monitor.Target`

### New page handler:

```go
func (h *PageHandler) MonitorConfigFields(c *fiber.Ctx) error {
    monitorType := c.Query("type", "http")
    return h.render(c, "monitor_config_"+monitorType+".html", nil)
}
```

### New template partials:
- `templates/monitor_config_http.html` — URL, Method, Expected Status, Keyword
- `templates/monitor_config_tcp.html` — Host, Port
- `templates/monitor_config_dns.html` — Hostname, Expected IP, DNS Server

## Files Changed

### New files:
- `internal/database/migrations/006_add_monitor_type.sql`
- `internal/adapter/checker/registry.go`
- `internal/adapter/checker/tcp.go`
- `internal/adapter/checker/dns.go`
- `internal/web/templates/monitor_config_http.html`
- `internal/web/templates/monitor_config_tcp.html`
- `internal/web/templates/monitor_config_dns.html`

### Modified files:
- `internal/domain/monitor.go` — MonitorType, CheckConfig, remove URL/Method/ExpectedStatus/Keyword from Monitor
- `internal/port/checker.go` — CheckerRegistry interface replaces MonitorChecker alone
- `internal/app/monitoring.go` — registry field, RunCheck with lookup, CreateMonitor validates type
- `internal/adapter/checker/http.go` (renamed from client.go) — reads from CheckConfig.HTTP
- `internal/adapter/checker/client_test.go` — updated for new Monitor structure
- `internal/adapter/postgres/mapper.go` — CheckConfig JSON marshal/unmarshal
- `internal/adapter/postgres/monitor_repo.go` — handle check_config JSONB
- `internal/adapter/http/server.go` — API type mappings updated
- `internal/adapter/http/pages.go` — MonitorConfigFields handler, form handling
- `internal/adapter/http/setup.go` — new route for config fields
- `internal/sqlc/queries/monitors.sql` — new columns, removed old
- `api/openapi.yaml` — CheckConfig schemas, Monitor schema updated
- `internal/web/templates/monitor_form.html` — type selector + HTMX
- `internal/web/templates/dashboard.html` — .Monitor.Target instead of .Monitor.URL
- `internal/web/templates/monitor_detail.html` — same
- `cmd/api/main.go` — create registry, pass to MonitoringService
- `cmd/checker/main.go` — create registry with all checkers, pass to MonitoringService

### Deleted files:
- `internal/adapter/checker/client.go` — renamed to `http.go`

### Unchanged:
- `internal/adapter/checker/scheduler.go` — type-agnostic
- `internal/adapter/checker/worker.go` — type-agnostic
- `internal/adapter/checker/hostlimit.go`
- `internal/adapter/nats/` — events carry domain.Monitor, type-agnostic
- `internal/adapter/telegram/`, `internal/adapter/smtp/` — alert senders, unchanged
- `internal/app/auth.go`, `internal/app/alert.go`
- `internal/domain/user.go`, `internal/domain/incident.go`, `internal/domain/alert.go`
- `cmd/notifier/main.go`

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

### CheckConfig — raw JSON bytes in `domain/monitor.go`:

Domain stores raw JSON — does NOT know about specific config structures. Each checker adapter owns its config type and deserializes from raw bytes.

```go
import "encoding/json"

// Monitor stores type-specific config as raw JSON.
// Deserialization is the checker adapter's responsibility.
type Monitor struct {
    ID                 uuid.UUID
    UserID             uuid.UUID
    Name               string
    Type               MonitorType
    CheckConfig        json.RawMessage  // raw JSON, adapter deserializes
    IntervalSeconds    int
    AlertAfterFailures int
    IsPaused           bool
    IsPublic           bool
    CurrentStatus      MonitorStatus
    CreatedAt          time.Time
}
```

**Removed fields:** `URL`, `Method`, `ExpectedStatus`, `Keyword` — moved to adapter-specific config structs.

`json.RawMessage` is `[]byte` from stdlib `encoding/json` — no external deps, hex-compliant.

### Config structs live in adapters, NOT in domain:

```go
// adapter/checker/http.go
type HTTPCheckConfig struct {
    URL            string       `json:"url"`
    Method         HTTPMethod   `json:"method"`
    ExpectedStatus int          `json:"expected_status"`
    Keyword        *string      `json:"keyword,omitempty"`
}

// adapter/checker/tcp.go
type TCPCheckConfig struct {
    Host string `json:"host"`
    Port int    `json:"port"`
}

// adapter/checker/dns.go
type DNSCheckConfig struct {
    Hostname   string  `json:"hostname"`
    ExpectedIP *string `json:"expected_ip,omitempty"`
    DNSServer  *string `json:"dns_server,omitempty"`
}
```

Each checker deserializes `monitor.CheckConfig` into its own struct:
```go
func (c *HTTPChecker) Check(ctx context.Context, monitor *domain.Monitor) *domain.CheckResult {
    var cfg HTTPCheckConfig
    if err := json.Unmarshal(monitor.CheckConfig, &cfg); err != nil {
        // return down with error
    }
    // use cfg.URL, cfg.Method, etc.
}
```

### `Target()` and `Host()` — moved to port:

Since domain doesn't know about config structures, `Target()` and `Host()` can't live on Monitor directly. Instead, the `MonitorChecker` port gets two new optional methods, or we add a new port:

```go
// port/checker.go
type MonitorChecker interface {
    Check(ctx context.Context, monitor *domain.Monitor) *domain.CheckResult
    ValidateConfig(raw json.RawMessage) error  // validates config at creation time
    Target(raw json.RawMessage) string         // display string: "GET https://...", "tcp://host:port"
    Host(raw json.RawMessage) string           // for host rate-limiting
}
```

This way each checker knows how to extract target/host from its own config. The registry delegates:

```go
// Called by app layer or adapter when display string is needed
func (r *Registry) Target(monitorType domain.MonitorType, config json.RawMessage) string {
    checker, err := r.Get(monitorType)
    if err != nil {
        return ""
    }
    return checker.Target(config)
}
```

### Adding a new checker type requires:
1. Add const `MonitorNewType` to `domain/monitor.go`
2. Create `adapter/checker/newtype.go` with config struct + `Check` + `ValidateConfig` + `ConfigSchema` + `Target` + `Host`
3. Register in `cmd/*/main.go`: `registry.Register(domain.MonitorNewType, "New Type", checker.NewNewTypeChecker())`
4. No port changes, no frontend changes, no migration — frontend renders form dynamically from ConfigSchema

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
    Types() []MonitorTypeInfo                                       // registered types for UI
    ValidateConfig(monitorType domain.MonitorType, raw json.RawMessage) error
    Target(monitorType domain.MonitorType, raw json.RawMessage) string
    Host(monitorType domain.MonitorType, raw json.RawMessage) string
}

type MonitorTypeInfo struct {
    Type   domain.MonitorType
    Label  string             // "HTTP", "TCP", "DNS" — human-readable
    Schema ConfigSchema       // field definitions for dynamic form rendering
}

type ConfigSchema struct {
    Fields []ConfigField `json:"fields"`
}

type ConfigField struct {
    Name        string   `json:"name"`                   // "url", "host", "port"
    Label       string   `json:"label"`                  // "URL", "Host", "Port"
    Type        string   `json:"type"`                   // "text", "number", "select"
    Required    bool     `json:"required"`
    Default     any      `json:"default,omitempty"`
    Placeholder string   `json:"placeholder,omitempty"`
    Options     []Option `json:"options,omitempty"`       // for select fields
}

type Option struct {
    Value string `json:"value"`
    Label string `json:"label"`
}

type MonitorChecker interface {
    Check(ctx context.Context, monitor *domain.Monitor) *domain.CheckResult
    ValidateConfig(raw json.RawMessage) error
    ConfigSchema() ConfigSchema
    Target(raw json.RawMessage) string
    Host(raw json.RawMessage) string
}
```

Registry delegates all methods to the appropriate checker by type. `Types()` returns info for all registered checkers — used by API to build the monitor-types endpoint and by frontend to render forms dynamically.

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
    CheckConfig        json.RawMessage  // raw JSON, validated via registry
    IntervalSeconds    int
    AlertAfterFailures int
    IsPublic           bool
}
```

`CreateMonitor` validates config: `s.registry.ValidateConfig(input.Type, input.CheckConfig)`

Removed: `URL`, `Method`, `ExpectedStatus`, `Keyword`. Added: `Type`, `CheckConfig`.

### `UpdateMonitorInput` changes:

```go
type UpdateMonitorInput struct {
    Name               *string
    CheckConfig        json.RawMessage  // if provided, validated against existing Type
    IntervalSeconds    *int
    AlertAfterFailures *int
    IsPaused           *bool
    IsPublic           *bool
}
```

`Type` is not in UpdateMonitorInput — type is immutable after creation. If `CheckConfig` is provided, it is validated against the monitor's existing `Type` via `registry.ValidateConfig`.

## Adapter — Checker Registry + Three Checkers

### `adapter/checker/registry.go`:

```go
type registryEntry struct {
    label   string
    checker port.MonitorChecker
}

type Registry struct {
    entries map[domain.MonitorType]registryEntry
}

func NewRegistry() *Registry
func (r *Registry) Register(t domain.MonitorType, label string, c port.MonitorChecker)
func (r *Registry) Get(t domain.MonitorType) (port.MonitorChecker, error)
func (r *Registry) Types() []port.MonitorTypeInfo  // builds from entries, includes ConfigSchema from each checker
func (r *Registry) ValidateConfig(t domain.MonitorType, raw json.RawMessage) error  // delegates to checker
func (r *Registry) Target(t domain.MonitorType, raw json.RawMessage) string
func (r *Registry) Host(t domain.MonitorType, raw json.RawMessage) string
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
registry.Register(domain.MonitorHTTP, "HTTP", checker.NewHTTPChecker())
registry.Register(domain.MonitorTCP, "TCP", checker.NewTCPChecker())
registry.Register(domain.MonitorDNS, "DNS", checker.NewDNSChecker())

monitoringSvc := app.NewMonitoringService(..., registry)
```

### `cmd/api/main.go`:

Same registry — needed for validation + `/api/monitor-types` endpoint.

```go
registry := checker.NewRegistry()
registry.Register(domain.MonitorHTTP, "HTTP", checker.NewHTTPChecker())
registry.Register(domain.MonitorTCP, "TCP", checker.NewTCPChecker())
registry.Register(domain.MonitorDNS, "DNS", checker.NewDNSChecker())

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
  description: "Type-specific config. Structure depends on monitor type."
  additionalProperties: true

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

### Config validation — each checker adapter validates its own config:

```go
// adapter/checker/http.go
func (c *HTTPChecker) ValidateConfig(raw json.RawMessage) error {
    var cfg HTTPCheckConfig
    if err := json.Unmarshal(raw, &cfg); err != nil {
        return fmt.Errorf("invalid http config: %w", err)
    }
    if cfg.URL == "" { return errors.New("url required") }
    return nil
}

// adapter/checker/tcp.go
func (c *TCPChecker) ValidateConfig(raw json.RawMessage) error {
    var cfg TCPCheckConfig
    if err := json.Unmarshal(raw, &cfg); err != nil {
        return fmt.Errorf("invalid tcp config: %w", err)
    }
    if cfg.Host == "" { return errors.New("host required") }
    if cfg.Port <= 0 || cfg.Port > 65535 { return errors.New("invalid port") }
    return nil
}

// adapter/checker/dns.go
func (c *DNSChecker) ValidateConfig(raw json.RawMessage) error {
    var cfg DNSCheckConfig
    if err := json.Unmarshal(raw, &cfg); err != nil {
        return fmt.Errorf("invalid dns config: %w", err)
    }
    if cfg.Hostname == "" { return errors.New("hostname required") }
    return nil
}
```

App layer calls `registry.ValidateConfig(type, raw)` — registry delegates to the right checker. Domain has no validation logic for configs.

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

`HTTPMethod` type moves from domain to `adapter/checker/http.go` — it's HTTP-specific, not a domain concern. Domain only knows about `MonitorType`.

```go
// adapter/checker/http.go
type HTTPMethod string
const (
    MethodGET  HTTPMethod = "GET"
    MethodPOST HTTPMethod = "POST"
)

type HTTPCheckConfig struct {
    URL            string     `json:"url"`
    Method         HTTPMethod `json:"method"`
    ExpectedStatus int        `json:"expected_status"`
    Keyword        *string    `json:"keyword,omitempty"`
}
```

`domain.HTTPMethod` is deleted from domain/monitor.go.

## MonitorConfigFields input validation

Handled in the page handler (see Frontend Changes section). Validates type against registry — no path traversal possible since there's only one universal template. Unknown types get 404.

## Frontend Changes

### API endpoint for monitor types:

`GET /api/monitor-types` — returns registered types with config schemas:

```json
[
  {
    "type": "http",
    "label": "HTTP",
    "fields": [
      {"name": "url", "label": "URL", "type": "text", "required": true, "placeholder": "https://example.com/health"},
      {"name": "method", "label": "Method", "type": "select", "default": "GET", "options": [{"value": "GET", "label": "GET"}, {"value": "POST", "label": "POST"}]},
      {"name": "expected_status", "label": "Expected Status", "type": "number", "default": 200},
      {"name": "keyword", "label": "Keyword", "type": "text", "placeholder": "optional"}
    ]
  },
  {
    "type": "tcp",
    "label": "TCP",
    "fields": [
      {"name": "host", "label": "Host", "type": "text", "required": true, "placeholder": "db.example.com"},
      {"name": "port", "label": "Port", "type": "number", "required": true, "placeholder": "5432"}
    ]
  }
]
```

Served by `Server.ListMonitorTypes` which calls `registry.Types()`.

### Monitor form — fully dynamic, no hardcoded types:

Type selector populated from API:

```html
<select name="type" hx-get="/monitors/config-fields" hx-target="#config-fields" hx-include="[name='type']">
    {{range .MonitorTypes}}
    <option value="{{.Type}}">{{.Label}}</option>
    {{end}}
</select>

<div id="config-fields">
    <!-- HTMX loads fields dynamically -->
</div>
```

### Config fields endpoint — renders from schema, no per-type templates:

`GET /monitors/config-fields?type=tcp` returns HTML fragment rendered from a single universal template:

```html
<!-- templates/monitor_config_fields.html — ONE template for ALL types -->
{{range .Fields}}
<div class="form-group">
    <label for="config.{{.Name}}">{{.Label}}</label>
    {{if eq .Type "select"}}
        <select id="config.{{.Name}}" name="config.{{.Name}}">
            {{range .Options}}<option value="{{.Value}}">{{.Label}}</option>{{end}}
        </select>
    {{else}}
        <input type="{{.Type}}" id="config.{{.Name}}" name="config.{{.Name}}"
            {{if .Required}}required{{end}}
            {{if .Placeholder}}placeholder="{{.Placeholder}}"{{end}}
            {{if .Default}}value="{{.Default}}"{{end}}>
    {{end}}
</div>
{{end}}
```

### Page handler — schema-driven, validates type against registry:

```go
func (h *PageHandler) MonitorConfigFields(c *fiber.Ctx) error {
    monitorType := domain.MonitorType(c.Query("type", "http"))
    if !monitorType.Valid() {
        return c.Status(400).SendString("invalid monitor type")
    }
    types := h.monitoring.Registry().Types()
    for _, t := range types {
        if t.Type == monitorType {
            return h.render(c, "monitor_config_fields.html", fiber.Map{"Fields": t.Schema.Fields})
        }
    }
    return c.Status(404).SendString("unknown monitor type")
}
```

### Dashboard — shows target from registry:

Templates use `.Target` field computed by the HTTP adapter via `registry.Target(monitor.Type, monitor.CheckConfig)`:

```html
<div class="url">{{.Target}}</div>
```

### Templates:
- `monitor_form.html` — type selector from data, HTMX dynamic fields
- `monitor_config_fields.html` — ONE universal template, renders from ConfigSchema
- `dashboard.html` — `.Target` instead of `.Monitor.URL`
- `monitor_detail.html` — `.Target` instead of `.Monitor.URL`

### Deleted templates:
- `monitor_config_http.html` — not needed, universal template replaces all
- `monitor_config_tcp.html` — same
- `monitor_config_dns.html` — same

## Files Changed

### New files:
- `internal/database/migrations/006_add_monitor_type_and_config.sql`
- `internal/database/migrations/007_drop_old_monitor_columns.sql`
- `internal/adapter/checker/registry.go`
- `internal/adapter/checker/tcp.go`
- `internal/adapter/checker/dns.go`
- `internal/web/templates/monitor_config_fields.html` — ONE universal schema-driven template

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

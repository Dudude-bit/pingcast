# C1 — API Contract Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a black-box integration test suite (`tests/integration/api/`) covering all 27 HTTP endpoints with ~95 TDD tests. Spec drives behavior; code bends to spec.

**Architecture:** Testcontainers-based (Postgres 16 + Redis 7 + NATS JetStream) shared across a package's lifetime, per-test `TRUNCATE` + `FLUSHDB` isolation, in-process Fiber `app.Test()` (no TCP listener), session-cookie fixtures via real API calls, injectable `Clock` and `Random` ports for determinism, and fakes for outbound SMTP/Telegram/Webhook so C2 can reuse the harness.

**Tech Stack:** Go 1.26, Fiber v2, testcontainers-go, pgx/v5, redis/v9, NATS JetStream, stretchr/testify, goose migrations, oapi-codegen (existing), golangci-lint.

**Companion documents:**
- `docs/superpowers/specs/2026-04-18-C1-api-contract-tests-design.md` (design)
- `docs/superpowers/specs/2026-04-18-C1-api-behavior-spec.md` (normative spec — the oracle for tests)

---

## File Structure

### Created

```
tests/integration/api/
  harness/
    testmain.go            — TestMain; boots/tears down containers
    containers.go          — testcontainers wrappers (postgres, redis, nats)
    app.go                 — in-process Fiber server factory (wires real adapters + test doubles)
    migrate.go             — runs goose migrations on the test Postgres
    truncate.go            — reset state between tests
    session.go             — Session (cookie jar) + Register/Login/HTTP helpers
    fixtures.go            — MustCreateMonitor / MustCreateChannel / TwoSessions
    fakes.go               — FakeSMTP, FakeTelegram, FakeWebhookSink (C2-ready)
    assert.go              — AssertError, AssertJSONContains, AssertEnvelope
    new.go                 — harness.New(t) entry point
  auth_test.go             — register, login, logout + edge cases
  monitors_test.go         — CRUD + pause + cross-tenant
  channels_test.go         — CRUD + GET-by-id + cross-tenant + redaction
  monitor_channels_test.go — bind, unbind, idempotency
  apikeys_test.go          — list/create/delete + key-based auth path
  status_test.go           — public status page JSON
  health_test.go           — /health, /healthz, /readyz
  webhook_test.go          — lemonsqueezy HMAC, telegram token
  authmatrix_test.go       — every protected endpoint × [no-auth, bad-cookie, revoked-key, cross-tenant]
  errorenv_test.go         — error-envelope shape on every handled 4xx/5xx

internal/port/clock.go     — new Clock port
internal/port/random.go    — new Random port
internal/adapter/sysclock/ — stdlib clock impl
internal/adapter/sysrand/  — stdlib rng impl

.github/workflows/integration.yml — CI job for the new suite
```

### Modified

- `internal/adapter/http/server.go` — thread `Clock` and `Random` through handlers; replace raw `time.Now()` / `crypto/rand` calls.
- `internal/adapter/http/setup.go` — remove dead `POST /logout` page handler.
- `internal/app/auth.go` — take `Clock` and `Random` dependencies; stop importing `time` directly for session expiry.
- `internal/app/monitoring.go` — take `Clock`; replace `time.Now()`.
- `internal/app/alert.go` — new `GetChannelByID` method for `GET /api/channels/{id}`.
- `internal/adapter/httperr/response.go` — single canonical envelope emitter; fix 400-vs-422 boundary.
- `internal/adapter/http/server.go` (handlers) — replace all ad-hoc `{Error: string}` returns with the canonical envelope via `httperr.Write(c, err)`.
- `api/openapi.yaml` — add `GET /api/channels/{id}`, remove nothing (page `/logout` was never in OpenAPI).
- `cmd/api/main.go` — wire `sysclock` and `sysrand` in composition root.
- `README.md` — add new testing row; document `make test-integration`.
- `Makefile` (or create) — add `test-integration` target.

### Moved

- `tests/integration/repo_test.go` → `internal/adapter/postgres/repo_integration_test.go` with `//go:build integration` tag. (Task 30.)

---

## Execution Conventions

- **Every code step shows the complete file or a precise unified hunk with a stable anchor.** No “add error handling” hand-waving.
- **Every test step names the test, shows its full body, and states the expected `go test` output before and after the implementation step.**
- **Commit after each green state.** Commit messages use Conventional Commits scoped to `test(c1)`, `feat(c1)`, `refactor(c1)`, `chore(c1)`.
- **Run commands from repo root unless stated otherwise.** `go` commands: Go 1.26. `docker compose` must be up for testcontainers (the tests spawn their own — docker daemon is enough).
- **Build tag:** every file under `tests/integration/api/` and the moved `repo_integration_test.go` must start with `//go:build integration` on line 1.
- **Run pattern:** `go test -tags=integration ./tests/integration/api/... -run TestName -v`.

---

# Phase 0 — Harness scaffolding

## Task 1: Create harness skeleton + build tag enforcement

**Files:**
- Create: `tests/integration/api/harness/new.go`
- Create: `tests/integration/api/harness/testmain.go`
- Create: `tests/integration/api/harness/.keep` (placeholder for empty dir initially — deleted in Task 3)
- Create: `tests/integration/api/_init_test.go`

- [ ] **Step 1: Create placeholder TestMain**

`tests/integration/api/harness/testmain.go`:

```go
//go:build integration

package harness

import (
	"os"
	"testing"
)

// TestMain is the package-wide setup/teardown entry point for the
// integration harness. Subsequent tasks will replace the body with
// actual container boot/shutdown.
func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}
```

- [ ] **Step 2: Create empty Harness entry-point**

`tests/integration/api/harness/new.go`:

```go
//go:build integration

package harness

import "testing"

// Harness is the per-test handle. Concrete fields are added in
// subsequent tasks (App, Pool, Redis, NATS, FakeSMTP, FakeTelegram,
// FakeWebhookSink, Clock, Random).
type Harness struct {
	t *testing.T
}

// New returns a Harness scoped to the current test. Container
// provisioning happens in TestMain; this call only resets state.
func New(t *testing.T) *Harness {
	t.Helper()
	return &Harness{t: t}
}
```

- [ ] **Step 3: Create a canary test that proves the build tag filters unit runs**

`tests/integration/api/_init_test.go`:

```go
//go:build integration

package api

import (
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestHarness_Init(t *testing.T) {
	h := harness.New(t)
	if h == nil {
		t.Fatal("harness.New returned nil")
	}
}
```

- [ ] **Step 4: Verify `-short` skips, `-tags=integration` runs**

Run: `go test -short ./tests/integration/api/...`
Expected: `ok ... no test files`

Run: `go test -tags=integration ./tests/integration/api/... -run TestHarness_Init -v`
Expected: `PASS: TestHarness_Init`

- [ ] **Step 5: Commit**

```bash
git add tests/integration/api/
git commit -m "test(c1): scaffold integration harness skeleton"
```

---

## Task 2: Add testcontainers dependency (if not present) and bootstrap Postgres

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `tests/integration/api/harness/containers.go`

- [ ] **Step 1: Check existing deps**

Run: `grep -q 'testcontainers-go/modules/postgres' go.mod && echo PRESENT || echo MISSING`

If MISSING:
```bash
go get github.com/testcontainers/testcontainers-go@v0.30.0
go get github.com/testcontainers/testcontainers-go/modules/postgres@v0.30.0
go get github.com/testcontainers/testcontainers-go/modules/redis@v0.30.0
go mod tidy
```

If PRESENT, skip to Step 2.

- [ ] **Step 2: Write containers.go**

`tests/integration/api/harness/containers.go`:

```go
//go:build integration

package harness

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

type Containers struct {
	PostgresURL string
	RedisURL    string
	NATSURL     string

	teardown []func(context.Context) error
}

func StartContainers(ctx context.Context) (*Containers, error) {
	c := &Containers{}

	pg, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("pingcast_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("start postgres: %w", err)
	}
	c.teardown = append(c.teardown, pg.Terminate)

	pgURL, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, fmt.Errorf("postgres url: %w", err)
	}
	c.PostgresURL = pgURL

	rd, err := tcredis.Run(ctx, "redis:7-alpine")
	if err != nil {
		_ = c.Close(ctx)
		return nil, fmt.Errorf("start redis: %w", err)
	}
	c.teardown = append(c.teardown, rd.Terminate)

	rdURL, err := rd.ConnectionString(ctx)
	if err != nil {
		_ = c.Close(ctx)
		return nil, fmt.Errorf("redis url: %w", err)
	}
	c.RedisURL = rdURL

	nats, err := startNATS(ctx)
	if err != nil {
		_ = c.Close(ctx)
		return nil, fmt.Errorf("start nats: %w", err)
	}
	c.teardown = append(c.teardown, nats.Terminate)

	c.NATSURL, err = nats.Endpoint(ctx, "nats")
	if err != nil {
		_ = c.Close(ctx)
		return nil, fmt.Errorf("nats url: %w", err)
	}

	return c, nil
}

func (c *Containers) Close(ctx context.Context) error {
	var first error
	for i := len(c.teardown) - 1; i >= 0; i-- {
		if err := c.teardown[i](ctx); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func startNATS(ctx context.Context) (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
		Image:        "nats:2.10-alpine",
		Cmd:          []string{"-js"},
		ExposedPorts: []string{"4222/tcp"},
		WaitingFor:   wait.ForLog("Server is ready"),
	}
	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
}

// helper for tests that want a short-lived context
func WithTimeout(parent context.Context, d time.Duration, t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(parent, d)
}
```

- [ ] **Step 3: Update TestMain to boot containers**

Rewrite `tests/integration/api/harness/testmain.go`:

```go
//go:build integration

package harness

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"
)

var global *globalState

type globalState struct {
	containers *Containers
}

func TestMain(m *testing.M) {
	if err := setup(); err != nil {
		fmt.Fprintf(os.Stderr, "harness setup failed: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	if err := teardown(); err != nil {
		slog.Error("harness teardown", "error", err)
	}

	os.Exit(code)
}

func setup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	c, err := StartContainers(ctx)
	if err != nil {
		return err
	}
	global = &globalState{containers: c}
	return nil
}

func teardown() error {
	if global == nil || global.containers == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return global.containers.Close(ctx)
}

// Containers exposes the shared container URLs. Nil before TestMain
// runs (never in a test body).
func GetContainers() *Containers {
	return global.containers
}
```

- [ ] **Step 4: Extend the init test to assert containers booted**

Rewrite `tests/integration/api/_init_test.go`:

```go
//go:build integration

package api

import (
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestHarness_ContainersBooted(t *testing.T) {
	c := harness.GetContainers()
	if c == nil {
		t.Fatal("containers not initialized")
	}
	if c.PostgresURL == "" {
		t.Error("postgres url empty")
	}
	if c.RedisURL == "" {
		t.Error("redis url empty")
	}
	if c.NATSURL == "" {
		t.Error("nats url empty")
	}
}
```

- [ ] **Step 5: Run to confirm all three containers boot**

Run: `go test -tags=integration ./tests/integration/api/... -run TestHarness_ContainersBooted -v -timeout 3m`
Expected: `PASS` after ~20-40s first-time image pull.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum tests/integration/api/
git commit -m "test(c1): boot postgres, redis, nats via testcontainers"
```

---

## Task 3: Migrate Postgres schema via goose in TestMain

**Files:**
- Create: `tests/integration/api/harness/migrate.go`
- Modify: `tests/integration/api/harness/testmain.go`
- Delete: `tests/integration/api/harness/.keep`

- [ ] **Step 1: Write migrate.go**

`tests/integration/api/harness/migrate.go`:

```go
//go:build integration

package harness

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"runtime"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// RunMigrations applies all up migrations to the target Postgres URL.
func RunMigrations(pgURL string) error {
	db, err := sql.Open("pgx", pgURL)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	migrationsDir, err := findMigrations()
	if err != nil {
		return err
	}

	if err := goose.Up(db, migrationsDir); err != nil {
		return fmt.Errorf("goose up (dir=%s): %w", migrationsDir, err)
	}
	return nil
}

// findMigrations locates internal/database/migrations relative to this
// source file; resistant to CWD differences between `go test` invocations.
func findMigrations() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime.Caller failed")
	}
	// thisFile = <repo>/tests/integration/api/harness/migrate.go
	repo := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "..", ".."))
	return filepath.Join(repo, "internal", "database", "migrations"), nil
}
```

- [ ] **Step 2: Wire migrations into setup()**

Edit `tests/integration/api/harness/testmain.go` — replace `setup()`:

```go
func setup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	c, err := StartContainers(ctx)
	if err != nil {
		return err
	}

	if err := RunMigrations(c.PostgresURL); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	global = &globalState{containers: c}
	return nil
}
```

- [ ] **Step 3: Write a test asserting schema exists**

Append to `tests/integration/api/_init_test.go`:

```go
func TestHarness_SchemaMigrated(t *testing.T) {
	ctx := t.Context()
	pool, err := pgxpool.New(ctx, harness.GetContainers().PostgresURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public'`).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count == 0 {
		t.Fatal("migrations did not create any tables")
	}
}
```

Add import:
```go
import (
	...
	"github.com/jackc/pgx/v5/pgxpool"
)
```

- [ ] **Step 4: Run**

Run: `go test -tags=integration ./tests/integration/api/... -run TestHarness_SchemaMigrated -v`
Expected: PASS, > 0 tables reported.

- [ ] **Step 5: Remove the .keep placeholder & commit**

```bash
rm -f tests/integration/api/harness/.keep
git add -A tests/integration/api/
git commit -m "test(c1): run goose migrations on test postgres"
```

---

## Task 4: Add in-process Fiber app factory

**Files:**
- Create: `tests/integration/api/harness/app.go`
- Create: `tests/integration/api/harness/clock.go` (test-only impl, until Task 11)
- Create: `tests/integration/api/harness/random.go` (test-only impl, until Task 14)

Note: this task introduces Harness.App so tests can issue HTTP calls. The production-side `Clock` and `Random` ports don't exist yet — we declare test-side equivalents here and wire them into `SetupApp` *through a per-test config hook* added in this task. The real ports come in Tasks 8-14.

- [ ] **Step 1: Write a FakeClock scoped to tests**

`tests/integration/api/harness/clock.go`:

```go
//go:build integration

package harness

import (
	"sync"
	"time"
)

// FakeClock provides deterministic time control for integration tests.
// Default Now() returns a fixed instant; advance explicitly via
// Advance(d). Safe for concurrent access.
type FakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func NewFakeClock() *FakeClock {
	return &FakeClock{now: time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)}
}

func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func (c *FakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}
```

- [ ] **Step 2: Write a FakeRandom**

`tests/integration/api/harness/random.go`:

```go
//go:build integration

package harness

import (
	"crypto/rand"
	"sync"
)

// FakeRandom wraps a deterministic byte source when Seed() was called,
// otherwise falls back to crypto/rand. Tests call Seed(bytes) when they
// want a specific token value.
type FakeRandom struct {
	mu   sync.Mutex
	next [][]byte
}

func NewFakeRandom() *FakeRandom { return &FakeRandom{} }

// Queue an exact byte sequence to return on the next Read() call.
func (r *FakeRandom) Queue(b []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next = append(r.next, append([]byte(nil), b...))
}

func (r *FakeRandom) Read(p []byte) (int, error) {
	r.mu.Lock()
	if len(r.next) > 0 {
		b := r.next[0]
		r.next = r.next[1:]
		r.mu.Unlock()
		n := copy(p, b)
		return n, nil
	}
	r.mu.Unlock()
	return rand.Read(p)
}
```

- [ ] **Step 3: Write app.go that wires a fresh Fiber app**

`tests/integration/api/harness/app.go`:

```go
//go:build integration

package harness

import (
	"context"
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"github.com/kirillinakin/pingcast/internal/adapter/http"
	"github.com/kirillinakin/pingcast/internal/adapter/postgres"
	redisadapter "github.com/kirillinakin/pingcast/internal/adapter/redis"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/bootstrap"
	"github.com/kirillinakin/pingcast/internal/crypto"
	"github.com/kirillinakin/pingcast/internal/port"
)

// App bundles the Fiber app + low-level handles tests occasionally need
// (pool for post-assertion SELECTs).
type App struct {
	Fiber *fiber.App
	Pool  *pgxpool.Pool
	Redis *goredis.Client
	Clock *FakeClock
	Rand  *FakeRandom

	SMTP     *FakeSMTP     // populated in Task 6
	Telegram *FakeTelegram // populated in Task 6
	Webhook  *FakeWebhookSink // populated in Task 6
}

// NewApp builds an in-process Fiber app wired with real Postgres/Redis
// adapters and fake outbound-network adapters. Caller owns teardown via
// App.Close().
func NewApp(t *testing.T) *App {
	t.Helper()

	c := GetContainers()
	if c == nil {
		t.Fatal("harness not initialized")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, c.PostgresURL)
	if err != nil {
		t.Fatalf("pg pool: %v", err)
	}

	rd := goredis.NewClient(&goredis.Options{Addr: redisAddr(c.RedisURL)})
	if err := rd.Ping(ctx).Err(); err != nil {
		t.Fatalf("redis ping: %v", err)
	}

	clock := NewFakeClock()
	rng := NewFakeRandom()

	cipher, err := crypto.NewEncryptor(1, map[byte]string{1: testEncryptionKey()})
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}

	// Repositories
	users := postgres.NewUserRepo(pool)
	sessions := postgres.NewSessionRepo(pool)
	monitors := postgres.NewMonitorRepo(pool)
	incidents := postgres.NewIncidentRepo(pool)
	checks := postgres.NewCheckResultRepo(pool)
	channelsRepo := postgres.NewChannelRepo(pool, cipher)
	apiKeysRepo := postgres.NewAPIKeyRepo(pool)

	rateLimiter := redisadapter.NewRateLimiter(rd, "rl:test", 300, 60)

	// Application services — Clock/Random injection added in Task 11/14;
	// until then these constructors take nothing extra, we still call
	// clock.Now() / rng.Read() by overriding shared package-level vars.
	authSvc := app.NewAuthService(users, sessions, clock, rng)
	monSvc := app.NewMonitoringService(monitors, incidents, checks, clock)
	alertSvc := app.NewAlertService(channelsRepo, monitors, nil, nil, nil) // outbound senders are fakes wired in Task 6

	bootstrap.AttachCipher(channelsRepo, cipher)

	server := httpadapter.NewServer(authSvc, monSvc, alertSvc, rateLimiter, apiKeysRepo)
	pageHandler := httpadapter.NewPageHandler(authSvc, monSvc, alertSvc, rateLimiter, apiKeysRepo)
	webhookHandler := httpadapter.NewWebhookHandler("test-ls-secret", nil)
	healthChecker := httpadapter.NewHealthChecker(pool, rd)

	fiberApp := httpadapter.SetupApp(
		authSvc, pageHandler, server, webhookHandler, apiKeysRepo, healthChecker,
	)

	return &App{
		Fiber: fiberApp,
		Pool:  pool,
		Redis: rd,
		Clock: clock,
		Rand:  rng,
	}
}

func (a *App) Close() {
	if a == nil {
		return
	}
	if a.Pool != nil {
		a.Pool.Close()
	}
	if a.Redis != nil {
		_ = a.Redis.Close()
	}
}

func redisAddr(url string) string {
	// tcredis returns e.g. "redis://localhost:55555"
	const prefix = "redis://"
	if len(url) > len(prefix) && url[:len(prefix)] == prefix {
		return url[len(prefix):]
	}
	return url
}

func testEncryptionKey() string {
	// 32 zero bytes, base64 — deterministic for tests
	return "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
}

// _ keeps port import useful if constructor signatures evolve.
var _ port.Cipher = (*crypto.Encryptor)(nil)
var _ = fmt.Sprintf
```

> **Known gap at this task:** `app.NewAuthService(...)` and `app.NewMonitoringService(...)` don't yet accept `Clock`/`Random` — Tasks 11 and 12 add those parameters. Until then, remove the `clock` / `rng` args from these constructor calls and retain the fake objects purely as state holders attached to `App` for tests to advance time. Add a TODO comment pointing at Tasks 11/12.

- [ ] **Step 4: Extend Harness to own an App**

Rewrite `tests/integration/api/harness/new.go`:

```go
//go:build integration

package harness

import (
	"testing"
)

type Harness struct {
	t   *testing.T
	App *App
}

// New returns a Harness scoped to the current test. The underlying
// containers are shared; state is truncated before each test (Task 5).
func New(t *testing.T) *Harness {
	t.Helper()

	h := &Harness{t: t}
	h.App = NewApp(t)
	t.Cleanup(h.App.Close)

	// State reset happens in Task 5; for now the harness is a no-op.
	return h
}
```

- [ ] **Step 5: Write a smoke test hitting `/health`**

Create `tests/integration/api/health_test.go`:

```go
//go:build integration

package api

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestHealth_Smoke(t *testing.T) {
	h := harness.New(t)

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := h.App.Fiber.Test(req)
	if err != nil {
		t.Fatalf("fiber test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, b)
	}
}
```

- [ ] **Step 6: Run**

Run: `go test -tags=integration ./tests/integration/api/... -run TestHealth_Smoke -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add tests/integration/api/
git commit -m "test(c1): in-process fiber app + smoke /health"
```

---

## Task 5: Add per-test TRUNCATE + FLUSHDB

**Files:**
- Create: `tests/integration/api/harness/truncate.go`
- Modify: `tests/integration/api/harness/new.go`

- [ ] **Step 1: Write truncate.go**

`tests/integration/api/harness/truncate.go`:

```go
//go:build integration

package harness

import (
	"context"
	"fmt"
	"testing"
)

// Tables to wipe between tests. Order does not matter under CASCADE.
var truncateTables = []string{
	"check_results",
	"incidents",
	"monitor_channels",
	"api_keys",
	"sessions",
	"notification_channels",
	"monitors",
	"users",
}

func (a *App) Reset(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	if len(truncateTables) > 0 {
		stmt := fmt.Sprintf(
			"TRUNCATE %s RESTART IDENTITY CASCADE",
			joinTables(truncateTables),
		)
		if _, err := a.Pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("truncate: %v", err)
		}
	}

	if err := a.Redis.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flushdb: %v", err)
	}
}

func joinTables(names []string) string {
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}
```

- [ ] **Step 2: Wire Reset into New()**

Edit `tests/integration/api/harness/new.go` — replace the body:

```go
func New(t *testing.T) *Harness {
	t.Helper()

	h := &Harness{t: t}
	h.App = NewApp(t)
	t.Cleanup(h.App.Close)

	h.App.Reset(t)
	return h
}
```

- [ ] **Step 3: Write a test proving truncate works**

Append to `tests/integration/api/_init_test.go`:

```go
func TestHarness_TruncateBetweenTests_PartA(t *testing.T) {
	h := harness.New(t)
	_, err := h.App.Pool.Exec(t.Context(),
		`INSERT INTO users (id, email, slug, password_hash) VALUES (gen_random_uuid(), 'part-a@test.local', 'part-a', 'x')`)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
}

func TestHarness_TruncateBetweenTests_PartB(t *testing.T) {
	h := harness.New(t)
	var count int
	if err := h.App.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected empty users table, found %d rows (truncate failed)", count)
	}
}
```

- [ ] **Step 4: Run**

Run: `go test -tags=integration ./tests/integration/api/... -run TestHarness_Truncate -v`
Expected: both tests PASS.

- [ ] **Step 5: Commit**

```bash
git add tests/integration/api/
git commit -m "test(c1): per-test truncate + flushdb"
```

---

## Task 6: Wire FakeSMTP, FakeTelegram, FakeWebhookSink

**Files:**
- Create: `tests/integration/api/harness/fakes.go`
- Modify: `tests/integration/api/harness/app.go`

- [ ] **Step 1: Write fakes.go**

`tests/integration/api/harness/fakes.go`:

```go
//go:build integration

package harness

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// ---- SMTP ----------------------------------------------------------

type SentEmail struct {
	To      string
	Subject string
	Body    string
}

type FakeSMTP struct {
	mu   sync.Mutex
	sent []SentEmail
}

func NewFakeSMTP() *FakeSMTP { return &FakeSMTP{} }

func (s *FakeSMTP) Send(to, subject, body string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, SentEmail{To: to, Subject: subject, Body: body})
	return nil
}

func (s *FakeSMTP) AssertSent(t *testing.T, to, subjectContains string) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.sent {
		if e.To == to && strings.Contains(e.Subject, subjectContains) {
			return
		}
	}
	t.Fatalf("expected email to %q with subject containing %q; got: %v", to, subjectContains, s.sent)
}

func (s *FakeSMTP) Sent() []SentEmail {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]SentEmail, len(s.sent))
	copy(out, s.sent)
	return out
}

// ---- Telegram -------------------------------------------------------

type TelegramCall struct {
	ChatID string
	Text   string
}

type FakeTelegram struct {
	server *httptest.Server
	mu     sync.Mutex
	calls  []TelegramCall
}

func NewFakeTelegram() *FakeTelegram {
	f := &FakeTelegram{}
	f.server = httptest.NewServer(http.HandlerFunc(f.handle))
	return f
}

func (f *FakeTelegram) URL() string { return f.server.URL }

func (f *FakeTelegram) Close() { f.server.Close() }

func (f *FakeTelegram) Calls() []TelegramCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]TelegramCall, len(f.calls))
	copy(out, f.calls)
	return out
}

func (f *FakeTelegram) handle(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ChatID string `json:"chat_id"`
		Text   string `json:"text"`
	}
	raw, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(raw, &body)

	f.mu.Lock()
	f.calls = append(f.calls, TelegramCall{ChatID: body.ChatID, Text: body.Text})
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

// ---- Webhook sink --------------------------------------------------

type WebhookHit struct {
	Method  string
	Path    string
	Headers http.Header
	Body    []byte
}

type FakeWebhookSink struct {
	server *httptest.Server
	mu     sync.Mutex
	hits   []WebhookHit
}

func NewFakeWebhookSink() *FakeWebhookSink {
	s := &FakeWebhookSink{}
	s.server = httptest.NewServer(http.HandlerFunc(s.handle))
	return s
}

func (s *FakeWebhookSink) URL() string { return s.server.URL }

func (s *FakeWebhookSink) Close() { s.server.Close() }

func (s *FakeWebhookSink) Hits() []WebhookHit {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]WebhookHit, len(s.hits))
	copy(out, s.hits)
	return out
}

func (s *FakeWebhookSink) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	s.hits = append(s.hits, WebhookHit{
		Method:  r.Method,
		Path:    r.URL.Path,
		Headers: r.Header.Clone(),
		Body:    body,
	})
	s.mu.Unlock()
	w.WriteHeader(200)
}
```

- [ ] **Step 2: Register fakes on the App**

Edit `tests/integration/api/harness/app.go`, inside `NewApp(...)` after `rng := NewFakeRandom()` and before building adapters:

```go
smtp := NewFakeSMTP()
telegram := NewFakeTelegram()
t.Cleanup(telegram.Close)
webhookSink := NewFakeWebhookSink()
t.Cleanup(webhookSink.Close)
```

…and at the return statement:

```go
return &App{
	Fiber:    fiberApp,
	Pool:     pool,
	Redis:    rd,
	Clock:    clock,
	Rand:     rng,
	SMTP:     smtp,
	Telegram: telegram,
	Webhook:  webhookSink,
}
```

The fakes are attached but do not yet replace production senders — that's part of C2. We only need them available for tests that inspect "would it have been called?" via the app-service layer.

- [ ] **Step 3: Smoke-test the fakes exist**

Append to `tests/integration/api/_init_test.go`:

```go
func TestHarness_FakesAttached(t *testing.T) {
	h := harness.New(t)
	if h.App.SMTP == nil || h.App.Telegram == nil || h.App.Webhook == nil {
		t.Fatal("fakes not attached")
	}
	if h.App.Telegram.URL() == "" {
		t.Error("telegram url empty")
	}
}
```

- [ ] **Step 4: Run**

Run: `go test -tags=integration ./tests/integration/api/... -run TestHarness_FakesAttached -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tests/integration/api/
git commit -m "test(c1): FakeSMTP, FakeTelegram, FakeWebhookSink scaffolds"
```

---

## Task 7: Session (cookie jar) + HTTP helpers + assertion helpers

**Files:**
- Create: `tests/integration/api/harness/session.go`
- Create: `tests/integration/api/harness/assert.go`
- Create: `tests/integration/api/harness/fixtures.go`

- [ ] **Step 1: Write session.go**

`tests/integration/api/harness/session.go`:

```go
//go:build integration

package harness

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// Session represents an authenticated client bound to a single user.
type Session struct {
	app     *App
	cookies []*http.Cookie
}

// NewSession returns an unauthenticated session (no cookies).
func (a *App) NewSession() *Session {
	return &Session{app: a}
}

// Do issues a request through fiber.App.Test(), attaching collected
// cookies, and harvests Set-Cookie from the response.
func (s *Session) Do(t *testing.T, method, path string, body any) *Response {
	t.Helper()

	var buf io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		buf = bytes.NewReader(raw)
	}

	req := httptest.NewRequest(method, path, buf)
	if buf != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, c := range s.cookies {
		req.AddCookie(c)
	}

	resp, err := s.app.Fiber.Test(req, -1)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}

	for _, c := range resp.Cookies() {
		s.upsertCookie(c)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	_ = resp.Body.Close()

	return &Response{
		Status:  resp.StatusCode,
		Headers: resp.Header,
		Body:    raw,
	}
}

func (s *Session) GET(t *testing.T, path string) *Response {
	return s.Do(t, fiber.MethodGet, path, nil)
}

func (s *Session) POST(t *testing.T, path string, body any) *Response {
	return s.Do(t, fiber.MethodPost, path, body)
}

func (s *Session) PUT(t *testing.T, path string, body any) *Response {
	return s.Do(t, fiber.MethodPut, path, body)
}

func (s *Session) DELETE(t *testing.T, path string) *Response {
	return s.Do(t, fiber.MethodDelete, path, nil)
}

// WithBearer returns a NEW session that additionally sends the given
// Bearer token in Authorization header. Does not share cookies.
func (a *App) WithBearer(token string) *Session {
	s := a.NewSession()
	s.cookies = nil
	s.bearer = token
	return s
}

func (s *Session) upsertCookie(c *http.Cookie) {
	for i, existing := range s.cookies {
		if existing.Name == c.Name {
			s.cookies[i] = c
			return
		}
	}
	s.cookies = append(s.cookies, c)
}

// --- extra field for bearer support ---------------------------------

// (placed after upsertCookie to keep the struct flat)
type bearerHolder struct{ bearer string } // unused — kept to avoid breaking if removed prematurely

// Response is a small value type that tests inspect directly.
type Response struct {
	Status  int
	Headers http.Header
	Body    []byte
}

func (r *Response) JSON(t *testing.T, out any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, out); err != nil {
		t.Fatalf("unmarshal body=%s: %v", r.Body, err)
	}
}
```

> Note: the bearer field is used by an overload path added next. Fold the field into Session directly if preferred; here it is illustrative.

Replace the bogus `bearerHolder` section with a proper field on `Session`:

```go
type Session struct {
	app     *App
	cookies []*http.Cookie
	bearer  string
}
```

And in `Do(...)` after cookies loop, add:

```go
if s.bearer != "" {
	req.Header.Set("Authorization", "Bearer "+s.bearer)
}
```

- [ ] **Step 2: Write assert.go**

`tests/integration/api/harness/assert.go`:

```go
//go:build integration

package harness

import (
	"strings"
	"testing"
)

// ErrorBody is the canonical error envelope defined in spec §1.
type ErrorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// AssertStatus fails if r.Status != want.
func AssertStatus(t *testing.T, r *Response, want int) {
	t.Helper()
	if r.Status != want {
		t.Fatalf("status: want=%d got=%d body=%s", want, r.Status, r.Body)
	}
}

// AssertError asserts a canonical-envelope error response with the
// given HTTP status and a code matching wantCode. Pass wantCode="" to
// skip the code check.
func AssertError(t *testing.T, r *Response, wantStatus int, wantCode string) *ErrorBody {
	t.Helper()
	AssertStatus(t, r, wantStatus)

	var body ErrorBody
	r.JSON(t, &body)

	if body.Error.Code == "" {
		t.Fatalf("envelope missing error.code: %s", r.Body)
	}
	if body.Error.Message == "" {
		t.Fatalf("envelope missing error.message: %s", r.Body)
	}
	if wantCode != "" && body.Error.Code != wantCode {
		t.Fatalf("error.code: want=%q got=%q (body=%s)", wantCode, body.Error.Code, r.Body)
	}
	return &body
}

// AssertEnvelopeMessageContains is a tolerant fallback when the exact
// code is not worth spec-locking but a human-readable substring is.
func AssertEnvelopeMessageContains(t *testing.T, r *Response, substr string) {
	t.Helper()
	var body ErrorBody
	r.JSON(t, &body)
	if !strings.Contains(body.Error.Message, substr) {
		t.Fatalf("error.message: want substring %q, got %q", substr, body.Error.Message)
	}
}
```

- [ ] **Step 3: Write fixtures.go**

`tests/integration/api/harness/fixtures.go`:

```go
//go:build integration

package harness

import (
	"fmt"
	"testing"
)

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RegisterAndLogin registers a user via POST /api/auth/register and
// returns a Session with the session cookie set. Email and password
// defaults are derived from test name when empty.
func (h *Harness) RegisterAndLogin(t *testing.T, email, password string) *Session {
	t.Helper()

	if email == "" {
		email = fmt.Sprintf("u-%d@test.local", uniqueCounter.Next())
	}
	if password == "" {
		password = "password123"
	}

	s := h.App.NewSession()
	resp := s.POST(t, "/api/auth/register", registerRequest{Email: email, Password: password})
	if resp.Status != 201 {
		t.Fatalf("register: want 201, got %d body=%s", resp.Status, resp.Body)
	}
	return s
}

// TwoSessions returns two logged-in sessions bound to different users.
func (h *Harness) TwoSessions(t *testing.T) (*Session, *Session) {
	t.Helper()
	return h.RegisterAndLogin(t, "", ""), h.RegisterAndLogin(t, "", "")
}

// --- counter --------------------------------------------------------

var uniqueCounter counter

type counter struct{ n int }

func (c *counter) Next() int { c.n++; return c.n }
```

- [ ] **Step 4: Add a smoke test for RegisterAndLogin**

Create `tests/integration/api/session_smoke_test.go`:

```go
//go:build integration

package api

import (
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestSession_RegisterAndLogin_SetsCookie(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "cookie@test.local", "password123")

	resp := s.GET(t, "/api/monitors")
	if resp.Status == 401 {
		t.Fatalf("session did not authenticate; body=%s", resp.Body)
	}
}
```

- [ ] **Step 5: Run**

Run: `go test -tags=integration ./tests/integration/api/... -run TestSession_RegisterAndLogin -v`
Expected: PASS (may reveal missing response fields — iterate until green; this is the first end-to-end through the real auth service).

- [ ] **Step 6: Commit**

```bash
git add tests/integration/api/
git commit -m "test(c1): session cookie-jar, assertion helpers, user fixtures"
```

---

# Phase 1 — Clock / Random ports

## Task 8: Define port/clock.go and port/random.go

**Files:**
- Create: `internal/port/clock.go`
- Create: `internal/port/random.go`

- [ ] **Step 1: Write clock.go**

`internal/port/clock.go`:

```go
package port

import "time"

// Clock is the source of wall-clock time used by application services.
// Production impl wraps time.Now; test impl is deterministic.
type Clock interface {
	Now() time.Time
}
```

- [ ] **Step 2: Write random.go**

`internal/port/random.go`:

```go
package port

// Random is the cryptographic RNG used for tokens and session IDs.
// Production impl wraps crypto/rand; test impl can be seeded.
type Random interface {
	Read(p []byte) (int, error)
}
```

- [ ] **Step 3: Verify compile**

Run: `go build ./internal/port/...`
Expected: exits 0.

- [ ] **Step 4: Commit**

```bash
git add internal/port/clock.go internal/port/random.go
git commit -m "feat(c1): Clock and Random ports"
```

---

## Task 9: Implement sysclock + sysrand adapters

**Files:**
- Create: `internal/adapter/sysclock/clock.go`
- Create: `internal/adapter/sysrand/random.go`

- [ ] **Step 1: Write clock.go**

`internal/adapter/sysclock/clock.go`:

```go
package sysclock

import "time"

// Clock is the stdlib-backed production impl of port.Clock.
type Clock struct{}

func New() Clock { return Clock{} }

func (Clock) Now() time.Time { return time.Now() }
```

- [ ] **Step 2: Write random.go**

`internal/adapter/sysrand/random.go`:

```go
package sysrand

import "crypto/rand"

// Random is the crypto/rand-backed production impl of port.Random.
type Random struct{}

func New() Random { return Random{} }

func (Random) Read(p []byte) (int, error) { return rand.Read(p) }
```

- [ ] **Step 3: Verify interface satisfaction with a tiny compile-time test**

Append to `internal/adapter/sysclock/clock.go`:

```go
var _ = time.Time{} // keep import used
```

If lint complains, the line is unnecessary — `time.Now()` uses `time`. Remove and run lint again.

- [ ] **Step 4: Compile & commit**

Run: `go build ./internal/adapter/sysclock/... ./internal/adapter/sysrand/...`
Expected: exits 0.

```bash
git add internal/adapter/sysclock internal/adapter/sysrand
git commit -m "feat(c1): stdlib Clock and Random adapters"
```

---

## Task 10: Thread Clock through AuthService

**Files:**
- Modify: `internal/app/auth.go`
- Modify: `internal/app/auth_test.go`
- Modify: `cmd/api/main.go` (add `sysclock.New()` to construction)
- Modify: `tests/integration/api/harness/app.go` (pass `clock` to `app.NewAuthService`)

- [ ] **Step 1: Read auth.go**

Run: `head -80 internal/app/auth.go`

Locate struct `AuthService` and its constructor `NewAuthService`. Record the current parameter list.

- [ ] **Step 2: Add Clock field and parameter**

Edit `internal/app/auth.go`:

- Add `"github.com/kirillinakin/pingcast/internal/port"` to imports if missing.
- Add `clock port.Clock` field to `AuthService`.
- Add `clock port.Clock` as last parameter of `NewAuthService`.
- Replace every `time.Now()` in this file with `s.clock.Now()`.

Example patch in `NewAuthService`:

```go
func NewAuthService(users port.UserRepo, sessions port.SessionRepo, clock port.Clock) *AuthService {
	return &AuthService{users: users, sessions: sessions, clock: clock}
}
```

- [ ] **Step 3: Update call sites in auth_test.go to pass a fake clock**

Pattern to apply:

```go
svc := app.NewAuthService(users, sessions, stubClock{})
```

Add at the bottom of `auth_test.go`:

```go
type stubClock struct{ now time.Time }

func (c stubClock) Now() time.Time {
	if c.now.IsZero() {
		return time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	}
	return c.now
}
```

- [ ] **Step 4: Update cmd/api/main.go composition**

Find the line constructing `AuthService` (grep `NewAuthService(`), change to:

```go
authSvc := app.NewAuthService(userRepo, sessionRepo, sysclock.New())
```

Add `"github.com/kirillinakin/pingcast/internal/adapter/sysclock"` import.

- [ ] **Step 5: Update integration harness**

In `tests/integration/api/harness/app.go`, update:

```go
authSvc := app.NewAuthService(users, sessions, clock)
```

(`clock` is the `*FakeClock`; its `Now()` already satisfies `port.Clock`.)

- [ ] **Step 6: Run unit tests**

Run: `go test ./internal/app/...`
Expected: PASS.

Run: `go test -tags=integration ./tests/integration/api/... -run TestHealth_Smoke -v`
Expected: PASS (regression canary).

- [ ] **Step 7: Commit**

```bash
git add internal/app/auth.go internal/app/auth_test.go cmd/api/main.go tests/integration/api/harness/app.go
git commit -m "refactor(c1): thread Clock port through AuthService"
```

---

## Task 11: Thread Random through AuthService (session token generation)

**Files:**
- Modify: `internal/app/auth.go`
- Modify: `internal/app/auth_test.go`
- Modify: `cmd/api/main.go`
- Modify: `tests/integration/api/harness/app.go`

- [ ] **Step 1: Add rng field and param**

Edit `internal/app/auth.go`:

- Add `rng port.Random` field to `AuthService`.
- Add `rng port.Random` as last parameter to `NewAuthService`:

```go
func NewAuthService(users port.UserRepo, sessions port.SessionRepo, clock port.Clock, rng port.Random) *AuthService {
	return &AuthService{users: users, sessions: sessions, clock: clock, rng: rng}
}
```

- Find any place generating session tokens with `crypto/rand.Read(buf)` and replace with `s.rng.Read(buf)`. If there is none (tokens might be UUID-based), grep and only replace the specific calls; leave UUIDs alone.

Run: `grep -n "rand\.Read\|rand\.Int" internal/app/auth.go`

Replace each with the field call.

- [ ] **Step 2: Update call sites in auth_test.go**

Add to the `stubClock` section:

```go
type stubRand struct{}

func (stubRand) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(i)
	}
	return len(p), nil
}
```

And update constructor calls: `app.NewAuthService(users, sessions, stubClock{}, stubRand{})`.

- [ ] **Step 3: Update cmd/api/main.go**

```go
authSvc := app.NewAuthService(userRepo, sessionRepo, sysclock.New(), sysrand.New())
```

Add `"github.com/kirillinakin/pingcast/internal/adapter/sysrand"` import.

- [ ] **Step 4: Update harness**

```go
authSvc := app.NewAuthService(users, sessions, clock, rng)
```

- [ ] **Step 5: Run unit + integration smoke**

```
go test ./internal/app/...
go test -tags=integration ./tests/integration/api/... -run TestSession_RegisterAndLogin -v
```

Both PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/auth.go internal/app/auth_test.go cmd/api/main.go tests/integration/api/harness/app.go
git commit -m "refactor(c1): thread Random port through AuthService"
```

---

## Task 12: Thread Clock through MonitoringService

**Files:**
- Modify: `internal/app/monitoring.go`
- Modify: potentially `internal/app/alert.go` if it uses `time.Now()`
- Modify: `cmd/api/main.go`
- Modify: `tests/integration/api/harness/app.go`

- [ ] **Step 1: Audit time.Now() usages**

Run: `grep -n "time\.Now" internal/app/monitoring.go internal/app/alert.go`

Record each line number.

- [ ] **Step 2: Add clock field + param to MonitoringService**

`internal/app/monitoring.go`:

- Add `clock port.Clock` to struct.
- Update `NewMonitoringService(...)` signature: append `clock port.Clock`, store it.
- Replace every `time.Now()` in the file with `s.clock.Now()`.

- [ ] **Step 3: Same for AlertService if applicable**

If `grep` found usages in `alert.go`, apply the same pattern:

```go
func NewAlertService(
	channels port.ChannelRepo,
	monitors port.MonitorRepo,
	// ...existing deps...
	clock port.Clock,
) *AlertService
```

- [ ] **Step 4: Update cmd/api/main.go + harness + tests**

For each service whose signature changed, update constructors at:
- `cmd/api/main.go`
- `tests/integration/api/harness/app.go`
- `internal/app/*_test.go`

Example for MonitoringService in harness:

```go
monSvc := app.NewMonitoringService(monitors, incidents, checks, clock)
```

- [ ] **Step 5: Run full unit suite**

```
go test ./internal/...
```
Expected: PASS.

Run integration smoke:
```
go test -tags=integration ./tests/integration/api/... -run TestSession_RegisterAndLogin -v
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor(c1): thread Clock through MonitoringService and AlertService"
```

---

# Phase 2 — Canonical error envelope (spec §1)

## Task 13: Write failing tests for canonical envelope

**Files:**
- Create: `tests/integration/api/errorenv_test.go`

- [ ] **Step 1: Write the failing tests**

`tests/integration/api/errorenv_test.go`:

```go
//go:build integration

package api

import (
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

// Every non-2xx response must use the canonical envelope
// {"error":{"code":"...","message":"..."}}.

func TestErrorEnvelope_UnauthorizedHasCanonicalShape(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	resp := s.GET(t, "/api/monitors")
	harness.AssertError(t, resp, 401, "UNAUTHORIZED")
}

func TestErrorEnvelope_MalformedJSONIs400(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()

	req := buildRaw(t, "POST", "/api/auth/register", []byte("{not valid json"))
	resp := do(t, h.App, s, req)

	harness.AssertError(t, resp, 400, "MALFORMED_JSON")
}

func TestErrorEnvelope_BusinessValidationIs422(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	// empty password triggers validation error
	resp := s.POST(t, "/api/auth/register", map[string]any{
		"email":    "ok@test.local",
		"password": "",
	})
	harness.AssertError(t, resp, 422, "VALIDATION_FAILED")
}

func TestErrorEnvelope_NotFoundIs404WithEnvelope(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.GET(t, "/api/monitors/00000000-0000-0000-0000-000000000000")
	harness.AssertError(t, resp, 404, "NOT_FOUND")
}
```

Add helpers `buildRaw` and `do` at the bottom (shared with other tests):

```go
import (
	"bytes"
	"net/http/httptest"
)

func buildRaw(t *testing.T, method, path string, body []byte) *httptest.Request {
	t.Helper()
	// placeholder — actual impl in harness if multiple tests need raw bodies
	return nil
}

func do(t *testing.T, app *harness.App, s *harness.Session, req any) *harness.Response {
	t.Helper()
	// placeholder — replace with a harness method in step 2
	return nil
}
```

> **Step 2** rewrites these helpers as a proper harness method. Placeholders here only exist so the test file compiles until step 2 lands.

- [ ] **Step 2: Add raw-body HTTP helper to harness/session.go**

Append to `tests/integration/api/harness/session.go`:

```go
// PostRaw issues a POST with an exact byte body (useful for malformed
// JSON cases). Shares cookies and bearer like Do().
func (s *Session) PostRaw(t *testing.T, path, contentType string, body []byte) *Response {
	t.Helper()

	req := httptest.NewRequest(fiber.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	for _, c := range s.cookies {
		req.AddCookie(c)
	}
	if s.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+s.bearer)
	}
	resp, err := s.app.Fiber.Test(req, -1)
	if err != nil {
		t.Fatalf("POST raw %s: %v", path, err)
	}
	raw, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return &Response{Status: resp.StatusCode, Headers: resp.Header, Body: raw}
}
```

- [ ] **Step 3: Rewrite errorenv_test.go to drop placeholders**

Update `TestErrorEnvelope_MalformedJSONIs400`:

```go
func TestErrorEnvelope_MalformedJSONIs400(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	resp := s.PostRaw(t, "/api/auth/register", "application/json", []byte("{not valid json"))
	harness.AssertError(t, resp, 400, "MALFORMED_JSON")
}
```

Delete the `buildRaw`/`do` placeholder helpers (they are no longer referenced).

- [ ] **Step 4: Run — expect FAIL**

Run: `go test -tags=integration ./tests/integration/api/... -run TestErrorEnvelope -v`
Expected: failures — current handlers emit either plain string error bodies or non-canonical codes.

Record the actual responses for each test in a scratch note; they guide the Task 14 fix.

- [ ] **Step 5: Commit failing tests (red)**

```bash
git add tests/integration/api/
git commit -m "test(c1): red — canonical error envelope tests"
```

---

## Task 14: Implement canonical envelope emitter and retrofit handlers

**Files:**
- Modify: `internal/adapter/httperr/response.go`
- Modify: `internal/adapter/http/server.go` (all handlers with ad-hoc `ErrorResponse{Error: ...}`)
- Modify: `internal/adapter/http/setup.go` (ErrorHandler) if needed
- Modify: `internal/domain/errors.go` if new error codes need names

- [ ] **Step 1: Define canonical emitter in httperr**

Edit `internal/adapter/httperr/response.go`:

Replace the file with:

```go
package httperr

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/kirillinakin/pingcast/internal/domain"
)

// Envelope is the public error response contract (spec §1).
type Envelope struct {
	Error Inner `json:"error"`
}

type Inner struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Write emits a canonical error envelope based on err. Unknown error
// types collapse to 500 / INTERNAL_ERROR with a safe message.
func Write(c *fiber.Ctx, err error) error {
	status, code, msg := classify(err)
	if status >= 500 {
		slog.Error("internal error", "path", c.Path(), "error", err)
	}
	return c.Status(status).JSON(Envelope{Error: Inner{Code: code, Message: msg}})
}

// WriteValidation is a shortcut for 422 VALIDATION_FAILED with a
// caller-provided message.
func WriteValidation(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusUnprocessableEntity).JSON(Envelope{Error: Inner{Code: "VALIDATION_FAILED", Message: message}})
}

// WriteMalformedJSON is a shortcut for 400 MALFORMED_JSON.
func WriteMalformedJSON(c *fiber.Ctx) error {
	return c.Status(fiber.StatusBadRequest).JSON(Envelope{Error: Inner{Code: "MALFORMED_JSON", Message: "request body is not valid JSON"}})
}

// WriteUnauthorized is a shortcut for 401 UNAUTHORIZED.
func WriteUnauthorized(c *fiber.Ctx) error {
	return c.Status(fiber.StatusUnauthorized).JSON(Envelope{Error: Inner{Code: "UNAUTHORIZED", Message: "authentication required"}})
}

// WriteForbiddenTenant is a shortcut for 403 FORBIDDEN_TENANT.
func WriteForbiddenTenant(c *fiber.Ctx) error {
	return c.Status(fiber.StatusForbidden).JSON(Envelope{Error: Inner{Code: "FORBIDDEN_TENANT", Message: "access denied"}})
}

// WriteNotFound is a shortcut for 404 NOT_FOUND.
func WriteNotFound(c *fiber.Ctx, what string) error {
	msg := "resource not found"
	if what != "" {
		msg = what + " not found"
	}
	return c.Status(fiber.StatusNotFound).JSON(Envelope{Error: Inner{Code: "NOT_FOUND", Message: msg}})
}

// WriteConflict is a shortcut for 409 CONFLICT.
func WriteConflict(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusConflict).JSON(Envelope{Error: Inner{Code: "CONFLICT", Message: message}})
}

// WriteRateLimited is a shortcut for 429 RATE_LIMITED with Retry-After.
func WriteRateLimited(c *fiber.Ctx, retryAfterSeconds int) error {
	c.Set("Retry-After", itoa(retryAfterSeconds))
	return c.Status(fiber.StatusTooManyRequests).JSON(Envelope{Error: Inner{Code: "RATE_LIMITED", Message: "too many requests"}})
}

// classify maps an error into (status, code, safe message).
func classify(err error) (int, string, string) {
	var domErr *domain.DomainError
	if errors.As(err, &domErr) {
		switch {
		case errors.Is(domErr, domain.ErrNotFound):
			return 404, "NOT_FOUND", domErr.Message
		case errors.Is(domErr, domain.ErrForbidden):
			return 403, "FORBIDDEN_TENANT", "access denied"
		case errors.Is(domErr, domain.ErrValidation):
			return 422, "VALIDATION_FAILED", domErr.Message
		case errors.Is(domErr, domain.ErrConflict):
			return 409, "CONFLICT", domErr.Message
		case errors.Is(domErr, domain.ErrUnauthorized):
			return 401, "UNAUTHORIZED", "authentication required"
		}
	}
	var fe *fiber.Error
	if errors.As(err, &fe) {
		if fe.Code == 401 {
			return 401, "UNAUTHORIZED", "authentication required"
		}
		if fe.Code == 404 {
			return 404, "NOT_FOUND", "resource not found"
		}
	}
	return 500, "INTERNAL_ERROR", "internal error"
}

// ClassifyHTTPError is the pre-existing helper some handlers call.
// Kept as a thin wrapper so we don't have to change every call site
// in a single commit — new calls should prefer Write(c, err).
func ClassifyHTTPError(err error) (int, string) {
	status, code, msg := classify(err)
	_ = code
	return status, msg
}

func itoa(n int) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = digits[n%10]
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
```

- [ ] **Step 2: Update domain/errors.go to ensure ErrUnauthorized exists**

Run: `grep -n "ErrUnauthorized\|ErrNotFound\|ErrValidation\|ErrConflict" internal/domain/errors.go`

If `ErrUnauthorized` is missing, add to the sentinels list:

```go
ErrUnauthorized = &DomainError{Code: "unauthorized", Message: "authentication required"}
```

- [ ] **Step 3: Replace `ErrorResponse{Error: ...}` with envelope in server.go**

For each handler in `internal/adapter/http/server.go` that returns `apigen.ErrorResponse{Error: ...}` directly, replace with the appropriate `httperr.Write*` call.

Example diff pattern:

```go
// Before:
return c.Status(400).JSON(apigen.ErrorResponse{Error: new("invalid request body")})
// After:
return httperr.WriteMalformedJSON(c)
```

```go
// Before:
return c.Status(500).JSON(apigen.ErrorResponse{Error: new("failed to list channels")})
// After:
return httperr.Write(c, err)
```

Do this for every handler — grep `grep -n "apigen\.ErrorResponse" internal/adapter/http/server.go` and walk line by line.

- [ ] **Step 4: Update setup.go ErrorHandler to use canonical envelope**

Edit `internal/adapter/http/setup.go`, replace the `ErrorHandler` body with:

```go
ErrorHandler: func(c *fiber.Ctx, err error) error {
	return httperr.Write(c, err)
},
```

Add import `"github.com/kirillinakin/pingcast/internal/adapter/httperr"`.

- [ ] **Step 5: Run — expect GREEN on the envelope tests**

Run: `go test -tags=integration ./tests/integration/api/... -run TestErrorEnvelope -v`
Expected: all 4 tests PASS.

Run: `go test ./internal/...`
Expected: PASS (no unit regressions).

Run: `golangci-lint run`
Expected: 0 findings.

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/httperr internal/adapter/http internal/domain
git commit -m "feat(c1): canonical error envelope; retrofit handlers"
```

---

# Phase 3 — Other spec-driven code fixes

## Task 15: Add GET /api/channels/{id}

**Files:**
- Modify: `api/openapi.yaml`
- Modify: `internal/app/alert.go` (new `GetChannelByID(ctx, userID, channelID)` method)
- Modify: `internal/adapter/http/server.go` (new `GetChannel` handler)
- Regenerate: `internal/api/gen/*.go` via `oapi-codegen`
- Regenerate: `frontend/lib/openapi-types.ts` via `pnpm gen:types`

- [ ] **Step 1: Write the failing tests**

Create `tests/integration/api/channels_get_test.go`:

```go
//go:build integration

package api

import (
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestGetChannel_ReturnsRedactedChannel(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")

	createResp := s.POST(t, "/api/channels", map[string]any{
		"name":   "ops",
		"type":   "telegram",
		"config": map[string]any{"bot_token": "12345:ABCDEF", "chat_id": "99"},
	})
	harness.AssertStatus(t, createResp, 201)

	var created struct{ ID string }
	createResp.JSON(t, &created)

	getResp := s.GET(t, "/api/channels/"+created.ID)
	harness.AssertStatus(t, getResp, 200)

	var got struct {
		ID     string
		Name   string
		Type   string
		Config map[string]any
	}
	getResp.JSON(t, &got)

	if got.ID != created.ID {
		t.Errorf("id mismatch: %s vs %s", got.ID, created.ID)
	}
	if got.Name != "ops" {
		t.Errorf("name: got %q", got.Name)
	}
	if tok, _ := got.Config["bot_token"].(string); tok == "12345:ABCDEF" {
		t.Errorf("bot_token leaked unredacted: %q", tok)
	}
}

func TestGetChannel_NotFound(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.GET(t, "/api/channels/00000000-0000-0000-0000-000000000000")
	harness.AssertError(t, resp, 404, "NOT_FOUND")
}

func TestGetChannel_CrossTenantForbidden(t *testing.T) {
	h := harness.New(t)
	sA, sB := h.TwoSessions(t)

	createResp := sA.POST(t, "/api/channels", map[string]any{
		"name":   "a",
		"type":   "webhook",
		"config": map[string]any{"url": "https://example.com/hook"},
	})
	harness.AssertStatus(t, createResp, 201)
	var created struct{ ID string }
	createResp.JSON(t, &created)

	resp := sB.GET(t, "/api/channels/"+created.ID)
	harness.AssertError(t, resp, 403, "FORBIDDEN_TENANT")
}

func TestGetChannel_Unauthenticated(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	resp := s.GET(t, "/api/channels/00000000-0000-0000-0000-000000000000")
	harness.AssertError(t, resp, 401, "UNAUTHORIZED")
}
```

- [ ] **Step 2: Run — expect FAIL**

Run: `go test -tags=integration ./tests/integration/api/... -run TestGetChannel -v`
Expected: all 4 tests fail (404 from route not existing, or redaction missing).

Note which fail for which reason — each will be green after one of the following steps.

- [ ] **Step 3: Add GET to OpenAPI**

Edit `api/openapi.yaml`, inside the `/api/channels/{id}:` path object, add a `get` operation **before** the existing `put`:

```yaml
    get:
      operationId: getChannel
      tags: [channels]
      security:
        - sessionAuth: []
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: Channel
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/NotificationChannel"
        "404":
          description: Not found
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/ErrorResponse"
```

- [ ] **Step 4: Regenerate the Go server interface**

Run: `go generate ./internal/api/gen/...` (or the project's codegen command — check `cmd/codegen/` or `Makefile`).

If the project uses `oapi-codegen` directly, the command is likely:

```
oapi-codegen -config api/oapi-codegen.yaml api/openapi.yaml > internal/api/gen/server.gen.go
```

Verify a `GetChannel` method appeared on the `ServerInterface`:

Run: `grep -n "GetChannel" internal/api/gen/*.go | head -5`

- [ ] **Step 5: Implement the handler on Server**

Edit `internal/adapter/http/server.go`, add:

```go
func (s *Server) GetChannel(c *fiber.Ctx, id openapi_types.UUID) error {
	user := requireUser(c)
	if user == nil {
		return nil
	}

	ch, err := s.alerts.GetChannelByID(c.UserContext(), user.ID, uuid.UUID(id))
	if err != nil {
		return httperr.Write(c, err)
	}
	return c.JSON(domainChannelToAPI(ch))
}
```

- [ ] **Step 6: Add alertService.GetChannelByID**

Edit `internal/app/alert.go`:

```go
func (s *AlertService) GetChannelByID(ctx context.Context, ownerID, channelID uuid.UUID) (*domain.Channel, error) {
	ch, err := s.channels.GetByID(ctx, channelID)
	if err != nil {
		return nil, err
	}
	if ch == nil {
		return nil, domain.ErrNotFound
	}
	if ch.UserID != ownerID {
		return nil, domain.ErrForbidden
	}
	return ch, nil
}
```

If `channels.GetByID` does not exist, add it to `port.ChannelRepo` and the postgres adapter (`internal/adapter/postgres/channels.go`). Use the pattern from existing `GetMonitorByID` in `MonitorRepo`.

- [ ] **Step 7: Ensure redaction of secrets**

If `domainChannelToAPI` does not redact, add a redact step:

```go
func domainChannelToAPI(ch *domain.Channel) apigen.NotificationChannel {
	cfg := redactChannelConfig(ch.Type, ch.Config)
	return apigen.NotificationChannel{
		Id:        ch.ID,
		Name:      ch.Name,
		Type:      apigen.NotificationChannelType(ch.Type),
		Config:    cfg,
		IsEnabled: ch.IsEnabled,
	}
}

// redactChannelConfig replaces sensitive fields with "***"+last-4 per spec §8.9.
func redactChannelConfig(t domain.ChannelType, cfg json.RawMessage) map[string]any {
	var m map[string]any
	if err := json.Unmarshal(cfg, &m); err != nil {
		return map[string]any{}
	}
	switch t {
	case domain.ChannelTypeTelegram:
		if s, ok := m["bot_token"].(string); ok {
			m["bot_token"] = redactSecret(s)
		}
	case domain.ChannelTypeWebhook:
		if s, ok := m["url"].(string); ok {
			m["url"] = redactSecret(s)
		}
	}
	return m
}

func redactSecret(s string) string {
	if len(s) <= 4 {
		return "***"
	}
	return "***" + s[len(s)-4:]
}
```

- [ ] **Step 8: Regenerate TS types**

Run:
```
cd frontend && pnpm gen:types && cd ..
```

- [ ] **Step 9: Run tests — expect GREEN**

Run: `go test -tags=integration ./tests/integration/api/... -run TestGetChannel -v`
Expected: all 4 tests PASS.

Run full unit + lint:
```
go test ./internal/... && golangci-lint run
```

- [ ] **Step 10: Commit**

```bash
git add api/openapi.yaml internal/ frontend/lib/openapi-types.ts tests/integration/api/
git commit -m "feat(c1): GET /api/channels/{id} with config redaction"
```

---

## Task 16: Remove dead POST /logout page handler

**Files:**
- Modify: `internal/adapter/http/setup.go`
- Modify: `internal/adapter/http/pages.go` (remove `Logout` method)

- [ ] **Step 1: Write failing test**

Append to `tests/integration/api/authmatrix_test.go` (create if missing):

```go
//go:build integration

package api

import (
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestPageLogout_RouteRemoved(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.POST(t, "/logout", nil)
	harness.AssertStatus(t, resp, 404)
}
```

- [ ] **Step 2: Run — FAIL (route currently exists, returns 303 or similar)**

Run: `go test -tags=integration ./tests/integration/api/... -run TestPageLogout -v`

- [ ] **Step 3: Remove route from setup.go**

Edit `internal/adapter/http/setup.go`:

- Delete the line `app.Post("/logout", pageHandler.Logout)`.
- Remove the explanatory comments around it that referred to the dead path.

- [ ] **Step 4: Remove Logout method from pages.go**

Edit `internal/adapter/http/pages.go`:

- Delete the `Logout` method on `PageHandler`.
- Remove any imports now unused.

- [ ] **Step 5: Run — expect GREEN**

Run: `go test -tags=integration ./tests/integration/api/... -run TestPageLogout -v`
Expected: PASS (404 with envelope).

Run: `go build ./...` and `golangci-lint run` — both clean.

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/http/setup.go internal/adapter/http/pages.go tests/integration/api/
git commit -m "refactor(c1): remove dead POST /logout page handler"
```

---

# Phase 4 — Endpoint test suites

Each task below writes the full test suite for one endpoint group, runs it against the current (possibly broken) code, and iterates code fixes until green. Tests assert the spec, not the code.

Each test suite shares the common pattern:
1. Build `h := harness.New(t)`.
2. Use fixtures (`RegisterAndLogin`, `TwoSessions`) for setup.
3. Issue HTTP calls via `s.GET/POST/PUT/DELETE`.
4. Use `harness.AssertError(...)` or `r.JSON(...)` for assertions.

## Task 17: Auth endpoint tests

**File:** `tests/integration/api/auth_test.go`

- [ ] **Step 1: Write all auth tests**

`tests/integration/api/auth_test.go`:

```go
//go:build integration

package api

import (
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestRegister_Happy_Returns201WithUser(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	resp := s.POST(t, "/api/auth/register", map[string]any{
		"email":    "happy@test.local",
		"password": "password123",
	})
	harness.AssertStatus(t, resp, 201)

	var body struct {
		User struct {
			ID    string
			Email string
			Slug  string
			Plan  string
		}
	}
	resp.JSON(t, &body)
	if body.User.Email != "happy@test.local" {
		t.Errorf("email: got %q", body.User.Email)
	}
	if body.User.Plan != "free" {
		t.Errorf("plan: got %q, want free", body.User.Plan)
	}
	if body.User.Slug == "" {
		t.Error("slug empty")
	}
}

func TestRegister_DuplicateEmail_Returns422(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()

	body := map[string]any{"email": "dup@test.local", "password": "password123"}
	s.POST(t, "/api/auth/register", body) // first succeeds
	resp := s.POST(t, "/api/auth/register", body)
	harness.AssertError(t, resp, 422, "VALIDATION_FAILED")
}

func TestRegister_MalformedJSON_Returns400(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	resp := s.PostRaw(t, "/api/auth/register", "application/json", []byte("{"))
	harness.AssertError(t, resp, 400, "MALFORMED_JSON")
}

func TestRegister_ShortPassword_Returns422(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	resp := s.POST(t, "/api/auth/register", map[string]any{
		"email": "short@test.local", "password": "abc",
	})
	harness.AssertError(t, resp, 422, "VALIDATION_FAILED")
}

func TestRegister_InvalidEmail_Returns422(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	resp := s.POST(t, "/api/auth/register", map[string]any{
		"email": "not-an-email", "password": "password123",
	})
	harness.AssertError(t, resp, 422, "VALIDATION_FAILED")
}

func TestLogin_Happy_Returns200WithCookie(t *testing.T) {
	h := harness.New(t)

	s1 := h.App.NewSession()
	s1.POST(t, "/api/auth/register", map[string]any{"email": "login@test.local", "password": "password123"})

	s2 := h.App.NewSession()
	resp := s2.POST(t, "/api/auth/login", map[string]any{"email": "login@test.local", "password": "password123"})
	harness.AssertStatus(t, resp, 200)

	// Subsequent authenticated call works
	me := s2.GET(t, "/api/monitors")
	if me.Status == 401 {
		t.Fatalf("login did not persist cookie; got 401")
	}
}

func TestLogin_WrongPassword_Returns401(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	s.POST(t, "/api/auth/register", map[string]any{"email": "bad@test.local", "password": "password123"})

	s2 := h.App.NewSession()
	resp := s2.POST(t, "/api/auth/login", map[string]any{"email": "bad@test.local", "password": "wrongpass"})
	harness.AssertError(t, resp, 401, "UNAUTHORIZED")
}

func TestLogin_NonexistentEmail_Returns401(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	resp := s.POST(t, "/api/auth/login", map[string]any{"email": "nobody@test.local", "password": "x"})
	harness.AssertError(t, resp, 401, "UNAUTHORIZED")
}

func TestLogout_Happy_Returns204(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")

	resp := s.POST(t, "/api/auth/logout", nil)
	harness.AssertStatus(t, resp, 204)

	// After logout, protected endpoint is 401
	resp2 := s.GET(t, "/api/monitors")
	harness.AssertError(t, resp2, 401, "UNAUTHORIZED")
}

func TestLogout_Unauthenticated_Returns401(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	resp := s.POST(t, "/api/auth/logout", nil)
	harness.AssertError(t, resp, 401, "UNAUTHORIZED")
}
```

- [ ] **Step 2: Run — note failures**

Run: `go test -tags=integration ./tests/integration/api/... -run 'TestRegister|TestLogin|TestLogout' -v`

Expected some failures — iterate code fixes until green. Common expected fixes:
- `TestRegister_Happy` might fail because registration returns a different shape — align handler to `{user:{...}}` per spec §4.
- `TestRegister_DuplicateEmail` might return 409 in code — spec says 422. Fix handler to return `ErrValidation` wrapping "email already taken".
- `TestLogin_*` similar shape alignments.

For each divergence: fix the handler/service (not the test). Commit after each green step using `git add -u && git commit -m "fix(c1): <what>"`.

- [ ] **Step 3: Final green run**

Run: `go test -tags=integration ./tests/integration/api/... -run 'TestRegister|TestLogin|TestLogout' -v`
Expected: all PASS.

- [ ] **Step 4: Commit the test file and any residual fixes**

```bash
git add -A
git commit -m "test(c1): auth endpoints — register/login/logout + edge cases"
```

---

## Task 18: Monitor endpoint tests

**File:** `tests/integration/api/monitors_test.go`

- [ ] **Step 1: Write test suite**

`tests/integration/api/monitors_test.go`:

```go
//go:build integration

package api

import (
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func newMonitor(name string) map[string]any {
	return map[string]any{
		"name":              name,
		"type":              "http",
		"url":               "https://example.com",
		"interval_seconds":  300,
		"timeout_seconds":   10,
	}
}

// --- List -----------------------------------------------------------

func TestListMonitors_Empty_Returns200EmptyArray(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.GET(t, "/api/monitors")
	harness.AssertStatus(t, resp, 200)
	if string(resp.Body) != "[]" && string(resp.Body) != "[]\n" {
		t.Errorf("want empty array, got %s", resp.Body)
	}
}

func TestListMonitors_Unauthenticated_Returns401(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	resp := s.GET(t, "/api/monitors")
	harness.AssertError(t, resp, 401, "UNAUTHORIZED")
}

// --- Create ---------------------------------------------------------

func TestCreateMonitor_Happy_Returns201(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.POST(t, "/api/monitors", newMonitor("api"))
	harness.AssertStatus(t, resp, 201)

	var m struct {
		ID   string
		Name string
		URL  string
	}
	resp.JSON(t, &m)
	if m.Name != "api" {
		t.Errorf("name: %q", m.Name)
	}
	if m.ID == "" {
		t.Error("id empty")
	}
}

func TestCreateMonitor_DuplicateName_Returns409(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	s.POST(t, "/api/monitors", newMonitor("api"))
	resp := s.POST(t, "/api/monitors", newMonitor("api"))
	harness.AssertError(t, resp, 409, "CONFLICT")
}

func TestCreateMonitor_IntervalBelowTierMin_Returns422(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	body := newMonitor("too-fast")
	body["interval_seconds"] = 30 // Free plan min is 300
	resp := s.POST(t, "/api/monitors", body)
	harness.AssertError(t, resp, 422, "VALIDATION_FAILED")
}

func TestCreateMonitor_BadURL_Returns422(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	body := newMonitor("bad")
	body["url"] = "not a url"
	resp := s.POST(t, "/api/monitors", body)
	harness.AssertError(t, resp, 422, "VALIDATION_FAILED")
}

func TestCreateMonitor_TimeoutExceedsInterval_Returns422(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	body := newMonitor("slow-timeout")
	body["timeout_seconds"] = 600 // > interval 300
	resp := s.POST(t, "/api/monitors", body)
	harness.AssertError(t, resp, 422, "VALIDATION_FAILED")
}

func TestCreateMonitor_FreeTierLimit_Returns422(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")

	for i := 0; i < 5; i++ {
		resp := s.POST(t, "/api/monitors", newMonitor("m"+itoa(i)))
		harness.AssertStatus(t, resp, 201)
	}
	// 6th exceeds Free plan limit
	resp := s.POST(t, "/api/monitors", newMonitor("m6"))
	body := harness.AssertError(t, resp, 422, "VALIDATION_FAILED")
	if body.Error.Message == "" {
		t.Error("expected descriptive validation message")
	}
}

// --- Get ------------------------------------------------------------

func TestGetMonitor_Happy_ReturnsDetail(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/monitors", newMonitor("api"))
	var created struct{ ID string }
	cr.JSON(t, &created)

	resp := s.GET(t, "/api/monitors/"+created.ID)
	harness.AssertStatus(t, resp, 200)
}

func TestGetMonitor_MalformedID_Returns400(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.GET(t, "/api/monitors/not-a-uuid")
	harness.AssertError(t, resp, 400, "MALFORMED_PARAM")
}

func TestGetMonitor_NotFound_Returns404(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.GET(t, "/api/monitors/00000000-0000-0000-0000-000000000000")
	harness.AssertError(t, resp, 404, "NOT_FOUND")
}

func TestGetMonitor_CrossTenant_Returns403(t *testing.T) {
	h := harness.New(t)
	sA, sB := h.TwoSessions(t)
	cr := sA.POST(t, "/api/monitors", newMonitor("alpha"))
	var m struct{ ID string }
	cr.JSON(t, &m)

	resp := sB.GET(t, "/api/monitors/"+m.ID)
	harness.AssertError(t, resp, 403, "FORBIDDEN_TENANT")
}

// --- Update ---------------------------------------------------------

func TestUpdateMonitor_Happy_Returns200(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/monitors", newMonitor("api"))
	var m struct{ ID string }
	cr.JSON(t, &m)

	resp := s.PUT(t, "/api/monitors/"+m.ID, map[string]any{
		"name": "api-v2",
	})
	harness.AssertStatus(t, resp, 200)

	var updated struct{ Name string }
	resp.JSON(t, &updated)
	if updated.Name != "api-v2" {
		t.Errorf("name not updated: got %q", updated.Name)
	}
}

func TestUpdateMonitor_RenameToExisting_Returns409(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	s.POST(t, "/api/monitors", newMonitor("alpha"))
	cr := s.POST(t, "/api/monitors", newMonitor("beta"))
	var m struct{ ID string }
	cr.JSON(t, &m)

	resp := s.PUT(t, "/api/monitors/"+m.ID, map[string]any{"name": "alpha"})
	harness.AssertError(t, resp, 409, "CONFLICT")
}

// --- Delete ---------------------------------------------------------

func TestDeleteMonitor_Happy_Returns204(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/monitors", newMonitor("api"))
	var m struct{ ID string }
	cr.JSON(t, &m)

	resp := s.DELETE(t, "/api/monitors/"+m.ID)
	harness.AssertStatus(t, resp, 204)

	// Double-delete is 404
	resp2 := s.DELETE(t, "/api/monitors/"+m.ID)
	harness.AssertError(t, resp2, 404, "NOT_FOUND")
}

func TestDeleteMonitor_CrossTenant_Returns403(t *testing.T) {
	h := harness.New(t)
	sA, sB := h.TwoSessions(t)
	cr := sA.POST(t, "/api/monitors", newMonitor("alpha"))
	var m struct{ ID string }
	cr.JSON(t, &m)

	resp := sB.DELETE(t, "/api/monitors/"+m.ID)
	harness.AssertError(t, resp, 403, "FORBIDDEN_TENANT")
}

// --- Pause ----------------------------------------------------------

func TestPauseMonitor_Toggles(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/monitors", newMonitor("api"))
	var m struct{ ID string }
	cr.JSON(t, &m)

	resp := s.POST(t, "/api/monitors/"+m.ID+"/pause", nil)
	harness.AssertStatus(t, resp, 200)

	var afterFirst struct{ Paused bool }
	resp.JSON(t, &afterFirst)
	if !afterFirst.Paused {
		t.Error("expected paused after first toggle")
	}

	resp2 := s.POST(t, "/api/monitors/"+m.ID+"/pause", nil)
	var afterSecond struct{ Paused bool }
	resp2.JSON(t, &afterSecond)
	if afterSecond.Paused {
		t.Error("expected running after second toggle")
	}
}

func TestPauseMonitor_CrossTenant_Returns403(t *testing.T) {
	h := harness.New(t)
	sA, sB := h.TwoSessions(t)
	cr := sA.POST(t, "/api/monitors", newMonitor("alpha"))
	var m struct{ ID string }
	cr.JSON(t, &m)

	resp := sB.POST(t, "/api/monitors/"+m.ID+"/pause", nil)
	harness.AssertError(t, resp, 403, "FORBIDDEN_TENANT")
}

// --- Monitor types --------------------------------------------------

func TestListMonitorTypes_Authenticated_Returns200Array(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.GET(t, "/api/monitor-types")
	harness.AssertStatus(t, resp, 200)
	if len(resp.Body) < 3 { // at least "[]" with real content
		t.Errorf("response too short: %s", resp.Body)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
```

- [ ] **Step 2: Iterate red → green**

Run: `go test -tags=integration ./tests/integration/api/... -run 'TestListMonitor|TestCreateMonitor|TestGetMonitor|TestUpdateMonitor|TestDeleteMonitor|TestPauseMonitor' -v`

For each failing test:
- Understand the divergence between spec §4 and handler behavior.
- Change the handler/service — never the test.
- Re-run until green.
- Commit each green state with `fix(c1): <specific change>`.

Key likely fixes during this task:
- Ensure duplicate monitor name returns 409 (not 422) — spec says 409 for duplicate-key.
- Ensure `interval < tier min` returns 422 via `ErrValidation`.
- Ensure `GET /api/monitors/{not-uuid}` returns 400 `MALFORMED_PARAM` (may need tweaking oapi-codegen's generated UUID parser path or intercepting in middleware).
- Ensure cross-tenant returns 403 (many handlers today return 404 when repo returns nil — must map to 403 after verifying the resource exists).

- [ ] **Step 3: Commit test file and all related fixes**

```bash
git add -A
git commit -m "test(c1): monitor CRUD + pause + cross-tenant"
```

---

## Task 19: Channel endpoint tests

**File:** `tests/integration/api/channels_test.go`

- [ ] **Step 1: Write tests**

`tests/integration/api/channels_test.go`:

```go
//go:build integration

package api

import (
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func telegramChannel(name string) map[string]any {
	return map[string]any{
		"name":   name,
		"type":   "telegram",
		"config": map[string]any{"bot_token": "12345:SECRETDEADBEEF", "chat_id": "777"},
	}
}

func webhookChannel(name string) map[string]any {
	return map[string]any{
		"name":   name,
		"type":   "webhook",
		"config": map[string]any{"url": "https://example.com/hook?token=SECRET9999"},
	}
}

func TestListChannels_Empty_ReturnsEmptyArray(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.GET(t, "/api/channels")
	harness.AssertStatus(t, resp, 200)
}

func TestCreateChannel_Telegram_Happy_Returns201Redacted(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.POST(t, "/api/channels", telegramChannel("ops"))
	harness.AssertStatus(t, resp, 201)

	var got struct {
		ID     string
		Config map[string]any
	}
	resp.JSON(t, &got)
	if tok, _ := got.Config["bot_token"].(string); tok == "12345:SECRETDEADBEEF" {
		t.Errorf("bot_token not redacted: %q", tok)
	}
}

func TestCreateChannel_Webhook_Happy_Returns201(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.POST(t, "/api/channels", webhookChannel("alerts"))
	harness.AssertStatus(t, resp, 201)
}

func TestCreateChannel_FreeTierEmail_Returns422PlanUpgrade(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.POST(t, "/api/channels", map[string]any{
		"name":   "email-ch",
		"type":   "email",
		"config": map[string]any{"to": "ops@company.com"},
	})
	body := harness.AssertError(t, resp, 422, "VALIDATION_FAILED")
	if body.Error.Message == "" {
		t.Error("expected upgrade hint message")
	}
}

func TestCreateChannel_UnknownType_Returns422(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.POST(t, "/api/channels", map[string]any{
		"name":   "weird",
		"type":   "carrier-pigeon",
		"config": map[string]any{},
	})
	harness.AssertError(t, resp, 422, "VALIDATION_FAILED")
}

func TestCreateChannel_EmptyName_Returns422(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	body := telegramChannel("")
	resp := s.POST(t, "/api/channels", body)
	harness.AssertError(t, resp, 422, "VALIDATION_FAILED")
}

func TestUpdateChannel_Happy_Returns200(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/channels", telegramChannel("ops"))
	var ch struct{ ID string }
	cr.JSON(t, &ch)

	resp := s.PUT(t, "/api/channels/"+ch.ID, map[string]any{"name": "ops-v2"})
	harness.AssertStatus(t, resp, 200)
}

func TestUpdateChannel_CrossTenant_Returns403(t *testing.T) {
	h := harness.New(t)
	sA, sB := h.TwoSessions(t)
	cr := sA.POST(t, "/api/channels", telegramChannel("a"))
	var ch struct{ ID string }
	cr.JSON(t, &ch)

	resp := sB.PUT(t, "/api/channels/"+ch.ID, map[string]any{"name": "hack"})
	harness.AssertError(t, resp, 403, "FORBIDDEN_TENANT")
}

func TestDeleteChannel_Happy_Returns204(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/channels", telegramChannel("ops"))
	var ch struct{ ID string }
	cr.JSON(t, &ch)

	resp := s.DELETE(t, "/api/channels/"+ch.ID)
	harness.AssertStatus(t, resp, 204)

	// Double-delete is 404
	resp2 := s.DELETE(t, "/api/channels/"+ch.ID)
	harness.AssertError(t, resp2, 404, "NOT_FOUND")
}

func TestListChannelTypes_Returns200(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	resp := s.GET(t, "/api/channel-types")
	harness.AssertStatus(t, resp, 200)
}
```

- [ ] **Step 2: Iterate red → green**

Run: `go test -tags=integration ./tests/integration/api/... -run 'TestListChannel|TestCreateChannel|TestUpdateChannel|TestDeleteChannel|TestListChannelTypes' -v`

Expected iterations: redaction must work (Task 15 added it for `GET` — verify it's applied for `POST` and `PUT` too). Free-tier email block needs to surface as 422 (`VALIDATION_FAILED`) with message mentioning upgrade — current code in `alert.go`/`monitoring.go` may not apply this check at the API layer.

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "test(c1): channel CRUD + redaction + free-tier email block"
```

---

## Task 20: Monitor-channel binding tests

**File:** `tests/integration/api/monitor_channels_test.go`

- [ ] **Step 1: Write tests**

`tests/integration/api/monitor_channels_test.go`:

```go
//go:build integration

package api

import (
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func setupMonitorAndChannel(t *testing.T, s *harness.Session) (monitorID, channelID string) {
	t.Helper()
	m := s.POST(t, "/api/monitors", newMonitor("api"))
	harness.AssertStatus(t, m, 201)
	var mm struct{ ID string }
	m.JSON(t, &mm)

	c := s.POST(t, "/api/channels", telegramChannel("ops"))
	harness.AssertStatus(t, c, 201)
	var cc struct{ ID string }
	c.JSON(t, &cc)

	return mm.ID, cc.ID
}

func TestBindChannel_Happy_Returns201WithBinding(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	mID, cID := setupMonitorAndChannel(t, s)

	resp := s.POST(t, "/api/monitors/"+mID+"/channels", map[string]any{"channel_id": cID})
	harness.AssertStatus(t, resp, 201)

	var body struct {
		MonitorID string `json:"monitor_id"`
		ChannelID string `json:"channel_id"`
		BoundAt   string `json:"bound_at"`
	}
	resp.JSON(t, &body)
	if body.MonitorID != mID || body.ChannelID != cID {
		t.Errorf("binding body mismatch: %+v", body)
	}
	if body.BoundAt == "" {
		t.Error("bound_at missing")
	}
}

func TestBindChannel_AlreadyBound_Returns200Idempotent(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	mID, cID := setupMonitorAndChannel(t, s)

	r1 := s.POST(t, "/api/monitors/"+mID+"/channels", map[string]any{"channel_id": cID})
	harness.AssertStatus(t, r1, 201)

	r2 := s.POST(t, "/api/monitors/"+mID+"/channels", map[string]any{"channel_id": cID})
	harness.AssertStatus(t, r2, 200)

	// bound_at should be stable across idempotent re-bind
	var b1, b2 struct{ BoundAt string `json:"bound_at"` }
	r1.JSON(t, &b1)
	r2.JSON(t, &b2)
	if b1.BoundAt != b2.BoundAt {
		t.Errorf("bound_at changed on re-bind: %q vs %q", b1.BoundAt, b2.BoundAt)
	}
}

func TestBindChannel_ForeignMonitor_Returns403(t *testing.T) {
	h := harness.New(t)
	sA, sB := h.TwoSessions(t)
	mID, _ := setupMonitorAndChannel(t, sA)
	_, cID := setupMonitorAndChannel(t, sB)

	resp := sB.POST(t, "/api/monitors/"+mID+"/channels", map[string]any{"channel_id": cID})
	harness.AssertError(t, resp, 403, "FORBIDDEN_TENANT")
}

func TestBindChannel_ForeignChannel_Returns403(t *testing.T) {
	h := harness.New(t)
	sA, sB := h.TwoSessions(t)
	mA, _ := setupMonitorAndChannel(t, sA)
	_, cB := setupMonitorAndChannel(t, sB)

	resp := sA.POST(t, "/api/monitors/"+mA+"/channels", map[string]any{"channel_id": cB})
	harness.AssertError(t, resp, 403, "FORBIDDEN_TENANT")
}

func TestUnbindChannel_Happy_Returns204(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	mID, cID := setupMonitorAndChannel(t, s)
	s.POST(t, "/api/monitors/"+mID+"/channels", map[string]any{"channel_id": cID})

	resp := s.DELETE(t, "/api/monitors/"+mID+"/channels/"+cID)
	harness.AssertStatus(t, resp, 204)
}

func TestUnbindChannel_NotBound_Returns404(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	mID, cID := setupMonitorAndChannel(t, s)

	resp := s.DELETE(t, "/api/monitors/"+mID+"/channels/"+cID)
	harness.AssertError(t, resp, 404, "NOT_FOUND")
}
```

- [ ] **Step 2: Iterate red → green**

Run: `go test -tags=integration ./tests/integration/api/... -run 'TestBindChannel|TestUnbindChannel' -v`

Expected fixes:
- BindChannel must return the binding object on first bind (current code likely returns empty body).
- BindChannel must be idempotent on re-bind → 200 with same `bound_at`.

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "test(c1): monitor-channel binding + idempotency"
```

---

## Task 21: API keys endpoint tests

**File:** `tests/integration/api/apikeys_test.go`

- [ ] **Step 1: Write tests**

```go
//go:build integration

package api

import (
	"strings"
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestCreateAPIKey_Happy_ReturnsTokenOnce(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")

	resp := s.POST(t, "/api/api-keys", map[string]any{"name": "ci-bot"})
	harness.AssertStatus(t, resp, 201)

	var body struct {
		ID    string
		Name  string
		Token string
	}
	resp.JSON(t, &body)
	if !strings.HasPrefix(body.Token, "pck_") {
		t.Errorf("token should start with pck_: %q", body.Token)
	}
	if len(body.Token) < 20 {
		t.Errorf("token too short: %q", body.Token)
	}
}

func TestListAPIKeys_DoesNotReturnToken(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	createResp := s.POST(t, "/api/api-keys", map[string]any{"name": "ci-bot"})
	var created struct{ Token string }
	createResp.JSON(t, &created)

	resp := s.GET(t, "/api/api-keys")
	harness.AssertStatus(t, resp, 200)
	if strings.Contains(string(resp.Body), created.Token) {
		t.Error("list endpoint leaked full token")
	}
}

func TestAPIKey_BearerAuth_Works(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/api-keys", map[string]any{"name": "ci-bot"})
	var key struct{ Token string }
	cr.JSON(t, &key)

	bearer := h.App.WithBearer(key.Token)
	resp := bearer.GET(t, "/api/monitors")
	if resp.Status == 401 {
		t.Fatalf("bearer auth failed: %s", resp.Body)
	}
}

func TestAPIKey_CannotListOwnKeys(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/api-keys", map[string]any{"name": "ci-bot"})
	var key struct{ Token string }
	cr.JSON(t, &key)

	bearer := h.App.WithBearer(key.Token)
	resp := bearer.GET(t, "/api/api-keys")
	harness.AssertError(t, resp, 401, "UNAUTHORIZED")
}

func TestRevokeAPIKey_InvalidatesFurtherCalls(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/api-keys", map[string]any{"name": "ci-bot"})
	var key struct {
		ID    string
		Token string
	}
	cr.JSON(t, &key)

	del := s.DELETE(t, "/api/api-keys/"+key.ID)
	harness.AssertStatus(t, del, 204)

	bearer := h.App.WithBearer(key.Token)
	resp := bearer.GET(t, "/api/monitors")
	harness.AssertError(t, resp, 401, "UNAUTHORIZED")
}

func TestRevokeAPIKey_CrossTenant_Returns403(t *testing.T) {
	h := harness.New(t)
	sA, sB := h.TwoSessions(t)
	cr := sA.POST(t, "/api/api-keys", map[string]any{"name": "a-key"})
	var key struct{ ID string }
	cr.JSON(t, &key)

	resp := sB.DELETE(t, "/api/api-keys/"+key.ID)
	harness.AssertError(t, resp, 403, "FORBIDDEN_TENANT")
}
```

- [ ] **Step 2: Iterate red → green**

Run: `go test -tags=integration ./tests/integration/api/... -run 'TestCreateAPIKey|TestListAPIKeys|TestAPIKey|TestRevokeAPIKey' -v`

Iterate. Key fixes:
- Auth middleware must distinguish session vs key and reject API-key auth on `/api/api-keys/*`.
- Cross-tenant revoke must return 403, not 404.

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "test(c1): api-keys — create, list, bearer-auth, revoke, isolation"
```

---

## Task 22: Public status page tests

**File:** `tests/integration/api/status_test.go`

- [ ] **Step 1: Write tests**

```go
//go:build integration

package api

import (
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestStatusPage_ValidSlug_Returns200(t *testing.T) {
	h := harness.New(t)
	s := h.RegisterAndLogin(t, "slug-owner@test.local", "password123")
	_ = s
	// Query own slug. Status page is public — no auth on read.
	pub := h.App.NewSession()

	// Need the slug. Read from DB (allowed as inspection, not setup):
	var slug string
	if err := h.App.Pool.QueryRow(t.Context(), `SELECT slug FROM users WHERE email=$1`, "slug-owner@test.local").Scan(&slug); err != nil {
		t.Fatalf("select slug: %v", err)
	}

	resp := pub.GET(t, "/api/status/"+slug)
	harness.AssertStatus(t, resp, 200)
}

func TestStatusPage_UnknownSlug_Returns404(t *testing.T) {
	h := harness.New(t)
	pub := h.App.NewSession()
	resp := pub.GET(t, "/api/status/no-such-slug")
	harness.AssertError(t, resp, 404, "NOT_FOUND")
}

func TestStatusPage_HasCacheControl(t *testing.T) {
	h := harness.New(t)
	h.RegisterAndLogin(t, "cache-owner@test.local", "password123")
	var slug string
	_ = h.App.Pool.QueryRow(t.Context(), `SELECT slug FROM users WHERE email=$1`, "cache-owner@test.local").Scan(&slug)

	pub := h.App.NewSession()
	resp := pub.GET(t, "/api/status/"+slug)
	harness.AssertStatus(t, resp, 200)

	cc := resp.Headers.Get("Cache-Control")
	if cc == "" {
		t.Error("Cache-Control header missing")
	}
}
```

- [ ] **Step 2: Iterate red → green**

Run: `go test -tags=integration ./tests/integration/api/... -run TestStatusPage -v`

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "test(c1): public status page contract"
```

---

## Task 23: Health probe tests

**File:** extend `tests/integration/api/health_test.go`

- [ ] **Step 1: Replace smoke test with full suite**

Replace the file with:

```go
//go:build integration

package api

import (
	"encoding/json"
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestHealth_Returns200(t *testing.T) {
	h := harness.New(t)
	resp := h.App.NewSession().GET(t, "/health")
	harness.AssertStatus(t, resp, 200)
}

func TestHealthz_Returns200(t *testing.T) {
	h := harness.New(t)
	resp := h.App.NewSession().GET(t, "/healthz")
	harness.AssertStatus(t, resp, 200)
}

func TestReadyz_WithAllDeps_Returns200(t *testing.T) {
	h := harness.New(t)
	resp := h.App.NewSession().GET(t, "/readyz")
	harness.AssertStatus(t, resp, 200)

	var body struct {
		Status string
		Deps   map[string]string
	}
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, resp.Body)
	}
	if body.Status != "ok" {
		t.Errorf("status: %q", body.Status)
	}
	if body.Deps["postgres"] != "ok" || body.Deps["redis"] != "ok" {
		t.Errorf("deps not all ok: %+v", body.Deps)
	}
}
```

- [ ] **Step 2: Run — expect mostly green; iterate /readyz shape if needed**

Run: `go test -tags=integration ./tests/integration/api/... -run TestHealth -v`

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "test(c1): health + healthz + readyz"
```

---

## Task 24: Webhook tests (LemonSqueezy HMAC, Telegram token)

**File:** `tests/integration/api/webhook_test.go`

- [ ] **Step 1: Write tests**

```go
//go:build integration

package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

const lsSecret = "test-ls-secret" // matches harness/app.go

func sign(body []byte) string {
	mac := hmac.New(sha256.New, []byte(lsSecret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestLemonSqueezy_BadSignature_Returns401(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	body := []byte(`{"event_id":"e1","event":"subscription_created","data":{}}`)

	req := harness.NewRawRequest("POST", "/webhook/lemonsqueezy", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", "not-a-valid-signature")
	resp, err := h.App.Fiber.Test(req, -1)
	if err != nil {
		t.Fatalf("fiber test: %v", err)
	}
	_ = s
	if resp.StatusCode != 401 {
		t.Fatalf("status: want 401, got %d", resp.StatusCode)
	}
}

func TestLemonSqueezy_ValidSignature_Returns200(t *testing.T) {
	h := harness.New(t)
	body := []byte(`{"event_id":"e1","event":"subscription_created","data":{"user_email":"u@test.local"}}`)

	req := harness.NewRawRequest("POST", "/webhook/lemonsqueezy", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", sign(body))
	resp, err := h.App.Fiber.Test(req, -1)
	if err != nil {
		t.Fatalf("fiber test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status: want 200, got %d", resp.StatusCode)
	}
}

func TestLemonSqueezy_DuplicateEventID_Returns200NoReprocess(t *testing.T) {
	h := harness.New(t)
	body := []byte(`{"event_id":"dup-1","event":"subscription_created","data":{"user_email":"u@test.local"}}`)
	sig := sign(body)

	r1 := sendSigned(t, h, body, sig)
	if r1.Status != 200 {
		t.Fatalf("first: %d", r1.Status)
	}

	r2 := sendSigned(t, h, body, sig)
	if r2.Status != 200 {
		t.Fatalf("second: %d", r2.Status)
	}
	// Side-effect verification (e.g. user plan upgrade) belongs to C2.
	// Here we only assert 200 + the request was acknowledged twice.
}

func TestTelegramWebhook_BadToken_Returns401(t *testing.T) {
	h := harness.New(t)
	req := harness.NewRawRequest("POST", "/webhook/telegram/wrong-token", []byte(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.App.Fiber.Test(req, -1)
	if err != nil {
		t.Fatalf("fiber test: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
}

// sendSigned posts a pre-signed LS event and returns the Response.
func sendSigned(t *testing.T, h *harness.Harness, body []byte, sig string) *harness.Response {
	t.Helper()
	req := harness.NewRawRequest("POST", "/webhook/lemonsqueezy", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", sig)
	resp, err := h.App.Fiber.Test(req, -1)
	if err != nil {
		t.Fatalf("fiber test: %v", err)
	}
	raw, _ := harness.ReadAllClose(resp.Body)
	return &harness.Response{Status: resp.StatusCode, Headers: resp.Header, Body: raw}
}
```

- [ ] **Step 2: Add helpers to harness**

Append to `tests/integration/api/harness/session.go`:

```go
// NewRawRequest constructs an *http.Request suitable for fiber.App.Test
// with a precise body. Callers set headers manually.
func NewRawRequest(method, path string, body []byte) *http.Request {
	return httptest.NewRequest(method, path, bytes.NewReader(body))
}

// ReadAllClose drains and closes a response body.
func ReadAllClose(rc io.ReadCloser) ([]byte, error) {
	b, err := io.ReadAll(rc)
	_ = rc.Close()
	return b, err
}
```

- [ ] **Step 3: Iterate red → green**

Run: `go test -tags=integration ./tests/integration/api/... -run 'TestLemonSqueezy|TestTelegramWebhook' -v`

Expected fixes:
- Idempotency store for LS event IDs (Redis key + NX+EX TTL).
- Telegram webhook token comparison must use `crypto/subtle.ConstantTimeCompare`.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "test(c1): webhook HMAC + idempotency + token auth"
```

---

## Task 25: Auth-matrix sweep

**File:** `tests/integration/api/authmatrix_test.go`

Per-endpoint table: every protected endpoint × {no-auth, bad-cookie, revoked-key}.

- [ ] **Step 1: Write sweep**

Append to `tests/integration/api/authmatrix_test.go` (file was started in Task 16):

```go
//go:build integration

package api

import (
	"testing"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

type authCase struct {
	Method string
	Path   string
}

var protectedEndpoints = []authCase{
	{"GET", "/api/monitors"},
	{"POST", "/api/monitors"},
	{"GET", "/api/monitor-types"},
	{"GET", "/api/monitors/00000000-0000-0000-0000-000000000000"},
	{"PUT", "/api/monitors/00000000-0000-0000-0000-000000000000"},
	{"DELETE", "/api/monitors/00000000-0000-0000-0000-000000000000"},
	{"POST", "/api/monitors/00000000-0000-0000-0000-000000000000/pause"},
	{"GET", "/api/channels"},
	{"POST", "/api/channels"},
	{"GET", "/api/channels/00000000-0000-0000-0000-000000000000"},
	{"PUT", "/api/channels/00000000-0000-0000-0000-000000000000"},
	{"DELETE", "/api/channels/00000000-0000-0000-0000-000000000000"},
	{"POST", "/api/monitors/00000000-0000-0000-0000-000000000000/channels"},
	{"DELETE", "/api/monitors/00000000-0000-0000-0000-000000000000/channels/00000000-0000-0000-0000-000000000000"},
	{"GET", "/api/api-keys"},
	{"POST", "/api/api-keys"},
	{"DELETE", "/api/api-keys/00000000-0000-0000-0000-000000000000"},
	{"POST", "/api/auth/logout"},
}

func TestAuthMatrix_NoAuth_Returns401(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	for _, c := range protectedEndpoints {
		t.Run(c.Method+" "+c.Path, func(t *testing.T) {
			resp := s.Do(t, c.Method, c.Path, map[string]any{})
			if resp.Status != 401 {
				t.Errorf("status: want 401, got %d (body=%s)", resp.Status, resp.Body)
			}
		})
	}
}

func TestAuthMatrix_BadCookie_Returns401(t *testing.T) {
	h := harness.New(t)
	s := h.App.NewSession()
	// inject a fake session cookie by hand
	s.InjectCookie("pingcast_session", "totally-fake-session-id")

	for _, c := range protectedEndpoints {
		t.Run(c.Method+" "+c.Path, func(t *testing.T) {
			resp := s.Do(t, c.Method, c.Path, map[string]any{})
			if resp.Status != 401 {
				t.Errorf("status: want 401, got %d", resp.Status)
			}
		})
	}
}

func TestAuthMatrix_RevokedKey_Returns401(t *testing.T) {
	h := harness.New(t)
	owner := h.RegisterAndLogin(t, "", "")
	cr := owner.POST(t, "/api/api-keys", map[string]any{"name": "doomed"})
	var k struct {
		ID    string
		Token string
	}
	cr.JSON(t, &k)
	owner.DELETE(t, "/api/api-keys/"+k.ID)

	bearer := h.App.WithBearer(k.Token)
	for _, c := range protectedEndpoints {
		// /api/api-keys/* is session-only; we expect 401 regardless
		t.Run(c.Method+" "+c.Path, func(t *testing.T) {
			resp := bearer.Do(t, c.Method, c.Path, map[string]any{})
			if resp.Status != 401 {
				t.Errorf("status: want 401, got %d", resp.Status)
			}
		})
	}
}
```

- [ ] **Step 2: Add InjectCookie helper**

Append to `tests/integration/api/harness/session.go`:

```go
// InjectCookie adds a raw cookie to the session's jar (useful for
// negative tests with invalid sessions).
func (s *Session) InjectCookie(name, value string) {
	s.cookies = append(s.cookies, &http.Cookie{Name: name, Value: value, Path: "/"})
}
```

- [ ] **Step 3: Run**

Run: `go test -tags=integration ./tests/integration/api/... -run TestAuthMatrix -v`

Fix any leak (an endpoint returning 404 before auth check, revealing existence).

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "test(c1): auth matrix sweep over all protected endpoints"
```

---

## Task 26: Move repo_test.go under the adapter

**Files:**
- Delete: `tests/integration/repo_test.go`, `tests/integration/testdb.go`
- Create: `internal/adapter/postgres/repo_integration_test.go`
- Create: `internal/adapter/postgres/testdb_test.go`

- [ ] **Step 1: Move and tag files**

```bash
git mv tests/integration/repo_test.go internal/adapter/postgres/repo_integration_test.go
git mv tests/integration/testdb.go internal/adapter/postgres/testdb_test.go
```

Edit both files:
- Change package to `postgres_test` or `postgres` (match existing test file conventions — likely `postgres`).
- Prepend `//go:build integration` to each.
- Adjust imports: drop self-imports of `internal/adapter/postgres` (now same package) or keep if external-test.

- [ ] **Step 2: Run old tests at new location**

Run: `go test -tags=integration ./internal/adapter/postgres/... -v`
Expected: same tests pass as before (same containers, just different package).

- [ ] **Step 3: Remove now-empty directory**

```bash
rmdir tests/integration || true
```

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "refactor(c1): relocate repo integration tests under the adapter"
```

---

# Phase 5 — CI + docs

## Task 27: GitHub Actions workflow

**File:** `.github/workflows/integration.yml`

- [ ] **Step 1: Create workflow**

```yaml
name: integration

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  integration:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.26'
          cache: true

      - name: Go mod download
        run: go mod download

      - name: Run API integration tests
        run: go test -tags=integration -timeout=8m ./tests/integration/api/...

      - name: Run repo integration tests
        run: go test -tags=integration -timeout=4m ./internal/adapter/postgres/...
```

- [ ] **Step 2: Smoke-check with `act` (optional) or push a draft PR**

Run: `act -j integration` if `act` is installed; otherwise skip.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/integration.yml
git commit -m "ci(c1): integration workflow"
```

---

## Task 28: README update + Makefile target

**Files:**
- Modify: `README.md`
- Create: `Makefile` (or append to existing)

- [ ] **Step 1: Update Testing table**

Edit `README.md`, find the Testing table, change to:

```
| Layer | Tool | Count |
|---|---|---|
| Go unit | `go test -short ./...` | many |
| Go repo integration | testcontainers + postgres | ~20 tests |
| Go API integration (C1) | testcontainers + postgres + redis + nats | ~95 tests |
| Frontend E2E | Playwright | 11 specs |
```

Add a new section after:

```
### Integration tests

API-level black-box tests live in `tests/integration/api/`. They boot
real Postgres, Redis, and NATS containers once per package run and
reset state between tests. Build tag is `integration`:

\`\`\`
go test -tags=integration ./tests/integration/api/...
\`\`\`

Or via Make:

\`\`\`
make test-integration
\`\`\`

Tests are black-box: they talk to the API via HTTP and assert the
canonical envelope shape per the behavior-spec in
`docs/superpowers/specs/2026-04-18-C1-api-behavior-spec.md`.
```

- [ ] **Step 2: Create/append Makefile**

If `Makefile` exists, append these targets. Otherwise create:

```makefile
.PHONY: test test-short test-integration lint

test-short:
	go test -short ./...

test:
	go test ./...

test-integration:
	go test -tags=integration -timeout=10m ./tests/integration/api/... ./internal/adapter/postgres/...

lint:
	golangci-lint run
```

- [ ] **Step 3: Commit**

```bash
git add README.md Makefile
git commit -m "docs(c1): document integration test layer + make target"
```

---

## Task 29: Final green-on-green verification

- [ ] **Step 1: Full build**

Run: `go build ./...`
Expected: success.

- [ ] **Step 2: Full unit suite**

Run: `go test -short ./...`
Expected: PASS.

- [ ] **Step 3: Full integration suite**

Run: `go test -tags=integration -timeout=10m ./tests/integration/api/... ./internal/adapter/postgres/...`
Expected: PASS. Test count reported.

- [ ] **Step 4: Lint**

Run: `golangci-lint run`
Expected: 0 findings.

- [ ] **Step 5: Count tests**

Run: `go test -tags=integration -v ./tests/integration/api/... 2>&1 | grep -c "^=== RUN"`
Expected: between 75 and 115 (acceptance-criterion #3 of the design doc).

- [ ] **Step 6: Commit final state**

No-op commit only if something was fixed here. Otherwise skip.

- [ ] **Step 7: Verify acceptance criteria (design §Acceptance criteria) line by line**

Cross-reference each:

1. `go test -tags=integration ./tests/integration/api/...` PASS → step 3 above.
2. Behavior-spec exists → `ls docs/superpowers/specs/2026-04-18-C1-api-behavior-spec.md`.
3. ~95 tests → step 5 above.
4. 5 findings resolved → grep the spec §1 findings; verify each has a corresponding committed fix.
5. Clock/Random ports → `ls internal/port/clock.go internal/port/random.go internal/adapter/sysclock internal/adapter/sysrand`.
6. CI workflow → `ls .github/workflows/integration.yml`.
7. README updated → `grep -q 'Go API integration (C1)' README.md`.

---

# Self-Review

**Spec coverage (§1–§8 of behavior-spec → task mapping):**
- §1 error envelope → Task 13 (tests) + Task 14 (emitter)
- §2 auth model → Task 17 (session) + Task 21 (API key) + Task 25 (matrix)
- §3 tenant isolation → threaded through Tasks 18–21 (`_CrossTenant_Returns403` tests)
- §4 per-endpoint contract → Tasks 17 (auth), 18 (monitors + monitor-types), 19 (channels + channel-types), 20 (bindings), 21 (api-keys), 22 (status), 23 (health), 24 (webhooks), 15 (GET channel detail)
- §5 rate limits → **not explicitly tested in C1** — a deliberate scoping choice because rate-limit tests need clock advancement and touch the redis_rate internals. Recorded as a gap; promoted to C4 (security edge-cases). Noted in Task 29 step 7 as an acceptance-criterion nuance.
- §6 validation rules → Task 18 (monitor validation), Task 19 (channel validation)
- §7 webhook contract → Task 24
- §8 product decisions → implemented across Tasks 15 (GET channel), 16 (remove /logout), 19 (email→422), 20 (bind idempotent), 14 (error envelope spec §8.9 redaction integrated in Task 15)

**Missing from plan, added as follow-up:**
- Rate-limit tests (deferred to C4).
- Email-verification flow (spec says no email-verification; nothing to build).

**Placeholder scan:** No `TBD`, `TODO`, or "fill in later" remain in executable tasks. The harness/app.go in Task 4 has a bridge comment about "until Task 11/12", which is the intended sequencing, not a placeholder.

**Type consistency:** `harness.Session`, `harness.App`, `harness.Response`, `harness.ErrorBody` names are stable across all task references. `httperr.Write`, `httperr.WriteMalformedJSON`, etc. are stable from Task 14 onward.

---

**Plan complete and saved to `docs/superpowers/plans/2026-04-18-C1-api-contract-tests.md`.**

Two execution options:

**1. Subagent-Driven (recommended)** — dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — execute tasks in this session using executing-plans, batch execution with checkpoints.

Which approach?

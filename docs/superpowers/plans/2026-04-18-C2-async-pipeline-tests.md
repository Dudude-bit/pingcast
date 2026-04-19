# C2 — Async Pipeline Integration Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 12 black-box scenario tests covering the full monitoring
pipeline (monitor create → scheduler → worker → incident → notifier →
channel delivery), wired on top of the C1 harness.

**Architecture:** Extract composition roots for scheduler, worker, and
notifier to `internal/bootstrap/` mirroring the `NewApp` pattern from
C1. Add `harness.FakeTarget` (httptest), `harness.WaitFor` (bounded
polling), and `harness.AsyncStack` that starts all three services
in-process against the shared test containers. Expose
`LeaderScheduler.DispatchAll(ctx)` so tests trigger fan-out deterministically
without leader election or the 1s ticker.

**Tech Stack:** Go 1.26, Fiber (existing API), NATS JetStream, Postgres
via pgx, Redis via redis_rate/redsync, testcontainers-go, stretchr/testify.

**Spec:** `docs/superpowers/specs/2026-04-18-C2-async-pipeline-tests-design.md`

---

## File Structure

### Created

```
tests/integration/api/harness/
  fake_target.go       — httptest target with configurable responses
  waitfor.go           — bounded polling assertion

tests/integration/api/harness/
  async.go             — AsyncStack (scheduler + worker + notifier glue)

tests/integration/api/
  async_failure_test.go      — scenarios 1, 7, 11 (failure threshold + status shapes)
  async_recovery_test.go     — scenario 2 (recovery + incident close)
  async_pause_delete_test.go — scenarios 3, 4 (skip paused / deleted)
  async_timeout_test.go      — scenario 8 (slow target → timeout)
  async_checker_types_test.go — scenarios 9, 10 (TCP + DNS)
  async_delivery_test.go     — scenarios 5, 6 (telegram + webhook)
  async_dlq_test.go          — scenario 12 (send failure → failed_alerts)

internal/bootstrap/
  scheduler.go        — NewScheduler(deps) *Scheduler
  worker.go           — NewWorker(deps) *Worker
  notifier.go         — NewNotifier(deps) *Notifier
```

### Modified

- `internal/adapter/checker/leader_scheduler.go` — exports
  `DispatchAll(ctx)` that runs the same loop as `dispatchDueChecks`
  but unconditionally (ignoring lastTick). Used only by tests;
  production code path unchanged.
- `cmd/scheduler/main.go` — shrinks to `bootstrap.NewScheduler` +
  Start/Stop.
- `cmd/worker/main.go` — same, with `bootstrap.NewWorker`.
- `cmd/notifier/main.go` — same, with `bootstrap.NewNotifier`.

---

## Execution Conventions

- Every new test file starts with `//go:build integration` on line 1.
- Same `harness.New(t)` entry point as C1; `AsyncStack` is started
  per-test where needed.
- Each task is a complete cycle: write, run, verify, commit. Commit
  messages: `test(c2): <scenario>`, `feat(c2): <bootstrap-root>`,
  `refactor(c2): <extraction>`.
- Run commands from repo root, always with `-tags=integration`.

---

# Phase 0 — Harness helpers

## Task 1: FakeTarget

**File:** `tests/integration/api/harness/fake_target.go`

- [ ] **Step 1: Write fake_target.go**

```go
//go:build integration

package harness

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// FakeTarget is an httptest.Server whose response can be scripted:
// chain RespondWith/FailNext/Slow to build a queue of behaviours. The
// last response repeats once the queue drains.
type FakeTarget struct {
	Server *httptest.Server
	URL    string

	mu    sync.Mutex
	queue []response
	hits  int
}

type response struct {
	status int
	delay  time.Duration
}

// NewFakeTarget starts a target that returns 200 by default. Tests
// append behaviour via RespondWith / FailNext / Slow.
func NewFakeTarget(t *testing.T) *FakeTarget {
	t.Helper()
	ft := &FakeTarget{queue: []response{{status: 200}}}
	ft.Server = httptest.NewServer(http.HandlerFunc(ft.handle))
	ft.URL = ft.Server.URL
	t.Cleanup(ft.Server.Close)
	return ft
}

func (f *FakeTarget) handle(w http.ResponseWriter, _ *http.Request) {
	f.mu.Lock()
	f.hits++
	// Drain the queue, keeping the tail as the repeating default.
	var r response
	if len(f.queue) > 1 {
		r = f.queue[0]
		f.queue = f.queue[1:]
	} else {
		r = f.queue[0]
	}
	f.mu.Unlock()

	if r.delay > 0 {
		time.Sleep(r.delay)
	}
	w.WriteHeader(r.status)
}

func (f *FakeTarget) RespondWith(status int) *FakeTarget {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queue = append(f.queue, response{status: status})
	return f
}

// FailNext enqueues n 500 responses, after which the default 200
// (or whatever tail the queue has) resumes.
func (f *FakeTarget) FailNext(n int) *FakeTarget {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := 0; i < n; i++ {
		f.queue = append(f.queue, response{status: 500})
	}
	return f
}

func (f *FakeTarget) Slow(d time.Duration) *FakeTarget {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queue = append(f.queue, response{status: 200, delay: d})
	return f
}

func (f *FakeTarget) Hits() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hits
}
```

- [ ] **Step 2: Run build to verify it compiles**

```
go build -tags=integration ./tests/integration/api/...
```

Expected: exits 0.

- [ ] **Step 3: Commit**

```
git add tests/integration/api/harness/fake_target.go
git commit -m "test(c2): FakeTarget httptest helper with scripted responses"
```

---

## Task 2: WaitFor polling helper

**File:** `tests/integration/api/harness/waitfor.go`

- [ ] **Step 1: Write waitfor.go**

```go
//go:build integration

package harness

import (
	"testing"
	"time"
)

// WaitFor polls predicate every 100ms until it returns true or deadline
// elapses. Fails the test with desc on timeout. Used for async assertions
// like "incident opened" or "fake got 3 calls".
func WaitFor(t *testing.T, deadline time.Duration, predicate func() bool, desc string) {
	t.Helper()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	timer := time.NewTimer(deadline)
	defer timer.Stop()

	if predicate() {
		return
	}
	for {
		select {
		case <-tick.C:
			if predicate() {
				return
			}
		case <-timer.C:
			t.Fatalf("timed out after %s waiting for: %s", deadline, desc)
		}
	}
}
```

- [ ] **Step 2: Verify**

```
go build -tags=integration ./tests/integration/api/...
```

- [ ] **Step 3: Commit**

```
git add tests/integration/api/harness/waitfor.go
git commit -m "test(c2): WaitFor bounded-polling assertion helper"
```

---

# Phase 1 — Composition roots

## Task 3: Expose `DispatchAll` on LeaderScheduler

**File:** `internal/adapter/checker/leader_scheduler.go`

- [ ] **Step 1: Read current file**

Run: `grep -n "dispatchDueChecks\|DispatchAll" internal/adapter/checker/leader_scheduler.go`

- [ ] **Step 2: Add public test-friendly methods after dispatchDueChecks**

Edit the file to add:

```go
// DispatchAll publishes a check task for every non-paused monitor
// regardless of its lastTick. Tests use this to force a synchronous
// fan-out without running the ticker/leader loop. Production code
// paths use Run() → dispatchDueChecks() unchanged.
func (s *LeaderScheduler) DispatchAll(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sm := range s.monitors {
		if sm.monitor.IsPaused {
			continue
		}
		if err := s.publisher.Publish(ctx, sm.monitor.ID); err != nil {
			return err
		}
	}
	return nil
}

// MonitorCount returns the number of monitors currently registered.
// Tests use it to verify Add/Remove bookkeeping without reaching into
// the unexported map.
func (s *LeaderScheduler) MonitorCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.monitors)
}
```

- [ ] **Step 3: Run the package's existing tests**

```
go test -short ./internal/adapter/checker/...
```

Expected: PASS (no regressions — production Run() path untouched).

- [ ] **Step 4: Commit**

```
git add internal/adapter/checker/leader_scheduler.go
git commit -m "feat(c2): expose DispatchAll + MonitorCount on LeaderScheduler"
```

---

## Task 4: bootstrap.NewScheduler

**File:** `internal/bootstrap/scheduler.go`

- [ ] **Step 1: Write scheduler.go**

```go
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"
	goredis "github.com/redis/go-redis/v9"

	"github.com/kirillinakin/pingcast/internal/adapter/checker"
	natsadapter "github.com/kirillinakin/pingcast/internal/adapter/nats"
	"github.com/kirillinakin/pingcast/internal/adapter/postgres"
	redisadapter "github.com/kirillinakin/pingcast/internal/adapter/redis"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
	sqlcgen "github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

type SchedulerDeps struct {
	Pool   *pgxpool.Pool
	Redis  *goredis.Client
	JS     jetstream.JetStream
	Cipher port.Cipher

	RetentionDays int

	// SkipLeaderElection is TEST-ONLY. When true, Scheduler.Run doesn't
	// take the distributed lock — DispatchAll is expected to be called
	// directly. Cannot be set via env; only by the integration harness.
	SkipLeaderElection bool
}

type Scheduler struct {
	Leader *checker.LeaderScheduler

	monitorSub    *natsadapter.MonitorSubscriber
	cleanupCancel context.CancelFunc
	cleanupWg     sync.WaitGroup

	deps SchedulerDeps
}

func NewScheduler(deps SchedulerDeps) (*Scheduler, error) {
	queries := sqlcgen.New(deps.Pool)
	monitorRepo := postgres.NewMonitorRepo(deps.Pool, queries, deps.Cipher)
	checkResultRepo := postgres.NewCheckResultRepo(queries)

	rs := redisadapter.NewRedsync(deps.Redis)
	schedulerMutex := rs.NewMutex("lock:scheduler:leader", redsync.WithExpiry(10*time.Second))
	checkPub := natsadapter.NewCheckPublisher(deps.JS)

	hostname, _ := os.Hostname()
	instanceID := fmt.Sprintf("%s-%d", hostname, os.Getpid())

	var mutex port.DistributedMutex = schedulerMutex
	if deps.SkipLeaderElection {
		mutex = noopMutex{}
	}

	leader := checker.NewLeaderScheduler(mutex, checkPub, instanceID)

	s := &Scheduler{
		Leader:       leader,
		deps:         deps,
		_cleanupCtx:  deps.Pool, // silence "unused" during clean compile
	}
	_ = s // keep reference live
	_ = checkResultRepo

	// Load existing monitors up-front so tests don't observe a stale scheduler.
	ctx := context.Background()
	active, err := monitorRepo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("load active monitors: %w", err)
	}
	for i := range active {
		leader.Add(&active[i])
	}

	// Monitor change subscriber — same logic as cmd/scheduler/main.go
	s.monitorSub = natsadapter.NewMonitorSubscriber(deps.JS)
	if err := s.monitorSub.Subscribe(ctx, func(_ context.Context, ev port.MonitorChangedEvent) error {
		switch ev.Action {
		case domain.ActionCreate, domain.ActionUpdate, domain.ActionResume:
			leader.Add(&domain.Monitor{
				ID:                 ev.MonitorID,
				Name:               ev.Name,
				Type:               ev.Type,
				CheckConfig:        ev.CheckConfig,
				IntervalSeconds:    ev.IntervalSeconds,
				AlertAfterFailures: ev.AlertAfterFailures,
				IsPaused:           ev.IsPaused,
			})
		case domain.ActionDelete, domain.ActionPause:
			leader.Remove(ev.MonitorID)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("subscribe to monitor changes: %w", err)
	}

	// Cleanup goroutine — only started by Start(). We initialise
	// checkResultRepo closure captures here.
	s.cleanupFunc = func(ctx context.Context) {
		defer s.cleanupWg.Done()
		cleanupMutex := rs.NewMutex("lock:cleanup:retention", redsync.WithExpiry(1*time.Hour))
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := cleanupMutex.Lock(); err != nil {
					if errors.Is(err, redsync.ErrFailed) {
						continue
					}
					slog.Warn("cleanup lock failed", "error", err)
					continue
				}
				cutoff := time.Now().Add(-time.Duration(deps.RetentionDays) * 24 * time.Hour)
				deleted, err := checkResultRepo.DeleteOlderThan(ctx, cutoff)
				if err != nil {
					slog.Error("retention cleanup failed", "error", err)
				} else if deleted > 0 {
					slog.Info("retention cleanup", "deleted_rows", deleted)
				}
				rangeStart := time.Date(time.Now().Year(), time.Now().Month()+1, 1, 0, 0, 0, 0, time.UTC)
				rangeEnd := rangeStart.AddDate(0, 1, 0)
				safeName := pgx.Identifier{fmt.Sprintf("check_results_%d_%02d", rangeStart.Year(), rangeStart.Month())}.Sanitize()
				ddl := fmt.Sprintf(
					"CREATE TABLE IF NOT EXISTS %s PARTITION OF check_results FOR VALUES FROM ('%s') TO ('%s')",
					safeName, rangeStart.Format("2006-01-02"), rangeEnd.Format("2006-01-02"),
				)
				if _, err := deps.Pool.Exec(ctx, ddl); err != nil {
					slog.Error("partition creation failed", "error", err)
				}
				if _, err := cleanupMutex.Unlock(); err != nil {
					slog.Warn("cleanup unlock failed", "error", err)
				}
			}
		}
	}

	return s, nil
}

// Start runs the leader scheduler and the retention cleanup goroutine.
// Non-blocking: returns after goroutines launch.
func (s *Scheduler) Start(ctx context.Context) {
	cleanupCtx, cancel := context.WithCancel(ctx)
	s.cleanupCancel = cancel

	s.cleanupWg.Add(1)
	go s.cleanupFunc(cleanupCtx)

	go s.Leader.Run(ctx)
}

// Stop signals the leader + cleanup loops and waits for them to exit.
func (s *Scheduler) Stop(shutdownCtx context.Context) {
	if s.monitorSub != nil {
		s.monitorSub.Stop()
	}
	s.Leader.Stop()
	if s.cleanupCancel != nil {
		s.cleanupCancel()
	}
	doneCh := make(chan struct{})
	go func() {
		s.cleanupWg.Wait()
		close(doneCh)
	}()
	select {
	case <-doneCh:
	case <-shutdownCtx.Done():
	}
}

// Private plumbing — the "cleanupFunc" field simply captures the
// closure built in NewScheduler so Start() can spawn it cleanly.
// _cleanupCtx is a placeholder to keep the struct stable across refactors.
type schedulerInternals struct {
	cleanupFunc func(context.Context)
}

// noopMutex implements port.DistributedMutex with no-op Lock/Extend/Unlock.
// Used only in tests via SchedulerDeps.SkipLeaderElection.
type noopMutex struct{}

func (noopMutex) Lock() error          { return nil }
func (noopMutex) Extend() (bool, error) { return true, nil }
func (noopMutex) Unlock() (bool, error) { return true, nil }
```

> **Gap flagged during plan authoring:** The code above uses
> `s.cleanupFunc` and `s._cleanupCtx` fields on a single struct
> (`Scheduler`) but also defines a second struct `schedulerInternals`
> that isn't connected. This is a drafting mistake — before
> implementing, the agent must collapse this into a clean single
> `Scheduler` struct with the fields it actually uses. The intent is
> clear: one `Scheduler` struct with `Leader`, `monitorSub`,
> `cleanupFunc func(context.Context)`, `cleanupCancel
> context.CancelFunc`, `cleanupWg sync.WaitGroup`. Apply that shape
> when writing the file.

- [ ] **Step 2: Clean up the struct shape**

Replace the dual-struct pattern with a single `Scheduler` struct:

```go
type Scheduler struct {
	Leader        *checker.LeaderScheduler
	monitorSub    *natsadapter.MonitorSubscriber
	cleanupFunc   func(context.Context)
	cleanupCancel context.CancelFunc
	cleanupWg     sync.WaitGroup
}
```

Drop the `_cleanupCtx`, `deps`, and `schedulerInternals` remnants.
`NewScheduler` writes into `s.Leader`, `s.monitorSub`, and
`s.cleanupFunc`; that's all.

- [ ] **Step 3: Verify compile**

```
go build ./...
```

Expected: exits 0.

- [ ] **Step 4: Commit**

```
git add internal/bootstrap/scheduler.go
git commit -m "feat(c2): extract bootstrap.NewScheduler composition root"
```

---

## Task 5: Update cmd/scheduler/main.go to use bootstrap.NewScheduler

**File:** `cmd/scheduler/main.go`

- [ ] **Step 1: Replace file contents**

```go
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	natsadapter "github.com/kirillinakin/pingcast/internal/adapter/nats"
	redisadapter "github.com/kirillinakin/pingcast/internal/adapter/redis"
	"github.com/kirillinakin/pingcast/internal/bootstrap"
	"github.com/kirillinakin/pingcast/internal/config"
	"github.com/kirillinakin/pingcast/internal/database"
	"github.com/kirillinakin/pingcast/internal/observability"
	"github.com/kirillinakin/pingcast/internal/version"
)

func main() {
	inner := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(observability.NewTracingHandler(inner)))

	slog.Info("starting", "service", "scheduler", "version", version.Version, "commit", version.Commit)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.LoadChecker()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	devMode := os.Getenv("DEV_MODE") == "true"
	tracer := observability.NewSlowQueryTracer(100*time.Millisecond, devMode)
	//nolint:gosec // G115: MaxDBConns far below int32 max
	pool, err := database.Connect(ctx, cfg.DatabaseURL, int32(cfg.MaxDBConns), database.WithTracer(tracer))
	if err != nil {
		slog.Error("db connect", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb, err := redisadapter.Connect(ctx, cfg.RedisURL)
	if err != nil {
		slog.Error("redis connect", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()

	nc, err := natsadapter.Connect(cfg.NatsURL)
	if err != nil {
		slog.Error("nats connect", "error", err)
		os.Exit(1)
	}
	defer func() { _ = nc.Drain() }()

	js, err := jetstream.New(nc)
	if err != nil {
		slog.Error("jetstream", "error", err)
		os.Exit(1)
	}
	if err := natsadapter.SetupStreams(ctx, js); err != nil {
		slog.Error("streams", "error", err)
		os.Exit(1)
	}

	cipher, err := bootstrap.InitCipher(cfg.EncryptionConfig)
	if err != nil {
		slog.Error("cipher", "error", err)
		os.Exit(1)
	}

	sched, err := bootstrap.NewScheduler(bootstrap.SchedulerDeps{
		Pool:          pool,
		Redis:         rdb,
		JS:            js,
		Cipher:        cipher,
		RetentionDays: cfg.RetentionDays,
	})
	if err != nil {
		slog.Error("compose scheduler", "error", err)
		os.Exit(1)
	}

	sched.Start(ctx)
	slog.Info("scheduler started")

	<-ctx.Done()
	slog.Info("scheduler shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	sched.Stop(shutdownCtx)
	slog.Info("scheduler shutdown complete")
}
```

- [ ] **Step 2: Build**

```
go build ./cmd/scheduler/
```

Expected: exits 0.

- [ ] **Step 3: Commit**

```
git add cmd/scheduler/main.go
git commit -m "refactor(c2): cmd/scheduler uses bootstrap.NewScheduler"
```

---

## Task 6: bootstrap.NewWorker

**File:** `internal/bootstrap/worker.go`

- [ ] **Step 1: Write worker.go**

```go
package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/kirillinakin/pingcast/internal/adapter/checker"
	natsadapter "github.com/kirillinakin/pingcast/internal/adapter/nats"
	"github.com/kirillinakin/pingcast/internal/adapter/postgres"
	"github.com/kirillinakin/pingcast/internal/adapter/sysclock"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/observability"
	"github.com/kirillinakin/pingcast/internal/port"
	sqlcgen "github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

type WorkerDeps struct {
	Pool   *pgxpool.Pool
	JS     jetstream.JetStream
	Cipher port.Cipher

	DefaultTimeoutSecs int

	// Optional Clock override. If nil, uses sysclock.New().
	Clock port.Clock
}

type Worker struct {
	checkSub     *natsadapter.CheckSubscriber
	MonitoringSvc *app.MonitoringService
}

func NewWorker(deps WorkerDeps) (*Worker, error) {
	queries := sqlcgen.New(deps.Pool)

	userRepo := postgres.NewUserRepo(queries)
	monitorRepo := postgres.NewMonitorRepo(deps.Pool, queries, deps.Cipher)
	channelRepo := postgres.NewChannelRepo(deps.Pool, queries, deps.Cipher)
	checkResultRepo := postgres.NewCheckResultRepo(queries)
	incidentRepo := postgres.NewIncidentRepo(queries)
	uptimeRepo := postgres.NewUptimeRepo(queries)
	txm := postgres.NewTxManager(deps.Pool)

	alertPub := natsadapter.NewAlertPublisher(deps.JS)

	registry := checker.NewRegistry()
	timeout := time.Duration(deps.DefaultTimeoutSecs) * time.Second
	registry.Register(domain.MonitorHTTP, "HTTP", checker.NewHTTPCheckerWithTimeout(deps.DefaultTimeoutSecs))
	registry.Register(domain.MonitorTCP, "TCP", checker.NewTCPChecker(timeout))
	registry.Register(domain.MonitorDNS, "DNS", checker.NewDNSChecker())

	metrics := observability.NewMetrics()

	clock := deps.Clock
	if clock == nil {
		clock = sysclock.New()
	}

	svc := app.NewMonitoringService(
		monitorRepo, channelRepo, checkResultRepo, incidentRepo,
		userRepo, uptimeRepo, txm, alertPub, nil, registry, metrics, clock,
	)

	w := &Worker{
		MonitoringSvc: svc,
		checkSub:      natsadapter.NewCheckSubscriber(deps.JS),
	}

	handler := func(ctx context.Context, monitorID uuid.UUID) error {
		mon, err := monitorRepo.GetByID(ctx, monitorID)
		if err != nil {
			return fmt.Errorf("get monitor %s: %w", monitorID, err)
		}
		return svc.RunCheck(ctx, mon)
	}

	if err := w.checkSub.Subscribe(context.Background(), handler); err != nil {
		return nil, fmt.Errorf("subscribe checks: %w", err)
	}

	return w, nil
}

func (w *Worker) Stop() {
	w.checkSub.Stop()
}
```

- [ ] **Step 2: Build**

```
go build ./...
```

- [ ] **Step 3: Commit**

```
git add internal/bootstrap/worker.go
git commit -m "feat(c2): extract bootstrap.NewWorker composition root"
```

---

## Task 7: Update cmd/worker/main.go

**File:** `cmd/worker/main.go`

- [ ] **Step 1: Replace file**

```go
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	natsadapter "github.com/kirillinakin/pingcast/internal/adapter/nats"
	redisadapter "github.com/kirillinakin/pingcast/internal/adapter/redis"
	"github.com/kirillinakin/pingcast/internal/bootstrap"
	"github.com/kirillinakin/pingcast/internal/config"
	"github.com/kirillinakin/pingcast/internal/database"
	"github.com/kirillinakin/pingcast/internal/observability"
	"github.com/kirillinakin/pingcast/internal/version"
)

func main() {
	inner := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(observability.NewTracingHandler(inner)))

	slog.Info("starting", "service", "worker", "version", version.Version, "commit", version.Commit)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.LoadChecker()
	if err != nil {
		slog.Error("config", "error", err)
		os.Exit(1)
	}

	devMode := os.Getenv("DEV_MODE") == "true"
	tracer := observability.NewSlowQueryTracer(100*time.Millisecond, devMode)
	//nolint:gosec // G115
	pool, err := database.Connect(ctx, cfg.DatabaseURL, int32(cfg.MaxDBConns), database.WithTracer(tracer))
	if err != nil {
		slog.Error("db connect", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb, err := redisadapter.Connect(ctx, cfg.RedisURL)
	if err != nil {
		slog.Error("redis connect", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()
	_ = rdb // worker doesn't currently need Redis; keep connection for future hooks

	nc, err := natsadapter.Connect(cfg.NatsURL)
	if err != nil {
		slog.Error("nats connect", "error", err)
		os.Exit(1)
	}
	defer func() { _ = nc.Drain() }()

	js, err := jetstream.New(nc)
	if err != nil {
		slog.Error("jetstream", "error", err)
		os.Exit(1)
	}
	if err := natsadapter.SetupStreams(ctx, js); err != nil {
		slog.Error("streams", "error", err)
		os.Exit(1)
	}

	cipher, err := bootstrap.InitCipher(cfg.EncryptionConfig)
	if err != nil {
		slog.Error("cipher", "error", err)
		os.Exit(1)
	}

	w, err := bootstrap.NewWorker(bootstrap.WorkerDeps{
		Pool:               pool,
		JS:                 js,
		Cipher:             cipher,
		DefaultTimeoutSecs: cfg.DefaultTimeoutSecs,
	})
	if err != nil {
		slog.Error("compose worker", "error", err)
		os.Exit(1)
	}

	slog.Info("worker started")
	<-ctx.Done()
	slog.Info("worker shutting down")

	w.Stop()
	slog.Info("worker shutdown complete")
}
```

- [ ] **Step 2: Build**

```
go build ./cmd/worker/
```

- [ ] **Step 3: Commit**

```
git add cmd/worker/main.go
git commit -m "refactor(c2): cmd/worker uses bootstrap.NewWorker"
```

---

## Task 8: bootstrap.NewNotifier + ChannelSender seam

**Files:**
- Create: `internal/port/channel_sender.go`
- Create: `internal/bootstrap/notifier.go`
- Modify: `internal/app/alert.go`

- [ ] **Step 1: Define port.ChannelSender**

`internal/port/channel_sender.go`:

```go
package port

import (
	"context"

	"github.com/kirillinakin/pingcast/internal/domain"
)

// ChannelSender is the abstraction the notifier uses to deliver an
// alert to a single channel. Production impls wrap the Telegram/SMTP/
// Webhook factories; tests substitute in-memory fakes.
type ChannelSender interface {
	Send(ctx context.Context, ch *domain.NotificationChannel, event *domain.AlertEvent) error
}
```

- [ ] **Step 2: Audit how alert.Handle currently dispatches to senders**

Run:
```
grep -n "channelReg\|registry\|factory\|Send" internal/app/alert.go | head -20
```

The current `AlertService.Handle` uses `s.registry` (a
`port.ChannelRegistry`) and calls `factory.Send` internally. The
extraction is minimally invasive: expose a `Senders
map[domain.ChannelType]port.ChannelSender` on AlertService that, if
non-nil, takes precedence over the registry.

- [ ] **Step 3: Modify AlertService to accept optional per-type overrides**

Edit `internal/app/alert.go`. Add a field and an option:

```go
type AlertService struct {
	channels     port.ChannelRepo
	monitors     port.MonitorRepo
	registry     port.ChannelRegistry
	failedAlerts port.FailedAlertRepo
	metrics      port.Metrics

	// Optional per-channel-type sender overrides. When a type appears
	// here, Handle uses the override instead of asking the registry.
	// Tests use this to inject FakeTelegram/FakeSMTP/FakeWebhookSink.
	sendOverrides map[domain.ChannelType]port.ChannelSender
}

// WithSendOverrides is a constructor option used only by
// bootstrap.NewNotifier's test-facing entry. Prod notifier passes nil.
func (s *AlertService) WithSendOverrides(m map[domain.ChannelType]port.ChannelSender) *AlertService {
	s.sendOverrides = m
	return s
}
```

Then in the dispatch loop of `Handle`, replace the factory call:

```go
if override, ok := s.sendOverrides[ch.Type]; ok {
    if err := override.Send(ctx, &ch, event); err != nil {
        // existing failure-handling path
    }
    continue
}
// fall back to existing registry path
```

Full edit location: the loop body inside `Handle` where per-channel
delivery occurs. Keep all existing metrics / DLQ / retry logic intact.

- [ ] **Step 4: Write bootstrap/notifier.go**

```go
package bootstrap

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/kirillinakin/pingcast/internal/adapter/channel"
	natsadapter "github.com/kirillinakin/pingcast/internal/adapter/nats"
	"github.com/kirillinakin/pingcast/internal/adapter/postgres"
	smtpadapter "github.com/kirillinakin/pingcast/internal/adapter/smtp"
	"github.com/kirillinakin/pingcast/internal/adapter/telegram"
	"github.com/kirillinakin/pingcast/internal/adapter/webhook"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/observability"
	"github.com/kirillinakin/pingcast/internal/port"
	sqlcgen "github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

type NotifierDeps struct {
	Pool   *pgxpool.Pool
	NATS   *nats.Conn
	JS     jetstream.JetStream
	Cipher port.Cipher

	TelegramToken string
	SMTPHost      string
	SMTPPort      int
	SMTPUser      string
	SMTPPass      string
	SMTPFrom      string

	// SendOverrides — test-only. When non-nil, AlertService routes
	// delivery through these instead of the Telegram/SMTP/Webhook
	// factories. Production passes nil.
	SendOverrides map[domain.ChannelType]port.ChannelSender
}

type Notifier struct {
	alertSub    *natsadapter.AlertSubscriber
	dlqConsumer *natsadapter.DLQConsumer
	AlertSvc    *app.AlertService
}

func NewNotifier(deps NotifierDeps) (*Notifier, error) {
	queries := sqlcgen.New(deps.Pool)
	channelRepo := postgres.NewChannelRepo(deps.Pool, queries, deps.Cipher)
	monitorRepo := postgres.NewMonitorRepo(deps.Pool, queries, deps.Cipher)
	failedAlertRepo := postgres.NewFailedAlertRepo(queries)

	// Registry — still required for backwards-compat validation paths
	// (e.g. the Handle fallback if SendOverrides lacks a type).
	reg := channel.NewRegistry()
	if deps.TelegramToken != "" {
		reg.Register(domain.ChannelTelegram, "Telegram", telegram.NewFactory(deps.TelegramToken))
	}
	if deps.SMTPHost != "" {
		reg.Register(domain.ChannelEmail, "Email",
			smtpadapter.NewFactory(deps.SMTPHost, deps.SMTPPort, deps.SMTPUser, deps.SMTPPass, deps.SMTPFrom))
	}
	reg.Register(domain.ChannelWebhook, "Webhook", webhook.NewFactory())

	metrics := observability.NewMetrics()
	svc := app.NewAlertService(channelRepo, monitorRepo, reg, failedAlertRepo, metrics)
	if deps.SendOverrides != nil {
		svc = svc.WithSendOverrides(deps.SendOverrides)
	}

	n := &Notifier{AlertSvc: svc}

	n.alertSub = natsadapter.NewAlertSubscriber(deps.JS)
	if err := n.alertSub.Subscribe(context.Background(), func(ctx context.Context, event *domain.AlertEvent) error {
		return svc.Handle(ctx, event)
	}); err != nil {
		return nil, fmt.Errorf("alert subscribe: %w", err)
	}

	n.dlqConsumer = natsadapter.NewDLQConsumer(deps.NATS)
	if err := n.dlqConsumer.Subscribe(context.Background(),
		func(ctx context.Context, streamName, consumerName string, seq uint64, data []byte) error {
			msg := fmt.Sprintf("max deliveries exhausted: stream=%s consumer=%s seq=%d", streamName, consumerName, seq)
			return failedAlertRepo.Create(ctx, data, msg, nil)
		}); err != nil {
		return nil, fmt.Errorf("dlq subscribe: %w", err)
	}

	return n, nil
}

func (n *Notifier) Stop() {
	n.alertSub.Stop()
	n.dlqConsumer.Stop()
}
```

- [ ] **Step 5: Build + unit tests**

```
go build ./...
go test -short ./internal/app/...
```

Both should pass — `WithSendOverrides(nil)` has no effect in prod unit tests.

- [ ] **Step 6: Commit**

```
git add internal/port/channel_sender.go internal/bootstrap/notifier.go internal/app/alert.go
git commit -m "feat(c2): ChannelSender port + bootstrap.NewNotifier composition root"
```

---

## Task 9: Update cmd/notifier/main.go

**File:** `cmd/notifier/main.go`

- [ ] **Step 1: Replace file**

```go
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	natsadapter "github.com/kirillinakin/pingcast/internal/adapter/nats"
	"github.com/kirillinakin/pingcast/internal/bootstrap"
	"github.com/kirillinakin/pingcast/internal/config"
	"github.com/kirillinakin/pingcast/internal/database"
	"github.com/kirillinakin/pingcast/internal/observability"
	"github.com/kirillinakin/pingcast/internal/version"
)

func main() {
	inner := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(observability.NewTracingHandler(inner)))

	slog.Info("starting", "service", "notifier", "version", version.Version, "commit", version.Commit)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.LoadNotifier()
	if err != nil {
		slog.Error("config", "error", err)
		os.Exit(1)
	}

	devMode := os.Getenv("DEV_MODE") == "true"
	tracer := observability.NewSlowQueryTracer(100*time.Millisecond, devMode)
	//nolint:gosec // G115
	pool, err := database.Connect(ctx, cfg.DatabaseURL, int32(cfg.MaxDBConns), database.WithTracer(tracer))
	if err != nil {
		slog.Error("db connect", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	nc, err := natsadapter.Connect(cfg.NatsURL)
	if err != nil {
		slog.Error("nats connect", "error", err)
		os.Exit(1)
	}
	defer func() { _ = nc.Drain() }()

	js, err := jetstream.New(nc)
	if err != nil {
		slog.Error("jetstream", "error", err)
		os.Exit(1)
	}
	if err := natsadapter.SetupStreams(ctx, js); err != nil {
		slog.Error("streams", "error", err)
		os.Exit(1)
	}

	cipher, err := bootstrap.InitCipher(cfg.EncryptionConfig)
	if err != nil {
		slog.Error("cipher", "error", err)
		os.Exit(1)
	}

	n, err := bootstrap.NewNotifier(bootstrap.NotifierDeps{
		Pool:          pool,
		NATS:          nc,
		JS:            js,
		Cipher:        cipher,
		TelegramToken: cfg.TelegramToken,
		SMTPHost:      cfg.SMTPHost,
		SMTPPort:      cfg.SMTPPort,
		SMTPUser:      cfg.SMTPUser,
		SMTPPass:      cfg.SMTPPass,
		SMTPFrom:      cfg.SMTPFrom,
	})
	if err != nil {
		slog.Error("compose notifier", "error", err)
		os.Exit(1)
	}

	slog.Info("notifier started")
	<-ctx.Done()
	slog.Info("notifier shutting down")

	n.Stop()
	slog.Info("notifier shutdown complete")
}
```

- [ ] **Step 2: Full build**

```
go build ./...
```

Expected: all 4 cmd binaries plus all packages compile cleanly.

- [ ] **Step 3: Commit**

```
git add cmd/notifier/main.go
git commit -m "refactor(c2): cmd/notifier uses bootstrap.NewNotifier"
```

---

# Phase 2 — AsyncStack

## Task 10: harness.AsyncStack

**File:** `tests/integration/api/harness/async.go`

- [ ] **Step 1: Write async.go**

```go
//go:build integration

package harness

import (
	"context"
	"sync"
	"testing"

	"github.com/kirillinakin/pingcast/internal/bootstrap"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

// AsyncStack glues the three background services (scheduler, worker,
// notifier) to the shared test containers and fakes. Start() launches
// the worker + notifier subscriptions and the scheduler's monitor-change
// subscriber; Stop() drains them all. Tests drive fan-out synchronously
// via Scheduler.Leader.DispatchAll.
type AsyncStack struct {
	Scheduler *bootstrap.Scheduler
	Worker    *bootstrap.Worker
	Notifier  *bootstrap.Notifier

	stopMu sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
}

// StartAsyncStack composes scheduler+worker+notifier in-process wired
// to App's containers. Notifier delivery goes through the harness
// fakes (SMTP, Telegram, Webhook).
func (a *App) StartAsyncStack(t *testing.T) *AsyncStack {
	t.Helper()

	senders := map[domain.ChannelType]port.ChannelSender{
		domain.ChannelTelegram: a.Telegram.AsSender(),
		domain.ChannelEmail:    a.SMTP.AsSender(),
		domain.ChannelWebhook:  a.Webhook.AsSender(),
	}

	sched, err := bootstrap.NewScheduler(bootstrap.SchedulerDeps{
		Pool:               a.Pool,
		Redis:              a.Redis,
		JS:                 a.JS(),
		Cipher:             a.App.Cipher, // exposed by bootstrap.App in Task 11 note below
		RetentionDays:      30,
		SkipLeaderElection: true,
	})
	if err != nil {
		t.Fatalf("scheduler: %v", err)
	}

	wrk, err := bootstrap.NewWorker(bootstrap.WorkerDeps{
		Pool:               a.Pool,
		JS:                 a.JS(),
		Cipher:             a.App.Cipher,
		DefaultTimeoutSecs: 10,
		Clock:              a.Clock,
	})
	if err != nil {
		t.Fatalf("worker: %v", err)
	}

	not, err := bootstrap.NewNotifier(bootstrap.NotifierDeps{
		Pool:          a.Pool,
		NATS:          a.NATS,
		JS:            a.JS(),
		Cipher:        a.App.Cipher,
		SendOverrides: senders,
	})
	if err != nil {
		t.Fatalf("notifier: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	stack := &AsyncStack{
		Scheduler: sched,
		Worker:    wrk,
		Notifier:  not,
		ctx:       ctx,
		cancel:    cancel,
	}
	// Start only the monitor-change + cleanup subscriber on the scheduler.
	// Tests call DispatchAll directly; the ticker-driven Run is not needed.
	sched.Start(ctx)

	t.Cleanup(stack.Stop)
	return stack
}

func (s *AsyncStack) Stop() {
	s.stopMu.Lock()
	defer s.stopMu.Unlock()
	if s.cancel == nil {
		return
	}
	s.cancel()
	s.cancel = nil

	shutdown, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Scheduler.Stop(shutdown)
	s.Worker.Stop()
	s.Notifier.Stop()
}
```

- [ ] **Step 2: Expose JS() and Cipher on harness.App**

The current `harness.App` stores Pool/Redis/NATS/Clock/Rand/fakes.
It needs to expose the JetStream handle and the cipher. Edit
`tests/integration/api/harness/app.go`:

- Add `JS jetstream.JetStream` field, initialise it from the same
  `jetstream.New(nc)` call that `NewApp` already makes.
- Expose it via `func (a *App) JS() jetstream.JetStream`.
- `bootstrap.App` (returned by `bootstrap.NewApp`) doesn't currently
  expose `Cipher`; add `Cipher port.Cipher` to `bootstrap.App` struct
  and populate from `deps.Cipher`. Tests reference it as
  `a.App.Cipher`.

- [ ] **Step 3: Add AsSender() methods on the fakes**

Edit `tests/integration/api/harness/fakes.go`. For each of
`FakeSMTP`, `FakeTelegram`, `FakeWebhookSink`, add:

```go
// AsSender adapts the fake to port.ChannelSender so it can be
// injected through bootstrap.NotifierDeps.SendOverrides.
func (s *FakeSMTP) AsSender() port.ChannelSender { return smtpSender{s} }
type smtpSender struct{ *FakeSMTP }
func (a smtpSender) Send(ctx context.Context, ch *domain.NotificationChannel, event *domain.AlertEvent) error {
    return a.FakeSMTP.Send(event.To(), event.Subject(), event.Body())
}
```

Repeat analogously for `FakeTelegram` (extract chat_id from the channel
config, push message to `f.calls`) and `FakeWebhookSink` (extract URL
from config, append to `s.hits`).

Exact signatures depend on what `domain.AlertEvent` exposes; the agent
must read `internal/domain/alert.go` and write the adapter so the
extracted fields match what the fake's existing AssertX methods use.

- [ ] **Step 4: Build**

```
go build -tags=integration ./tests/integration/api/...
```

Expected: 0 errors.

- [ ] **Step 5: Commit**

```
git add tests/integration/api/harness/async.go tests/integration/api/harness/app.go tests/integration/api/harness/fakes.go internal/bootstrap/app.go
git commit -m "test(c2): AsyncStack + fakes.AsSender() adapters"
```

---

# Phase 3 — Scenario tests

Each scenario file below follows the same pattern: build a monitor via
the API, drive the scheduler, verify DB/fake state via WaitFor. Tests
run with `-tags=integration` through the shared TestMain.

## Task 11: Failure detection (scenarios 1, 7, 11)

**File:** `tests/integration/api/async_failure_test.go`

- [ ] **Step 1: Write the suite**

```go
//go:build integration

package api

import (
	"context"
	"testing"
	"time"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestAsyncFailure_ThreeConsecutive500s_OpensIncident(t *testing.T) {
	h := harness.New(t)
	stack := h.App.StartAsyncStack(t)

	target := harness.NewFakeTarget(t).FailNext(5)

	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/monitors", map[string]any{
		"name":                 "api",
		"type":                 "http",
		"check_config":         map[string]any{"url": target.URL, "method": "GET"},
		"interval_seconds":     300,
		"alert_after_failures": 3,
	})
	harness.AssertStatus(t, cr, 201)
	var m struct{ ID string }
	cr.JSON(t, &m)

	// Three synchronous fan-outs → three worker-executed failures → 1 incident.
	for i := 0; i < 3; i++ {
		if err := stack.Scheduler.Leader.DispatchAll(context.Background()); err != nil {
			t.Fatalf("dispatch: %v", err)
		}
		harness.WaitFor(t, 5*time.Second, func() bool {
			return target.Hits() >= i+1
		}, "target hit count")
	}

	harness.WaitFor(t, 5*time.Second, func() bool {
		var open bool
		_ = h.App.Pool.QueryRow(context.Background(),
			`SELECT EXISTS(SELECT 1 FROM incidents WHERE monitor_id=$1 AND resolved_at IS NULL)`,
			m.ID).Scan(&open)
		return open
	}, "incident opened")
}

func TestAsyncFailure_AlertAfterOne_FiresOnFirstFailure(t *testing.T) {
	h := harness.New(t)
	stack := h.App.StartAsyncStack(t)

	target := harness.NewFakeTarget(t).FailNext(1)
	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/monitors", map[string]any{
		"name":                 "single-fail",
		"type":                 "http",
		"check_config":         map[string]any{"url": target.URL, "method": "GET"},
		"interval_seconds":     300,
		"alert_after_failures": 1,
	})
	var m struct{ ID string }
	cr.JSON(t, &m)

	if err := stack.Scheduler.Leader.DispatchAll(context.Background()); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	harness.WaitFor(t, 5*time.Second, func() bool {
		var open bool
		_ = h.App.Pool.QueryRow(context.Background(),
			`SELECT EXISTS(SELECT 1 FROM incidents WHERE monitor_id=$1 AND resolved_at IS NULL)`,
			m.ID).Scan(&open)
		return open
	}, "incident opened on first failure")
}

func TestAsyncFailure_5xxClassifiedAsFailure(t *testing.T) {
	h := harness.New(t)
	stack := h.App.StartAsyncStack(t)

	target := harness.NewFakeTarget(t).RespondWith(502)
	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/monitors", map[string]any{
		"name":             "5xx",
		"type":             "http",
		"check_config":     map[string]any{"url": target.URL, "method": "GET"},
		"interval_seconds": 300,
	})
	var m struct{ ID string }
	cr.JSON(t, &m)

	_ = stack.Scheduler.Leader.DispatchAll(context.Background())

	harness.WaitFor(t, 5*time.Second, func() bool {
		var status string
		_ = h.App.Pool.QueryRow(context.Background(),
			`SELECT status FROM check_results WHERE monitor_id=$1 ORDER BY checked_at DESC LIMIT 1`,
			m.ID).Scan(&status)
		return status == "down" || status == "fail"
	}, "check_result recorded with failure status")
}
```

- [ ] **Step 2: Run**

```
go test -tags=integration ./tests/integration/api/... -run TestAsyncFailure -v -timeout 5m
```

Expected: all three PASS. Any red test → fix underlying service code
(monitoring.go, worker, alert.go) per the spec; re-run until green.

- [ ] **Step 3: Commit**

```
git add tests/integration/api/async_failure_test.go
git commit -m "test(c2): failure-detection scenarios (threshold, single-fail, 5xx)"
```

---

## Task 12: Recovery (scenario 2)

**File:** `tests/integration/api/async_recovery_test.go`

- [ ] **Step 1: Write the suite**

```go
//go:build integration

package api

import (
	"context"
	"testing"
	"time"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestAsyncRecovery_AfterFailuresRecovers_ClosesIncident(t *testing.T) {
	h := harness.New(t)
	stack := h.App.StartAsyncStack(t)

	target := harness.NewFakeTarget(t).FailNext(3) // 3 fails then 200

	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/monitors", map[string]any{
		"name":                 "recovering",
		"type":                 "http",
		"check_config":         map[string]any{"url": target.URL, "method": "GET"},
		"interval_seconds":     300,
		"alert_after_failures": 3,
	})
	var m struct{ ID string }
	cr.JSON(t, &m)

	// 3 failures → incident opens
	for i := 0; i < 3; i++ {
		_ = stack.Scheduler.Leader.DispatchAll(context.Background())
		harness.WaitFor(t, 5*time.Second, func() bool { return target.Hits() >= i+1 }, "target hit")
	}
	harness.WaitFor(t, 5*time.Second, func() bool {
		var open bool
		_ = h.App.Pool.QueryRow(context.Background(),
			`SELECT EXISTS(SELECT 1 FROM incidents WHERE monitor_id=$1 AND resolved_at IS NULL)`, m.ID).Scan(&open)
		return open
	}, "incident opened")

	// Recovery tick — target now returns 200
	_ = stack.Scheduler.Leader.DispatchAll(context.Background())
	harness.WaitFor(t, 5*time.Second, func() bool { return target.Hits() >= 4 }, "recovery hit")

	harness.WaitFor(t, 5*time.Second, func() bool {
		var closed bool
		_ = h.App.Pool.QueryRow(context.Background(),
			`SELECT EXISTS(SELECT 1 FROM incidents WHERE monitor_id=$1 AND resolved_at IS NOT NULL)`, m.ID).Scan(&closed)
		return closed
	}, "incident resolved")
}
```

- [ ] **Step 2: Run + commit**

```
go test -tags=integration ./tests/integration/api/... -run TestAsyncRecovery -v -timeout 5m
git add tests/integration/api/async_recovery_test.go
git commit -m "test(c2): recovery scenario — incident closes after 200"
```

---

## Task 13: Pause / delete (scenarios 3, 4)

**File:** `tests/integration/api/async_pause_delete_test.go`

- [ ] **Step 1: Write**

```go
//go:build integration

package api

import (
	"context"
	"testing"
	"time"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestAsyncScheduler_SkipsPausedMonitor(t *testing.T) {
	h := harness.New(t)
	stack := h.App.StartAsyncStack(t)

	target := harness.NewFakeTarget(t)
	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/monitors", map[string]any{
		"name": "will-be-paused", "type": "http",
		"check_config": map[string]any{"url": target.URL, "method": "GET"},
		"interval_seconds": 300,
	})
	var m struct{ ID string }
	cr.JSON(t, &m)

	// Pause via API — triggers monitor.changed event → scheduler.Remove (or IsPaused=true)
	p := s.POST(t, "/api/monitors/"+m.ID+"/pause", nil)
	harness.AssertStatus(t, p, 200)

	// Let the monitor-change subscriber propagate.
	harness.WaitFor(t, 2*time.Second, func() bool {
		return stack.Scheduler.Leader.MonitorCount() == 0 ||
			monitorIsPaused(t, h, m.ID)
	}, "scheduler observed pause")

	_ = stack.Scheduler.Leader.DispatchAll(context.Background())
	time.Sleep(500 * time.Millisecond)
	if target.Hits() != 0 {
		t.Fatalf("paused monitor was dispatched: %d hits", target.Hits())
	}
}

func TestAsyncScheduler_SkipsDeletedMonitor(t *testing.T) {
	h := harness.New(t)
	stack := h.App.StartAsyncStack(t)

	target := harness.NewFakeTarget(t)
	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/monitors", map[string]any{
		"name": "will-delete", "type": "http",
		"check_config": map[string]any{"url": target.URL, "method": "GET"},
		"interval_seconds": 300,
	})
	var m struct{ ID string }
	cr.JSON(t, &m)

	d := s.DELETE(t, "/api/monitors/"+m.ID)
	harness.AssertStatus(t, d, 204)

	harness.WaitFor(t, 2*time.Second, func() bool {
		return stack.Scheduler.Leader.MonitorCount() == 0
	}, "scheduler observed delete")

	_ = stack.Scheduler.Leader.DispatchAll(context.Background())
	time.Sleep(500 * time.Millisecond)
	if target.Hits() != 0 {
		t.Fatalf("deleted monitor was dispatched: %d hits", target.Hits())
	}
}

// monitorIsPaused is a small helper local to this suite.
func monitorIsPaused(t *testing.T, h *harness.Harness, id string) bool {
	t.Helper()
	var paused bool
	_ = h.App.Pool.QueryRow(context.Background(),
		`SELECT is_paused FROM monitors WHERE id=$1`, id).Scan(&paused)
	return paused
}
```

- [ ] **Step 2: Run + commit**

```
go test -tags=integration ./tests/integration/api/... -run TestAsyncScheduler -v -timeout 5m
git add tests/integration/api/async_pause_delete_test.go
git commit -m "test(c2): scheduler skips paused and deleted monitors"
```

---

## Task 14: Timeout (scenario 8)

**File:** `tests/integration/api/async_timeout_test.go`

- [ ] **Step 1: Write**

```go
//go:build integration

package api

import (
	"context"
	"testing"
	"time"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestAsyncTimeout_SlowTarget_ClassifiedAsTimeout(t *testing.T) {
	h := harness.New(t)
	stack := h.App.StartAsyncStack(t)

	target := harness.NewFakeTarget(t).Slow(3 * time.Second).Slow(3 * time.Second)

	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/monitors", map[string]any{
		"name":             "slow",
		"type":             "http",
		"check_config":     map[string]any{"url": target.URL, "method": "GET"},
		"interval_seconds": 300,
		"timeout_seconds":  1,
	})
	var m struct{ ID string }
	cr.JSON(t, &m)

	_ = stack.Scheduler.Leader.DispatchAll(context.Background())

	harness.WaitFor(t, 10*time.Second, func() bool {
		var reason string
		_ = h.App.Pool.QueryRow(context.Background(),
			`SELECT COALESCE(error_reason, '') FROM check_results
			 WHERE monitor_id=$1 ORDER BY checked_at DESC LIMIT 1`,
			m.ID).Scan(&reason)
		return reason == "timeout"
	}, "check_result classified as timeout")
}
```

- [ ] **Step 2: Run + commit**

```
go test -tags=integration ./tests/integration/api/... -run TestAsyncTimeout -v -timeout 5m
git add tests/integration/api/async_timeout_test.go
git commit -m "test(c2): slow target → timeout classification"
```

---

## Task 15: TCP + DNS (scenarios 9, 10)

**File:** `tests/integration/api/async_checker_types_test.go`

- [ ] **Step 1: Write**

```go
//go:build integration

package api

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestAsyncTCP_ClosedPort_Fails(t *testing.T) {
	h := harness.New(t)
	stack := h.App.StartAsyncStack(t)

	// Find a free-but-unbound port
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close() // close so the port is now unbound

	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/monitors", map[string]any{
		"name":             "closed-tcp",
		"type":             "tcp",
		"check_config":     map[string]any{"host": "127.0.0.1", "port": port},
		"interval_seconds": 300,
	})
	var m struct{ ID string }
	cr.JSON(t, &m)

	_ = stack.Scheduler.Leader.DispatchAll(context.Background())

	harness.WaitFor(t, 5*time.Second, func() bool {
		var status string
		_ = h.App.Pool.QueryRow(context.Background(),
			`SELECT status FROM check_results WHERE monitor_id=$1 ORDER BY checked_at DESC LIMIT 1`,
			m.ID).Scan(&status)
		return status == "down" || status == "fail"
	}, "tcp check failed")
	_ = strconv.Itoa // keep import live
}

func TestAsyncDNS_NonresolvingHost_Fails(t *testing.T) {
	h := harness.New(t)
	stack := h.App.StartAsyncStack(t)

	s := h.RegisterAndLogin(t, "", "")
	cr := s.POST(t, "/api/monitors", map[string]any{
		"name":             "bad-dns",
		"type":             "dns",
		"check_config":     map[string]any{"host": "nonexistent.invalid"},
		"interval_seconds": 300,
	})
	var m struct{ ID string }
	cr.JSON(t, &m)

	_ = stack.Scheduler.Leader.DispatchAll(context.Background())

	harness.WaitFor(t, 10*time.Second, func() bool {
		var status string
		_ = h.App.Pool.QueryRow(context.Background(),
			`SELECT status FROM check_results WHERE monitor_id=$1 ORDER BY checked_at DESC LIMIT 1`,
			m.ID).Scan(&status)
		return status == "down" || status == "fail"
	}, "dns check failed")
}
```

- [ ] **Step 2: Run + commit**

```
go test -tags=integration ./tests/integration/api/... -run 'TestAsyncTCP|TestAsyncDNS' -v -timeout 5m
git add tests/integration/api/async_checker_types_test.go
git commit -m "test(c2): TCP closed-port + DNS non-resolving checkers"
```

---

## Task 16: Channel delivery (scenarios 5, 6)

**File:** `tests/integration/api/async_delivery_test.go`

- [ ] **Step 1: Write**

```go
//go:build integration

package api

import (
	"context"
	"testing"
	"time"

	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestAsyncDelivery_Telegram_ReceivesAlertOnIncident(t *testing.T) {
	h := harness.New(t)
	stack := h.App.StartAsyncStack(t)

	target := harness.NewFakeTarget(t).FailNext(5)
	s := h.RegisterAndLogin(t, "", "")

	// Create monitor
	mr := s.POST(t, "/api/monitors", map[string]any{
		"name":                 "tg-alert",
		"type":                 "http",
		"check_config":         map[string]any{"url": target.URL, "method": "GET"},
		"interval_seconds":     300,
		"alert_after_failures": 1,
	})
	var m struct{ ID string }
	mr.JSON(t, &m)

	// Create Telegram channel
	cr := s.POST(t, "/api/channels", map[string]any{
		"name":   "ops-tg",
		"type":   "telegram",
		"config": map[string]any{"bot_token": "12345:ABCDEFG", "chat_id": 777},
	})
	var ch struct{ ID string }
	cr.JSON(t, &ch)

	// Bind
	br := s.POST(t, "/api/monitors/"+m.ID+"/channels",
		map[string]any{"channel_id": ch.ID})
	if br.Status != 200 && br.Status != 201 {
		t.Fatalf("bind: %d", br.Status)
	}

	_ = stack.Scheduler.Leader.DispatchAll(context.Background())

	harness.WaitFor(t, 10*time.Second, func() bool {
		return len(h.App.Telegram.Calls()) > 0
	}, "telegram received alert")
}

func TestAsyncDelivery_Webhook_ReceivesPayload(t *testing.T) {
	h := harness.New(t)
	stack := h.App.StartAsyncStack(t)

	target := harness.NewFakeTarget(t).FailNext(5)
	s := h.RegisterAndLogin(t, "", "")

	mr := s.POST(t, "/api/monitors", map[string]any{
		"name":                 "wh-alert",
		"type":                 "http",
		"check_config":         map[string]any{"url": target.URL, "method": "GET"},
		"interval_seconds":     300,
		"alert_after_failures": 1,
	})
	var m struct{ ID string }
	mr.JSON(t, &m)

	cr := s.POST(t, "/api/channels", map[string]any{
		"name":   "ops-wh",
		"type":   "webhook",
		"config": map[string]any{"url": h.App.Webhook.URL()},
	})
	var ch struct{ ID string }
	cr.JSON(t, &ch)

	s.POST(t, "/api/monitors/"+m.ID+"/channels",
		map[string]any{"channel_id": ch.ID})

	_ = stack.Scheduler.Leader.DispatchAll(context.Background())

	harness.WaitFor(t, 10*time.Second, func() bool {
		return len(h.App.Webhook.Hits()) > 0
	}, "webhook sink received alert")
}
```

- [ ] **Step 2: Run + commit**

```
go test -tags=integration ./tests/integration/api/... -run TestAsyncDelivery -v -timeout 5m
git add tests/integration/api/async_delivery_test.go
git commit -m "test(c2): telegram + webhook alert delivery via fakes"
```

---

## Task 17: DLQ (scenario 12)

**File:** `tests/integration/api/async_dlq_test.go`

- [ ] **Step 1: Write**

```go
//go:build integration

package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/tests/integration/api/harness"
)

func TestAsyncDLQ_SenderAlwaysFails_WritesToFailedAlerts(t *testing.T) {
	h := harness.New(t)

	// Install a failing webhook sender override on the stack.
	h.App.Webhook.FailAll(errors.New("boom"))

	stack := h.App.StartAsyncStack(t)

	target := harness.NewFakeTarget(t).FailNext(5)
	s := h.RegisterAndLogin(t, "", "")
	mr := s.POST(t, "/api/monitors", map[string]any{
		"name":                 "dlq",
		"type":                 "http",
		"check_config":         map[string]any{"url": target.URL, "method": "GET"},
		"interval_seconds":     300,
		"alert_after_failures": 1,
	})
	var m struct{ ID string }
	mr.JSON(t, &m)

	cr := s.POST(t, "/api/channels", map[string]any{
		"name":   "doom-hook",
		"type":   "webhook",
		"config": map[string]any{"url": h.App.Webhook.URL()},
	})
	var ch struct{ ID string }
	cr.JSON(t, &ch)
	s.POST(t, "/api/monitors/"+m.ID+"/channels",
		map[string]any{"channel_id": ch.ID})

	_ = stack.Scheduler.Leader.DispatchAll(context.Background())

	harness.WaitFor(t, 15*time.Second, func() bool {
		var count int
		_ = h.App.Pool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM failed_alerts`).Scan(&count)
		return count > 0
	}, "failed_alerts row written")
	_ = domain.ChannelWebhook
	_ = stack
}
```

- [ ] **Step 2: Add FailAll to FakeWebhookSink**

Edit `tests/integration/api/harness/fakes.go` — add:

```go
// FailAll makes AsSender return err on every delivery until cleared
// with FailAll(nil). Hits are still recorded.
func (s *FakeWebhookSink) FailAll(err error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.failErr = err
}
```

And change the adapter inside the `AsSender()` chain to return
`s.failErr` when set (after recording the hit).

- [ ] **Step 3: Run + commit**

```
go test -tags=integration ./tests/integration/api/... -run TestAsyncDLQ -v -timeout 5m
git add tests/integration/api/
git commit -m "test(c2): DLQ scenario — repeated send failures persist to failed_alerts"
```

---

## Task 18: Final verification

- [ ] **Step 1: Full build**

```
go build ./...
```

- [ ] **Step 2: Full unit suite**

```
make test-short
```

- [ ] **Step 3: Full integration suite**

```
make test-integration
```

Expected: combined C1 + C2 = ~93 tests, all green.

- [ ] **Step 4: Lint**

```
make lint
```

- [ ] **Step 5: Test count check**

```
go test -tags=integration -v ./tests/integration/api/... 2>&1 | grep -c "^--- PASS"
```

Expected: 92-95.

- [ ] **Step 6: Push**

```
git push origin main
```

Dokploy auto-deploys. Sanity check prod still green:

```
curl -s -o /dev/null -w "%{http_code}\n" https://pingcast.kirillin.tech/health
curl -s -o /dev/null -w "%{http_code}\n" https://pingcast.kirillin.tech/api/monitor-types
```

Expected: 200, 401 (canonical envelope).

---

# Self-Review

**Spec coverage:**
- §1 purpose — covered by Phase 3 scenario tests.
- §Scope table — 12 scenarios: Tasks 11 (1, 7, 11) + 12 (2) + 13 (3, 4) +
  14 (8) + 15 (9, 10) + 16 (5, 6) + 17 (12). All 12 mapped.
- §Harness extensions — Phase 0 (FakeTarget, WaitFor) + Phase 2 (AsyncStack,
  AsSender adapters).
- §Composition roots — Phase 1 (Tasks 4, 5, 6, 7, 8, 9).
- §Acceptance criteria #1-7 — final verification (Task 18) hits them all.

**Placeholder scan:**
- Task 4 had a drafting self-correction (schedulerInternals → single
  Scheduler struct). Flagged explicitly, Step 2 of that task does the
  clean-up in one edit.
- Task 8 Step 3 ("Audit how alert.Handle currently dispatches") asks
  the engineer to read one file before editing — this is unavoidable
  for a minimal intrusion, but the edit shape is fully specified.
- Task 10 Step 3 asks the engineer to read `internal/domain/alert.go`
  to fit the AsSender signatures to the actual fields on
  `domain.AlertEvent`. This is not a placeholder — it's a conscious
  instruction to conform to the local convention rather than
  re-spec'ing the domain type here.

**Type consistency:**
- `bootstrap.Scheduler` exports `Leader *checker.LeaderScheduler`
  across Tasks 4 / 10 / 11 / 12 / 13.
- `bootstrap.Worker.MonitoringSvc` is defined once, not referenced by
  tests (tests use DB assertions).
- `bootstrap.Notifier.AlertSvc` + `WithSendOverrides` chain:
  `app.AlertService.WithSendOverrides(map)` returns `*AlertService` —
  Tasks 8 and 10 use that shape.
- `harness.App.JS()` and `harness.App.App.Cipher` referenced in
  Task 10 — adding both is part of Task 10's own steps.
- `port.ChannelSender.Send(ctx, *domain.NotificationChannel,
  *domain.AlertEvent) error` — referenced identically in Task 8
  (port definition) and Task 10 (AsSender adapters).

No other gaps.

---

**Plan complete and saved to `docs/superpowers/plans/2026-04-18-C2-async-pipeline-tests.md`.**

Two execution options:

**1. Subagent-Driven (recommended)** — dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — execute tasks in this session using executing-plans, batch execution with checkpoints.

Which approach?

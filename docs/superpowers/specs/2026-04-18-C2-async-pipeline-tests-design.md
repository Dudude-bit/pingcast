# C2 — Async Pipeline Integration Tests

Status: draft (pending user review)
Date: 2026-04-18
Follows: C1 (API contract tests). Precedes C3 (Playwright journeys)
and C4 (security edge-cases + rate-limit enforcement).

## Purpose

Black-box integration coverage of the full monitoring pipeline:
`monitor create → scheduler fan-out → worker check execution →
check_result → incident open/close → alert publish → notifier
delivery → FakeTelegram/FakeSMTP/FakeWebhookSink`.

C1 locked the synchronous API contract. C2 locks the async behavior
that the API triggers: if a user's API call causes work in the
background, the test suite observes that work end-to-end.

## Source of truth

Same discipline as C1:

1. **OpenAPI** for any response-shape question (rare — C2 is mostly
   server-side effects).
2. **Behavior spec** from C1 (`2026-04-18-C1-api-behavior-spec.md`)
   for tenant isolation, envelope shape, semantic rules like "3
   consecutive failures → incident". Extended inline here for
   pipeline-only invariants.
3. **Domain invariants** (`internal/domain/`) as defaults only.

Tests assert the *intended* pipeline behavior; code bends to match.

## Scope

**In scope — 12 end-to-end scenarios** covering the pipeline under
different shapes of failure, channel types, monitor types, and
thresholds:

| # | Scenario | Invariant under test |
|---|---|---|
| 1 | HTTP 200 monitor, 3 consecutive 500s → incident opens + alert | Failure threshold |
| 2 | DOWN monitor recovers → incident closes + recovery alert | Close-side of cycle |
| 3 | Paused monitor → scheduler skips, no check row appears | Spec §4 pause semantics |
| 4 | Deleted monitor → scheduler no longer dispatches | Cleanup on delete |
| 5 | Telegram-bound monitor fails → FakeTelegram receives message with monitor context | Channel delivery |
| 6 | Webhook-bound monitor fails → FakeWebhookSink receives HMAC-signed JSON | Webhook signing |
| 7 | Monitor target returns 5xx → classified as failure | Checker-worker contract |
| 8 | Monitor target too slow (> timeout_seconds) → timeout failure | Timeout handling |
| 9 | TCP monitor against closed port → failure | TCP checker type |
| 10 | DNS monitor against non-resolving host → failure | DNS checker type |
| 11 | Monitor with `alert_after_failures=1` → alert fires on first failure | Per-monitor threshold |
| 12 | Channel sender fails 3 times → event written to `failed_alerts` (DLQ) | Notifier resilience |

**Explicitly out of scope:**
- Rate-limit enforcement (→ C4)
- Browser journeys (→ C3)
- Leader-election race scenarios (covered by existing unit tests in
  `internal/adapter/checker/leader_scheduler.go`)
- Multi-instance scaling / NATS consumer groups
- Retention / prune jobs (spec §5 — tested at repo layer, not pipeline)

## Harness extensions

### 1. Composition roots for scheduler / worker / notifier

Mirror the C1 pattern where `bootstrap.NewApp(deps)` extracted the
API composition from `cmd/api/main.go`. For each background service:

- `internal/bootstrap/scheduler.go` exports
  `NewScheduler(deps SchedulerDeps) (*Scheduler, error)` returning a
  struct with `Start(ctx)` / `Stop(ctx)` methods.
- Same for worker (`NewWorker`) and notifier (`NewNotifier`).
- `cmd/scheduler/main.go`, `cmd/worker/main.go`, `cmd/notifier/main.go`
  shrink to: read config → open infra → `NewXyz(deps)` → `Start`.

The integration harness calls the same `NewXyz` with test-container
handles + FakeClock + fake channel senders. Proven to work in C1 —
no drift risk between prod and tests.

### 2. Fake HTTP target

`harness.FakeTarget`:

```go
type FakeTarget struct {
    URL string
    // Responses returned in order, then the last one repeats.
    // RespondWith(200), FailNext(3), Slow(5*time.Second) etc.
}
func NewFakeTarget(t *testing.T) *FakeTarget
func (f *FakeTarget) RespondWith(status int) *FakeTarget
func (f *FakeTarget) FailNext(n int) *FakeTarget
func (f *FakeTarget) Slow(d time.Duration) *FakeTarget
func (f *FakeTarget) Hits() int
```

Backed by `httptest.NewServer`. Harness cleanup closes it.

### 3. Polling helper

Async outcomes aren't instantaneous; tests need a bounded-wait
predicate:

```go
func WaitFor(t *testing.T, deadline time.Duration, predicate func() bool, desc string)
```

Usage: `harness.WaitFor(t, 5*time.Second, func() bool { ... }, "incident opened for monitor X")`.

Default deadline 10s, overridable per test.

### 4. Channel senders as injected dependencies

Currently the notifier constructs Telegram/SMTP/Webhook senders
inline with env-driven config. To let tests swap them for fakes:

- Define `port.ChannelSender` interface (one method: `Send(ctx, channel, event) error`).
- `NotifierDeps` carries `map[domain.ChannelType]port.ChannelSender` (nil → use prod default).
- Harness wires `{telegram: FakeTelegram.AsSender(), smtp: FakeSMTP.AsSender(), webhook: FakeWebhookSink.AsSender()}`.

### 5. Manual scheduler tick

Production scheduler ticks every 15s. Tests need determinism:

- `Scheduler.RunOnce(ctx)` exported alongside the ticker-driven
  `Start(ctx)`. Executes one fan-out pass synchronously.
- Harness calls `RunOnce()` directly; doesn't start the ticker.
- Leader-election bypass in test mode: `SchedulerDeps.SkipLeaderElection: true`.

## Test file layout

```
tests/integration/api/
  ...existing C1 files...
  async/
    scheduler_test.go       — scenarios 3, 4 (pause/delete skip)
    failure_detection_test.go — 1, 7, 11 (failure threshold + target shapes)
    recovery_test.go        — 2 (recovery + close)
    timeout_test.go         — 8 (timeout classification)
    tcp_dns_test.go         — 9, 10 (non-HTTP checker types)
    channel_delivery_test.go — 5, 6 (telegram + webhook)
    dlq_test.go             — 12 (delivery failures → failed_alerts)
  harness/
    ...existing harness files...
    fake_target.go          — FakeTarget
    waitfor.go              — WaitFor polling helper
    async.go                — AsyncStack struct wiring scheduler+worker+notifier
```

New harness package is a sibling of the existing one, not a subdir
— they share the `harness` namespace. Test files live under
`tests/integration/api/async/` as a new subpackage to keep the C1
suite unchanged.

Actually — simpler: keep all tests in `tests/integration/api/`, no
subpackage. Async files prefixed `async_` for mental grouping:
`async_scheduler_test.go`, `async_failure_test.go`, etc. Harness
helpers land in the existing `harness` package. One build tag, one
TestMain, one container set.

## Example test

```go
//go:build integration

func TestFailureDetection_ThreeConsecutive500s_OpensIncident(t *testing.T) {
    h := harness.New(t)
    stack := h.StartAsyncStack(t)
    defer stack.Stop()

    target := harness.NewFakeTarget(t).FailNext(3)
    s := h.RegisterAndLogin(t, "", "")

    cr := s.POST(t, "/api/monitors", map[string]any{
        "name": "api",
        "type": "http",
        "check_config": map[string]any{"url": target.URL, "method": "GET"},
        "interval_seconds": 30,
        "alert_after_failures": 3,
    })
    harness.AssertStatus(t, cr, 201)
    var m struct{ ID string }
    cr.JSON(t, &m)

    // Three scheduler ticks → 3 worker-executed failures → 1 incident
    stack.Scheduler.RunOnce(t.Context())
    stack.Scheduler.RunOnce(t.Context())
    stack.Scheduler.RunOnce(t.Context())

    harness.WaitFor(t, 5*time.Second, func() bool {
        var open bool
        _ = h.App.Pool.QueryRow(t.Context(),
            `SELECT EXISTS(SELECT 1 FROM incidents WHERE monitor_id=$1 AND resolved_at IS NULL)`,
            m.ID).Scan(&open)
        return open
    }, "incident opened")
}
```

## Acceptance criteria

C2 is complete when:

1. `make test-integration` runs all C1 + C2 tests with 0 failures.
2. ~12 scenario tests exist in `tests/integration/api/async_*.go`.
3. `bootstrap.NewScheduler`, `bootstrap.NewWorker`, `bootstrap.NewNotifier`
   exist and are consumed by both cmd/ and harness.
4. `cmd/{scheduler,worker,notifier}/main.go` each shrink to <80 lines
   (infra boot + bootstrap.NewXyz call + Start/Stop).
5. `port.ChannelSender` interface defined; prod impls adapted; harness
   fakes inject deterministic senders.
6. No regressions in the 81 C1 tests.
7. Lint clean (`golangci-lint run`).
8. CI workflow unchanged — the existing integration job covers it via
   `go test -tags=integration ./tests/integration/api/...`.

## Risks / decisions deferred

- **Leader-election bypass in prod mode is dangerous.** Gated behind
  a test-only config flag that is NOT reachable from env (only set
  via `SchedulerDeps` struct from the harness). Auditable at compile
  time.
- **Ticker vs manual tick parity.** Manual `RunOnce()` must execute
  the identical code path the ticker uses (same query, same
  publishing). No test-only branches inside RunOnce.
- **DLQ test (#12) requires channel-sender that deterministically
  fails.** `FakeTelegram` gets `.FailNext(n)` like FakeTarget.
- **Ordering of scheduler/worker/notifier under one process.**
  Goroutines; NATS JetStream decouples them. Tests use
  `WaitFor` + polling rather than goroutine sync primitives.

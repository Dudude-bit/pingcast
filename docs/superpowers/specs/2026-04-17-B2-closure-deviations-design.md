# B2 — Closure Deviations Remainder — Design & Plan

**Date:** 2026-04-17
**Parent:** Sub-project B (bug triage & hardening) — `B2` slice
**Scope:** Four small, independent fixes documented as accepted deviations
in the reliability-audit closure Verification Log (items 3.4, 4.2, 4.5, 4.8).
None of them has user-visible or security impact today — these are the
"do it properly like the spec said" cleanups.

## Items

### 3.4 — HTTP client cache by timeout

**File:** `internal/adapter/checker/http.go`

**Current state (B1-landed):** `clientForTimeout(timeoutSecs int) *http.Client` allocates a new `*http.Client` struct on every check whose config has a non-default timeout. The Transport is shared, so there's no connection-pool fragmentation — but there IS per-check allocation + GC churn for the Client wrapper.

**Fix:** cache `*http.Client` by timeout seconds in a `sync.Map`. Typical pingcast configs use 5/10/15/30s timeouts, so the map stays ~4 entries.

```go
type HTTPChecker struct {
    httpClient   *http.Client
    clientByTTL  sync.Map // map[int]*http.Client
}

func (c *HTTPChecker) clientForTimeout(timeoutSecs int) *http.Client {
    if v, ok := c.clientByTTL.Load(timeoutSecs); ok {
        return v.(*http.Client)
    }
    client := &http.Client{
        Timeout:       time.Duration(timeoutSecs) * time.Second,
        CheckRedirect: c.httpClient.CheckRedirect,
        Transport:     c.httpClient.Transport,
    }
    actual, _ := c.clientByTTL.LoadOrStore(timeoutSecs, client)
    return actual.(*http.Client)
}
```

Test: call `clientForTimeout(10)` twice, assert both return the exact same pointer (`==`).

### 4.2 — `requireUser` helper

**Files:** `internal/adapter/http/server.go`

**Current state:** 15 inline copies of

```go
user := UserFromCtx(c)
if user == nil {
    return c.Status(401).JSON(apigen.ErrorResponse{Error: new("unauthorized")})
}
```

**Fix:** extract to a helper — same package, next to `UserFromCtx` in `middleware.go`:

```go
// requireUser extracts the authenticated user from context or writes a 401
// JSON response. Callers return the returned error directly.
func requireUser(c *fiber.Ctx) (*domain.User, error) {
    user := UserFromCtx(c)
    if user == nil {
        return nil, c.Status(fiber.StatusUnauthorized).
            JSON(apigen.ErrorResponse{Error: new("unauthorized")})
    }
    return user, nil
}
```

Usage pattern in handlers:

```go
user, err := requireUser(c)
if err != nil {
    return err
}
// ... use user ...
```

Applied ONLY to the 15 sites in `server.go`. `pages.go` uses `PageMiddleware` which redirects to `/login` on missing session — different error flow, not in scope for this unification. (If pages.go ever needs the same dedup, add a separate `requireUserHTML` helper.)

Test: one table test asserting 401 JSON when `UserFromCtx` returns nil. Existing handler tests cover the success path.

### 4.5 — Touch goroutine semaphore

**File:** `internal/adapter/http/middleware.go`

**Current state:** every authenticated API-key request spawns a goroutine for `apiKeyRepo.Touch`. Detached context from B1/PR5 landed. Unbounded fanout under auth load (load test with 10k rps API-key requests → 10k background goroutines until Touch completes).

**Fix:** add a buffered channel semaphore at package level, cap 50:

```go
// touchSem bounds concurrent API-key Touch goroutines. If the channel is
// full, the Touch for this request is skipped with a Warn log — the
// trade-off is preferred over memory pressure under load.
var touchSem = make(chan struct{}, 50)

// Inside authenticateWithAPIKey, after detached ctx creation:
select {
case touchSem <- struct{}{}:
    touchCtx, touchCancel := xcontext.Detached(c.UserContext(), 5*time.Second, "api-key.touch")
    go func() {
        defer func() {
            <-touchSem
            touchCancel()
        }()
        if err := apiKeyRepo.Touch(touchCtx, apiKey.ID); err != nil {
            slog.Warn("failed to touch api key", "key_id", apiKey.ID, "error", err)
        }
    }()
default:
    slog.Warn("api-key touch skipped — semaphore full", "key_id", apiKey.ID)
}
```

Test: saturate the semaphore with 50 artificial holds, fire a request, assert the "skipped" log and that `apiKeyRepo.Touch` was NOT called.

### 4.8 — 400 response on out-of-range form values

**Files:** `internal/adapter/http/pages.go`

**Current state:** `MonitorCreate` / `MonitorUpdate` silently clamp out-of-range `interval_seconds` / `alert_after_failures` to the default value. User submits `interval_seconds=99999` → form shows it's accepted, but the monitor got `300`. Confusing.

**Fix:** treat out-of-range as a user-facing validation error, re-render the form with a specific message, preserve other fields:

```go
// pages_form_validation.go (new file, same package — keeps pages.go leaner)

type formRange struct {
    field string
    min   int
    max   int
}

func parseIntInRange(c *fiber.Ctx, r formRange, defaultVal int) (int, error) {
    v := c.FormValue(r.field)
    if v == "" {
        return defaultVal, nil
    }
    parsed, err := strconv.Atoi(v)
    if err != nil {
        return 0, fmt.Errorf("%s must be a number", r.field)
    }
    if parsed < r.min || parsed > r.max {
        return 0, fmt.Errorf("%s must be between %d and %d", r.field, r.min, r.max)
    }
    return parsed, nil
}
```

In `MonitorUpdate` / `MonitorCreate`, replace the four silent-clamp blocks:

```go
interval, err := parseIntInRange(c,
    formRange{field: "interval_seconds", min: 30, max: 86400}, 300)
if err != nil {
    return h.render(c, "monitor_form.html", fiber.Map{
        "User":         user,
        "Error":        err.Error(),
        "MonitorTypes": h.monitoring.Registry().Types(),
        // form state preservation — best-effort: re-show the raw FormValue
    })
}
```

Applied only to `pages.go` form handlers — JSON API (`CreateMonitor` in `server.go`) already validates via the apigen typed request and returns 400 cleanly.

Test: POST with `interval_seconds=99999` → 200 response (HTML re-render) + body contains "must be between 30 and 86400".

## Files touched

- `internal/adapter/checker/http.go` — 3.4
- `internal/adapter/http/middleware.go` — 4.2 helper + 4.5 semaphore
- `internal/adapter/http/server.go` — 4.2 apply helper to 15 sites
- `internal/adapter/http/pages.go` — 4.8 apply helper
- `internal/adapter/http/pages_form_validation.go` (new) — 4.8 helper
- Tests: per-item.

Total expected diff: ~150 added / ~40 removed across ~6 files.

## Commit structure

Single commit (`refactor: B2 — closure deviations remainder (3.4, 4.2, 4.5, 4.8)`).

## Testing strategy

- Unit test per item (pointer identity for 3.4 cache; 401 helper test for 4.2; semaphore saturation test for 4.5; out-of-range form test for 4.8).
- Full `build/vet/test/race` gate at the end.
- No integration tests required — all changes are local to the adapter layer.

## Success criteria

1. Lint delta: no new errors, 4.5 semaphore-introduced goroutine pattern passes race detector.
2. All 4 items' unit tests pass.
3. Rollback plan: single commit, revert-friendly.

## Risks

| Risk | Mitigation |
|---|---|
| 4.5 semaphore-full case drops Touches silently on sustained overload → `last_used_at` becomes stale | Explicit Warn log; B1 has no dashboard yet; document that LastUsed is eventually-consistent under load. |
| 4.8 form re-render loses user's other field values | Low concern; existing error flow already has this issue. Best-effort preservation only. |
| 4.2 helper swallows the 401 error-write call's own failure | `fiber.Ctx.JSON` returns an error; returning it propagates correctly — same as current inline code. |
| 3.4 cache grows unbounded if bad actors submit many distinct timeouts | `cfg.Timeout` is clamped to int, and valid pingcast configs produce ~5 distinct values. At most 2³¹ entries — negligible. If we want belt-and-braces, size the map via `sync.Map` (already does — no eviction but entries are tiny). No action. |

## Out of scope

- Further lint cleanup (B3)
- UX of form error display (sub-project D)
- Rate-limiter on other endpoints, CSRF (future)

---

## Implementation Plan

Each task is TDD-style where practical. Steps are minimal since each item
is small.

### Task 1 — Branch + baseline

- [ ] Create branch `b2-closure-deviations` from main.
- [ ] Quick green gate: `go build ./... && go vet ./... && go test -count=1 -short ./...`

### Task 2 — 3.4 HTTP client cache

- [ ] Test `TestHTTPChecker_ClientForTimeout_Caches` in `internal/adapter/checker/http_cache_test.go`:
  call `clientForTimeout(10)` twice, assert `==` identity.
- [ ] Run — FAIL.
- [ ] Add `clientByTTL sync.Map` field to `HTTPChecker`; update `clientForTimeout` to use it.
- [ ] Run — PASS.

### Task 3 — 4.2 requireUser helper

- [ ] Test `TestRequireUser_NilReturns401` in `internal/adapter/http/middleware_test.go`
  (new file if one doesn't exist; otherwise in `handler_test.go`).
  Create a fiber app with a handler that calls `requireUser`; send a
  request with no user set in `Locals`; assert 401 + body `"unauthorized"`.
- [ ] Run — FAIL (`requireUser` undefined).
- [ ] Add `requireUser` to `middleware.go`.
- [ ] Run — PASS.
- [ ] Apply helper to the 15 sites in `server.go`:
  replace each `user := UserFromCtx(c); if user == nil { return c.Status(401)... }`
  with `user, err := requireUser(c); if err != nil { return err }`.
- [ ] Run `go test -count=1 -short ./internal/adapter/http/...` — PASS (existing tests cover success paths).

### Task 4 — 4.5 Touch goroutine semaphore

- [ ] Test `TestAuthWithAPIKey_TouchSemaphoreFull_SkipsTouch` in
  `internal/adapter/http/middleware_test.go`:
  - Saturate a test-local semaphore by filling 50 slots.
  - Fire the middleware path.
  - Assert `apiKeyRepo.Touch` was NOT called (mock .AssertNotCalled) and
    a "skipped" Warn log was emitted (capture via `slog` test handler).
  - To enable this test, expose `touchSem` as a package-level `var` with
    a test-only setter (already a package var; test can set `touchSem = make(chan struct{}, 0)` before and restore after).
- [ ] Run — FAIL (current code always spawns a goroutine).
- [ ] Apply the select-default pattern from the spec to `authenticateWithAPIKey`.
- [ ] Run — PASS.
- [ ] Run `go test -race ./internal/adapter/http/...` — PASS (catches any goroutine-leak).

### Task 5 — 4.8 form range validation returns 400

- [ ] Test `TestMonitorUpdate_InvalidInterval_Returns400` in
  `internal/adapter/http/pages_validation_test.go`:
  POST form with `interval_seconds=99999` → HTML response contains
  "must be between 30 and 86400"; the handler returns 200 (re-render),
  not redirect.
- [ ] Run — FAIL (current behaviour silently clamps).
- [ ] Create `internal/adapter/http/pages_form_validation.go` with
  `parseIntInRange`.
- [ ] Replace the four silent-clamp blocks in `MonitorUpdate` and
  `MonitorCreate` with calls to the helper.
- [ ] Run — PASS.

### Task 6 — Final gate + commit + merge

- [ ] `go build ./... && go vet ./... && go test -count=1 ./... && go test -race -count=1 ./...` — all green.
- [ ] Lint delta vs pre-B2 baseline: no new warnings.
- [ ] `git add -A && git reset docs/articles/`
- [ ] Commit with structured message listing the 4 items.
- [ ] `git checkout main && git merge --ff-only b2-closure-deviations && git branch -d b2-closure-deviations`

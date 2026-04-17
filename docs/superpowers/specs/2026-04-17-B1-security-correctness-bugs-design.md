# B1 — Real Security/Correctness Bugs — Design

**Date:** 2026-04-17
**Parent:** Sub-project B (bug triage & hardening) — `B1` slice
**Scope:** Targeted fixes for the small set of bugs that have real security or
correctness impact. Mechanical lint cleanup (errcheck, shadow, G115) is
explicitly deferred to sub-project **B3**. Closure deviations that are
DRY/UX/scale-only (3.4, 4.2, 4.5, 4.8) are deferred to **B2**.

## Summary

Seven discrete fixes, all cohesive under "prevent real-world exploitation or
silent data loss." No shared refactor, no architectural change. Each item
is independently commitable; they are shipped together because they form
the security-minded polish pass after the reliability-audit landing.

## Items

### 1. TLS MinVersion on HTTP monitor checker (lint G402)

**File:** `internal/adapter/checker/http.go:63`

**Bug:** `&tls.Config{}` leaves `MinVersion` at Go's zero-value, which permits
TLS 1.0 and 1.1 handshakes. A monitor targeting an HTTPS endpoint that has
been correctly upgraded to TLS 1.2+ by the operator would still pass a
check against a mis-configured origin advertising TLS 1.0 — masking a real
security regression.

**Fix:**

```go
TLSClientConfig: &tls.Config{
    MinVersion: tls.VersionTLS12,
},
```

**Test:** table-driven unit test against a local TLS server configured with
`MaxVersion: tls.VersionTLS11` — check must error with a TLS-handshake
message. Assert error via `errors.As` against `*tls.AlertError` rather than
substring-matching the message.

### 2. `template.JS` XSS placeholder (lint G203)

**File:** `internal/adapter/http/pages.go:212`, `internal/web/templates/monitor_detail.html`

**Bug:** `template.JS(chartJSON)` bypasses HTML/JS auto-escape. The current
value is always `json.Marshal([]struct{}{})` → `"[]"` — exploitation is not
possible today, but the pattern is a foot-gun for the next change that
populates `chartJSON` from database or user input.

**Fix:** Remove the dead placeholder.

- Delete `chartJSON, _ := json.Marshal([]struct{}{})` from `pages.go`.
- Delete the `"ChartData": template.JS(chartJSON)` map key.
- Remove any `{{.ChartData}}` references from `monitor_detail.html`.

When chart rendering becomes a real feature, the correct approach (typed
data + `template` auto-escape) is applied fresh — unused code now isn't
earning its place on the attack surface.

### 3. Silent error in rate-limiter reset (lint SA9003)

**File:** `internal/adapter/http/pages.go:110`

**Bug:**

```go
if err := h.rateLimiter.Reset(c.UserContext(), email); err != nil {
    // Non-blocking: login succeeded, just log
}
```

The comment promises logging; body is empty. Redis failures after
successful login silently accumulate — operators cannot detect them.

**Fix:**

```go
if err := h.rateLimiter.Reset(c.UserContext(), email); err != nil {
    slog.Warn("rate limiter reset failed after successful login", "email", email, "error", err)
}
```

### 4. Unused test helper (`createSessionForUser`)

**File:** `internal/adapter/http/handler_test.go:81`

**Bug:** Dead code; may silently go stale and mis-lead future test authors
(the helper references an API that could change without the compiler
noticing).

**Fix:** Delete the function. If a future test needs it, re-introduce it
then with the current API.

### 5. Login timing-attack — user enumeration (new finding)

**File:** `internal/app/auth.go:68-84`

**Bug:** Current `AuthService.Login` returns early when `GetByEmail` fails
(user not found), *skipping* the bcrypt comparison:

```go
user, hash, err := s.users.GetByEmail(ctx, email)
if err != nil {
    return nil, "", fmt.Errorf("invalid email or password")  // ~1-5 ms
}
if !CheckPassword(hash, password) {
    return nil, "", fmt.Errorf("invalid email or password")  // ~100 ms (bcrypt)
}
```

Both paths return the same generic message — good. But response latency
differs by ~100 ms, so an attacker who measures timing can enumerate valid
emails. This is a well-known class of bug.

**Fix:** Always run a bcrypt compare, even when the user does not exist,
against a precomputed dummy hash:

```go
// package-level: precomputed at init to equalize Login timing for missing users.
var dummyHash = mustHashPassword("equalize-timing-never-matches")

func mustHashPassword(p string) string {
    h, err := HashPassword(p)
    if err != nil {
        panic(err)
    }
    return h
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*domain.User, string, error) {
    user, hash, err := s.users.GetByEmail(ctx, email)
    if err != nil {
        // Perform a dummy compare so the response time does not reveal
        // whether the email exists. Result is discarded.
        _ = CheckPassword(dummyHash, password)
        return nil, "", fmt.Errorf("invalid email or password")
    }
    if !CheckPassword(hash, password) {
        return nil, "", fmt.Errorf("invalid email or password")
    }
    // ... (unchanged session creation) ...
}
```

**Test:** `go test -bench` style benchmark with `b.Run("missing", …)` and
`b.Run("wrong-password", …)` — assert the ratio of medians is within 3×
(generous — real bcrypt jitter is ~10%). Runs only under `-short=false`
because bcrypt at `DefaultCost=10` is intentionally expensive.

### 6. `ErrUserExists` sentinel + Register classification

**Files:** `internal/domain/errors.go`, `internal/adapter/postgres/user_repo.go`, `internal/app/auth.go`, `internal/adapter/http/server.go`, `internal/adapter/http/pages.go`

**Bug:** `AuthService.Register` wraps any `users.Create` error with
`fmt.Errorf("create user: %w", err)`. For a duplicate email the user
sees a generic "Registration failed" — *handler-level* safe — but the
app and Postgres layers cannot classify duplicate-email as a normal
event. Log spam and missed metrics. Parent spec §4.4 prescribed a
`ErrUserExists` sentinel; this was deferred during closure.

**Fix:**

```go
// internal/domain/errors.go
var ErrUserExists = errors.New("user already exists")
```

```go
// internal/adapter/postgres/user_repo.go — Create method
_, err := r.q.CreateUser(ctx, ...)
if err != nil {
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
        return nil, domain.ErrUserExists
    }
    return nil, err
}
```

```go
// internal/app/auth.go — Register
user, err := s.users.Create(ctx, email, slug, hash)
if err != nil {
    if errors.Is(err, domain.ErrUserExists) {
        return nil, "", err  // pass through, do not wrap
    }
    return nil, "", fmt.Errorf("create user: %w", err)
}
```

```go
// internal/adapter/http/server.go — Register handler
user, sessionID, err := s.auth.Register(c.UserContext(), ...)
if err != nil {
    if errors.Is(err, domain.ErrUserExists) {
        slog.Info("duplicate registration attempt", "email", req.Email)
    } else {
        slog.Warn("registration failed", "error", err)
    }
    return c.Status(400).JSON(apigen.ErrorResponse{Error: new("registration failed")})
}
```

**Security deviation from parent spec §4.4:** parent prescribed mapping
`ErrUserExists → "email already registered"`. That message *enables*
user enumeration — an attacker probes with candidate emails and reads
the response body. This plan **explicitly rejects the parent prescription**
and keeps the generic `"registration failed"` body. Internal
classification (log level, future metrics, future "did you mean to log
in?" flow behind captcha) is still valuable and is preserved.

Symmetry: `pages.go` `RegisterSubmit` applies the same rule — render
`register.html` with `Error: "Registration failed"` regardless of
whether the root cause was `ErrUserExists` or some other failure.

Note: `handleDown` in `app/monitoring.go` already uses `errors.Is(err,
domain.ErrIncidentExists)` (landed in reliability closure) — this adds a
second sentinel of the same shape, no pattern change.

### 7. HTTP response classifier (`httperr.ClassifyHTTPError`)

**Files:** `internal/adapter/httperr/response.go` (new), `internal/adapter/http/server.go`

**Bug:** Five handlers in `server.go` still respond with raw error text:
`CreateChannel:507`, `UpdateChannel:539`, `DeleteChannel:550`,
`BindChannel:567`, `UnbindChannel:578`. Each does:

```go
if err != nil {
    return c.Status(400).JSON(apigen.ErrorResponse{Error: new(err.Error())})
}
```

Leaks internal error text (schema fragments, constraint names, wrapped
SQL strings) to unauthenticated and authenticated callers alike. The
setup.go global error handler was fixed in reliability closure
(`"internal error"` on unhandled), but these handlers return inline
without reaching it.

**Fix:** New file `internal/adapter/httperr/response.go`:

```go
package httperr

import (
    "errors"

    "github.com/kirillinakin/pingcast/internal/domain"
)

// ClassifyHTTPError maps a domain error to an HTTP status code and a
// client-safe message. Unclassified errors collapse to 500 / "internal
// error" — the raw error text is the caller's responsibility to log.
func ClassifyHTTPError(err error) (int, string) {
    switch {
    case errors.Is(err, domain.ErrNotFound):
        return 404, "not found"
    case errors.Is(err, domain.ErrValidation):
        return 400, "invalid request"
    case errors.Is(err, domain.ErrForbidden):
        return 403, "forbidden"
    case errors.Is(err, domain.ErrConflict):
        return 409, "conflict"
    case errors.Is(err, domain.ErrUserExists):
        return 400, "registration failed"  // enumeration-safe (see B1 §6)
    default:
        return 500, "internal error"
    }
}
```

Apply to the five channel handlers:

```go
if err != nil {
    slog.Warn("channel handler error", "path", c.Path(), "error", err)
    status, msg := httperr.ClassifyHTTPError(err)
    return c.Status(status).JSON(apigen.ErrorResponse{Error: new(msg)})
}
```

**Non-goal:** classifying every handler in the codebase. This ships the
pattern (plus tests) and applies it to the five sites that leak raw
errors today. Subsequent handlers can adopt it organically; a sweep
across all handlers is scope for **B3** (hygiene pass) or a dedicated
follow-up.

## Files touched (summary)

- `internal/adapter/checker/http.go` — §1
- `internal/adapter/http/pages.go` — §2, §3, §6 (RegisterSubmit branch)
- `internal/web/templates/monitor_detail.html` — §2
- `internal/adapter/http/handler_test.go` — §4
- `internal/app/auth.go` — §5, §6
- `internal/domain/errors.go` — §6
- `internal/adapter/postgres/user_repo.go` — §6 (surface unique violation)
- `internal/adapter/http/server.go` — §6, §7
- `internal/adapter/httperr/response.go` (new) — §7
- Tests: unit tests per item, benchmark for §5.

Total expected diff: **~200 added lines, ~30 removed lines** across ~10 files.

## Commit structure

Single commit (`fix: B1 — security & correctness bugs`). The items are
independent but cohere under one theme and are small; bisectable value of
splitting further is low for a 200-line diff. If individual items fail
review, drop them with targeted fixups rather than splitting PRs.

## Testing strategy

- Unit tests per item (TLS handshake, empty-branch logging, Login timing
  parity, ErrUserExists classification, HTTP response classifier table).
- Benchmark (§5) runs under `go test -bench=Login ./internal/app/... -run=^$`.
- No new integration tests required — existing testcontainers integration
  suite covers Postgres paths via `tests/integration/repo_test.go`; a
  minimal `ErrUserExists` assertion is added there.
- Race detector: full suite must pass `go test -race`. No new goroutines
  introduced, so low risk.

## Success criteria

1. `golangci-lint run` outputs no longer lists G402, G203, SA9003, or the
   `createSessionForUser` unused warning. (Remaining ~60 warnings unchanged.)
2. Login timing benchmark: non-existent-user and wrong-password medians
   within 3× of each other.
3. Integration test: `RegisterSubmit` with duplicate email → 400
   + response body contains "Registration failed" + nothing else + log
   line at `Info` level tagged `duplicate registration attempt`.
4. Integration test: `CreateChannel` with invalid config → 400
   + response body is a classifier-produced message (not raw error text).
5. Build/vet/test/test-race all green.
6. Rollback plan: single commit, `git revert` is enough.

## Out of scope

- Other 60 lint warnings (errcheck, shadow, G115, etc.) → **B3**.
- Closure deviations 3.4, 4.2, 4.5, 4.8 → **B2**.
- Expanded rate-limiting on other endpoints, CSRF review, session-cookie
  hardening → future work, separate specs if/when prioritised.
- Bcrypt cost review (currently `DefaultCost = 10` ≈ 80-100 ms) → out
  of scope; §5 fix is orthogonal to cost tuning.

## Risks

| Risk | Mitigation |
|---|---|
| Dummy bcrypt in §5 could panic at init if `bcrypt.GenerateFromPassword` fails | Panic is acceptable — `bcrypt.GenerateFromPassword` only fails on invalid cost (not our case); process start-up failure is visible. |
| `MinVersion: TLS12` rejects legitimate monitors against legacy-TLS-only hosts | Low: users who actively monitor a TLS 1.0 origin can patch or operator raise it. Alerting them via a clearer "TLS handshake failed" check result *is* the feature, not the bug. |
| Dead-code delete in §2 touches a template file | Template tests exist (`handler_test.go`); breakage caught at `go test`. |
| `ErrUserExists` pg-error classification triggered by *any* unique-constraint on user rows (e.g. future unique-on-slug) | Scope of `users.Create` today: email is the only unique constraint populated from user input; slug is validated upstream but also unique. If slug duplicate arrives, it would also be classified as `ErrUserExists` — slightly misleading. **Decision:** accept for now; if a `ErrSlugExists` need arises, refine via `pgErr.ConstraintName` check. |

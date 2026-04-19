# C4 — Security Edge-Cases + Rate-Limit Enforcement

Status: draft (pending user review)
Date: 2026-04-19
Follows: C3 (Playwright journeys). Completes the C series.

## Purpose

Close the remaining gaps in the integration test surface:

1. **Rate-limit enforcement** per spec §5 — currently only register+login
   share a single 5/15min bucket; status/write/read endpoints aren't
   limited at all.
2. **API key scope enforcement** — code has `requiredScope` logic in
   middleware; no integration test confirms it actually blocks
   cross-scope calls.
3. **Encryption roundtrip** — `ENCRYPTION_KEYS` was wired in C1 via
   `port.Cipher`, but nothing asserts that sensitive payloads are
   stored as ciphertext in Postgres.

## Source of truth

- C1 behavior spec §5 (rate-limit numbers) and §2 (API key auth).
- OpenAPI for response shapes.
- `internal/adapter/redis/ratelimit.go` for the existing primitive.

Tests assert the intended behaviour; code bends to match. Same
discipline as C1/C2.

## Scope

### In scope — 10 tests + refactor

**Rate-limit tests (6):**

| # | Scope | Test |
|---|---|---|
| 1 | Register | N register attempts from same IP allowed, N+1 returns 429 + Retry-After |
| 2 | Login | 5 failed logins on same email → 6th returns 429 |
| 3 | Login reset | 4 failed + 1 success → unlimited further attempts (counter reset) |
| 4 | Status page | 60 anon hits to `/api/status/{slug}` allowed, 61st → 429 |
| 5 | Write API | N authenticated writes allowed, N+1 → 429 with canonical envelope |
| 6 | Read API | N reads allowed, N+1 → 429 |

**API key scope tests (3):**

| # | Test |
|---|---|
| 7 | Key with `monitors:read` scope: `GET /api/monitors` → 200 |
| 8 | Same key: `POST /api/monitors` → 403 FORBIDDEN_TENANT |
| 9 | Key with `monitors:write` scope: `POST /api/monitors` → 201 |

**Encryption roundtrip (1):**

| # | Test |
|---|---|
| 10 | Create monitor with sentinel string in check_config, SELECT raw column from Postgres, assert string NOT present; re-read monitor via API, assert string present |

### Out of scope (explicitly)

- Channel config redaction — spec §8.9 defaults to redacted, but code
  doesn't implement it. Moved to the A (bug audit) track because it
  requires a feature, not just a test.
- Password bcrypt cost — covered by `auth_timing_test.go` already.
- CSRF — middleware removed in C1; there's nothing to test.
- Session rotation — C1 already covers post-logout cookie invalidation.

## Design

### Rate-limiter infrastructure refactor

Replace the single `port.RateLimiter` on `Server` with a typed
`RateLimiters` carrier:

```go
// internal/port/ratelimit.go (new)
type RateLimiters struct {
    Register RateLimiter // key: IP
    Login    RateLimiter // key: email (lowercased)
    Status   RateLimiter // key: IP+slug
    Write    RateLimiter // key: user ID
    Read     RateLimiter // key: user ID
}
```

Each field is the existing `port.RateLimiter` interface (Allow / Reset).
Implementation is still `redisadapter.NewRateLimiter` with different
(prefix, max, window) per bucket.

### Configurable windows

`bootstrap.AppDeps` gains:

```go
type RateLimitConfig struct {
    RegisterPerHour  int             // default 10
    LoginPer15Min    int             // default 5
    StatusPerMin     int             // default 60
    WritePerMin      int             // default 300
    ReadPerMin       int             // default 600
    WindowOverride   time.Duration   // test-only: overrides ALL windows
}
```

Production passes nil → defaults. Tests pass `WindowOverride=1s` + small
maxes (e.g. 3) so a burst-of-N test completes in seconds without
waiting real windows.

This sidesteps the FakeClock+Redis mismatch (redis_rate uses server
time, not process time) — we don't fake time, we make windows short.

### Middleware application

A new Fiber middleware factory in `internal/adapter/http/ratelimit_mw.go`:

```go
func rateLimitMW(limiter port.RateLimiter, keyFn func(*fiber.Ctx) string) fiber.Handler
```

Wired into `setup.go` per scope:
- `rateLimitMW(rls.Read, userKey)` on GET /api/* except public.
- `rateLimitMW(rls.Write, userKey)` on POST/PUT/DELETE /api/*.
- `rateLimitMW(rls.Status, ipSlugKey)` on /api/status/{slug}.
- Register/Login rate-limits stay inline in handlers (existing code).

On burst exhaustion, middleware emits canonical 429 envelope with
Retry-After via `httperr.WriteRateLimited`.

### API key scope enforcement

Already implemented in `middleware.go:requiredScope` + `apiKey.HasScope`.
C4 only adds tests for the behaviour — no code changes required here.

### Encryption roundtrip

`internal/adapter/postgres/monitor_repo.go` already encrypts check_config
on write and decrypts on read via `port.Cipher`. Test writes a monitor
with a distinctive substring, queries the raw `check_config` column,
asserts the substring is absent (ciphertext), then reads via API and
asserts the substring is present (decrypted).

## Acceptance criteria

1. All 10 C4 tests pass via `make test-integration`.
2. No regressions in 93 C1+C2 integration tests.
3. Lint clean.
4. Rate-limit middleware applied to /api/* per spec §5.
5. `bootstrap.AppDeps.RateLimits` field exposes the configurable numbers.
6. Production `cmd/api/main.go` unchanged in behaviour — defaults
   match spec.

## Risks

- **Counting with small windows is timing-sensitive.** Test config uses
  `WindowOverride=1*time.Second` + small bucket size. Burst + 1 check
  must finish within 1s. Mitigation: use `max=3` with `window=5*time.Second`
  in tests — 4-request burst, check 4th returns 429. Well within 5s.
- **Per-middleware ordering on /api/\*.** Rate limit must run AFTER
  auth (because write/read rate-limit keys use user.ID). Wire via
  the apigen middleware chain, same layer as `authMiddlewareSelector`.
- **Session-cookie auth tests interact with login rate-limit.** Tests
  that do many logins in one test will hit the bucket. Use a fresh
  email per test, and `flushdb` between tests (already done by Reset).

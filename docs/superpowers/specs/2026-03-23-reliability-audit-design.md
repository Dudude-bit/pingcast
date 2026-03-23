# PingCast Reliability Audit — Design Document

**Date:** 2026-03-23
**Scope:** Comprehensive reliability audit and fixes across all components
**Approach:** Systematic — grouped by component, one PR per component

## Summary

Preventive reliability audit of PingCast identified 25 issues across 4 components (NATS messaging, Notifier, Checker, API). Issues range from critical (silent message loss, race conditions) to medium (missing validation, logging improvements). All fixes are organized into 4 independent work streams.

---

## Component 1: NATS Messaging

### 1.1. Handle Publish Errors in HTTP Handlers

**Problem:** `PublishMonitorChanged()` errors are ignored in `server.go`. User gets 200, but checker never learns about the monitor.

**Fix:** If Publish fails, return HTTP 500 with message `"monitor saved but sync failed, retry later"`. The DB write has already succeeded, so the monitor exists but isn't being checked until the next sync.

**Files:** `internal/adapter/http/server.go` (lines 169, 244, 261, 285, 287)

### 1.2. Per-Message Context in Subscribe Closures

**Problem:** App-level context passed to NATS handler closures. On shutdown, all in-flight messages get cancelled context, causing failures and potential message loss.

**Fix:** Create per-message context with timeout instead of using the captured app context:
```go
msgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
handler(msgCtx, &event)
```

`consumer.Stop()` prevents new message delivery; in-flight messages complete with their own deadline.

**Files:** `internal/adapter/nats/subscriber.go` (lines 47, 98)

### 1.3. MaxDeliver + BackOff for MonitorSubscriber

**Problem:** MonitorSubscriber has no `MaxDeliver` — failed messages retry infinitely.

**Fix:** Add `MaxDeliver: 5` and `BackOff` array matching AlertSubscriber's pattern.

**Files:** `internal/adapter/nats/subscriber.go` (lines 31-34)

### 1.4. MaxBytes on All Streams

**Problem:** No `MaxBytes` limit on MONITORS, CHECKS, ALERTS streams. If consumers stop, disk fills up.

**Fix:** Add `MaxBytes: 1 * 1024 * 1024 * 1024` (1 GB) to all three stream configs.

**Files:** `internal/adapter/nats/client.go` (lines 31-67)

---

## Component 2: Notifier

### 2.1. SMTP Sender: Context + Timeout

**Problem:** `Send(_ context.Context, ...)` explicitly ignores context. `gosmtp.SendMail` has no timeout — can hang indefinitely.

**Fix:** Replace `gosmtp.SendMail` with `net/smtp` using explicit dial timeout and context cancellation. Use the context parameter instead of discarding it.

**Files:** `internal/adapter/smtp/sender.go`

### 2.2. Parallel Channel Delivery

**Problem:** Channels are sent sequentially. N channels x 10s timeout = catastrophic backpressure. Can exceed NATS AckWait.

**Fix:** Use `errgroup.Group` with per-channel goroutines. Each channel gets its own `context.WithTimeout(ctx, 10*time.Second)`. Overall group timeout: 30s.

**Files:** `internal/app/alert.go` (lines 57-84)

### 2.3. DLQ Write Before Ack

**Problem:** Alert is Ack'd to NATS before DLQ write. If DLQ write fails, partial failure metadata is lost forever.

**Fix:** Move `writeToDLQ()` before `return nil`. If DLQ write fails, return error so NATS retries the message.

**Files:** `internal/app/alert.go` (lines 86-104)

### 2.4. Wire DLQConsumer in Notifier

**Problem:** `DLQConsumer` is defined but never instantiated in `cmd/notifier/main.go`. After MaxDeliver=10, alerts disappear with no trace.

**Fix:** Create and start `DLQConsumer` in notifier's main, analogous to `alertSub`. It listens for max delivery advisories and persists to `failed_alerts`.

**Files:** `cmd/notifier/main.go`

### 2.5. Circuit Breaker Per-Channel

**Problem:** One CB per channel type (telegram, email). One user's misconfigured channel triggers CB for all users of that type.

**Fix:** Move CB creation to `CreateSenderWithRetry()` with key `channelType:channelID`. Store CBs in `sync.Map` inside Registry.

**Files:** `internal/adapter/channel/registry.go`

### 2.6. Response Body Drain

**Problem:** Telegram and Webhook senders don't drain response body before closing. Prevents HTTP connection reuse.

**Fix:** Add `io.Copy(io.Discard, resp.Body)` before `resp.Body.Close()` in both senders.

**Files:** `internal/adapter/telegram/sender.go`, `internal/adapter/webhook/sender.go`

### 2.7. BackOff Array Matches MaxDeliver

**Problem:** BackOff has 5 elements but MaxDeliver=10. Retries 6-10 all use last element (60s) = 5 minutes of waiting.

**Fix:** Extend BackOff to 10 elements: `2s, 5s, 10s, 30s, 60s, 120s, 120s, 120s, 120s, 120s`.

**Files:** `internal/adapter/nats/subscriber.go` (line 84)

### 2.8. Error Classification in Metrics

**Problem:** Metrics only record success/failure. Can't distinguish timeout vs auth error vs rate limit.

**Fix:** Add `reason` label to `RecordAlertSent()`: `timeout`, `auth_error`, `rate_limited`, `network_error`, `unknown`. Determine from error type / HTTP status code.

**Files:** `internal/app/alert.go`, metrics interface

---

## Component 3: Checker

### 3.1. Startup Order: Subscribe Before Run

**Problem:** `leaderScheduler.Run()` starts before `monitorSub.Subscribe()`. Race condition on `s.monitors` map between scheduler iteration and subscription handler.

**Fix:** Reorder: `monitorSub.Subscribe()` first, then `go leaderScheduler.Run()`. Subscribe buffers NATS messages while Run starts with complete state.

**Files:** `cmd/checker/main.go` (lines 136, 140)

### 3.2. Cleanup Goroutine: WaitGroup + Graceful Stop

**Problem:** Cleanup goroutine is not waited on during shutdown. Can access DB pool after it's closed.

**Fix:** Add `sync.WaitGroup` for cleanup goroutine. Shutdown sequence: cancel context -> `wg.Wait()` -> close DB pool.

**Files:** `cmd/checker/main.go` (lines 171-215, 218-235)

### 3.3. Cleanup Lock Failure: Error Differentiation

**Problem:** `cleanupMutex.Lock()` failure logged as Debug. If Redis is down, cleanup silently stops. Unbounded check_results growth.

**Fix:** Differentiate error types:
- `redsync.ErrFailed` (lock held by another instance) -> `slog.Debug` (normal)
- All other errors -> `slog.Warn` (infrastructure problem)

**Files:** `cmd/checker/main.go` (lines 179-182)

### 3.4. HTTP Client Caching by Timeout

**Problem:** New `*http.Client` created per check with custom timeout. All share one Transport, causing connection pool fragmentation.

**Fix:** Add `sync.Map` with key `timeout_seconds -> *http.Client`. Typical timeouts are few (5, 10, 15, 30s), so map stays small.

**Files:** `internal/adapter/checker/http.go` (lines 109-116)

---

## Component 4: API

### 4.1. Atomic ProcessCheckResult

**Problem:** Read-modify-write pattern in `ProcessCheckResult()` — GetByID then UpdateStatus. Race condition causes duplicate incidents or missed recovery.

**Fix:** Single SQL query:
```sql
UPDATE monitors SET current_status = $2
WHERE id = $1
RETURNING (SELECT current_status FROM monitors WHERE id = $1) AS previous_status
```

**Files:** `internal/app/monitoring.go` (lines 227-270), `internal/adapter/postgres/monitor_repo.go`

### 4.2. Nil Check on UserFromCtx

**Problem:** `ListAPIKeys` and potentially other handlers don't nil-check `UserFromCtx()`. Panic if auth middleware fails.

**Fix:** Grep all handlers for `UserFromCtx()` without nil check. Add uniform check or extract to helper `requireUser(c *fiber.Ctx) (*domain.User, error)`.

**Files:** `internal/adapter/http/server.go`

### 4.3. Atomic TogglePause

**Problem:** Read-modify-write in `TogglePause()`. Concurrent requests can cause lost update.

**Fix:** Single SQL:
```sql
UPDATE monitors SET is_paused = NOT is_paused
WHERE id = $1 AND user_id = $2
RETURNING is_paused
```

**Files:** `internal/app/monitoring.go`, `internal/adapter/postgres/monitor_repo.go`

### 4.4. Error Sanitization

**Problem:** Raw `err.Error()` returned to clients. Leaks internal info (user enumeration, schema details).

**Fix:** Map domain errors to client-safe messages:
- `ErrUserExists` -> `"email already registered"`
- `ErrNotFound` -> `"resource not found"`
- Everything else -> `"internal error"` (details in logs only)

**Files:** `internal/adapter/http/server.go`

### 4.5. Background Touch: Detached Context

**Problem:** `c.UserContext()` used in background goroutine for API key Touch. Context may be cancelled before goroutine executes.

**Fix:** Use `context.WithTimeout(context.Background(), 5*time.Second)`. Add semaphore (buffered channel, cap ~50) to limit concurrent goroutines.

**Files:** `internal/adapter/http/middleware.go` (lines 90-94)

### 4.6. Incident Cooldown: Database-Level Uniqueness

**Problem:** `IsInCooldown()` + `Create()` is not atomic. Concurrent check results can create duplicate incidents.

**Fix:** Add unique partial index:
```sql
CREATE UNIQUE INDEX idx_incidents_active_monitor
ON incidents (monitor_id) WHERE resolved_at IS NULL
```
Catch constraint violation in Go, treat as "cooldown active".

**Files:** `internal/adapter/postgres/`, new migration

### 4.7. Rate Limiting on /api/auth/register

**Problem:** Login has rate limiting, registration doesn't. Enables user enumeration and spam accounts.

**Fix:** Use same `rateLimiter.Allow()` as login. Key: IP address. Limit: 5 attempts per 15 minutes.

**Files:** `internal/adapter/http/server.go`

### 4.8. Range Validation on Form Values

**Problem:** `interval_seconds` and `alert_after_failures` parsed without range validation. Accepts negative, zero, or unreasonable values.

**Fix:** Validate ranges:
- `interval_seconds`: min 30, max 86400
- `alert_after_failures`: min 1, max 10

Return 400 with description on invalid values.

**Files:** `internal/adapter/http/pages.go` (lines 229-240)

### 4.9. Crypto Error Reporting for Key Rotation

**Problem:** When both encryption keys fail, only primary key error is returned. Confuses debugging during key rotation.

**Fix:** Return wrapped error with both causes:
```go
return nil, fmt.Errorf("decrypt failed: primary: %w; old key: %v", primaryErr, oldErr)
```

**Files:** `internal/crypto/crypto.go` (lines 63-84)

---

## Implementation Order

1. **PR 1: NATS Messaging** (issues 1.1-1.4) — foundation for all inter-service communication
2. **PR 2: Notifier** (issues 2.1-2.8) — depends on NATS fixes being in place
3. **PR 3: Checker** (issues 3.1-3.4) — independent of Notifier
4. **PR 4: API** (issues 4.1-4.9) — independent, largest scope

PRs 3 and 4 can be developed in parallel after PR 1 is merged.

## Testing Strategy

- Each fix should include unit tests for the specific behavior change
- Integration tests (testcontainers) for NATS message flow, DB atomicity
- Race condition fixes verified with `go test -race`
- Shutdown behavior tested with context cancellation in tests

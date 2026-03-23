# PingCast Reliability Audit — Design Document

**Date:** 2026-03-23
**Scope:** Comprehensive reliability audit and fixes across all components
**Approach:** Systematic — grouped by component, one PR per component

## Summary

Preventive reliability audit of PingCast identified 28 issues across 4 components (NATS messaging, Notifier, Checker, API). Issues range from critical (silent message loss, race conditions) to medium (missing validation, logging improvements). All fixes are organized into 5 PRs (Component 4 split into two for safer review).

---

## Component 1: NATS Messaging

### 1.1. Handle Publish Errors in HTTP Handlers

**Problem:** `PublishMonitorChanged()` errors are ignored in `server.go`. User gets 200, but checker never learns about the monitor.

**Fix:** If Publish fails, return HTTP 500 with message `"monitor saved but sync failed, retry later"`. The DB write has already succeeded, so the monitor exists but isn't being checked until the next sync.

**Files:** `internal/adapter/http/server.go` (lines 169, 244, 261, 285, 287)

### 1.1a. Fix DeleteMonitor Publish-Before-Delete Ordering

**Problem:** In `DeleteMonitor`, the event is published *before* the DB delete. If publish succeeds but DB delete fails, the checker removes a monitor that still exists.

**Fix:** Reorder: delete from DB first, then publish. If publish fails after a successful delete, the checker will eventually sync via periodic monitor reload.

**Files:** `internal/adapter/http/server.go` (lines 260-262)

### 1.2. Per-Message Context in Subscribe Closures

**Problem:** App-level context passed to NATS handler closures. On shutdown, all in-flight messages get cancelled context, causing failures and potential message loss.

**Fix:** Create per-message context with timeout instead of using the captured app context. Timeout should be tuned per subscriber:
- MonitorSubscriber: 5s (in-memory operations only — Add/Remove on scheduler map)
- AlertSubscriber: 30s (network I/O — channel delivery with retries)
- CheckSubscriber: 60s (matches existing AckWait)

```go
msgCtx, cancel := context.WithTimeout(context.Background(), timeout)
defer cancel()
handler(msgCtx, &event)
```

`consumer.Stop()` prevents new message delivery; in-flight messages complete with their own deadline.

**Files:** `internal/adapter/nats/subscriber.go` (lines 47, 98), `internal/adapter/nats/check_subscriber.go` (line 73)

### 1.3. MaxDeliver + BackOff for MonitorSubscriber

**Problem:** MonitorSubscriber has no `MaxDeliver` — failed messages retry infinitely.

**Fix:** Add `MaxDeliver: 5` and `BackOff` array matching AlertSubscriber's pattern.

**Files:** `internal/adapter/nats/subscriber.go` (lines 31-34)

### 1.4. MaxBytes on All Streams

**Problem:** No `MaxBytes` limit on MONITORS, CHECKS, ALERTS streams. If consumers stop, disk fills up.

**Fix:** Add `MaxBytes` to all three stream configs. Use different limits based on storage type:
- MONITORS stream (FileStorage): 1 GB
- ALERTS stream (FileStorage): 1 GB
- CHECKS stream (MemoryStorage): 100 MB (lower limit — RAM is more expensive than disk)

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

### 2.3. Reliable DLQ Write on Partial Failure

**Problem:** `writeToDLQ()` silently swallows its own errors (logs then returns). If the DB write fails, partial failure metadata is lost, and the alert is Ack'd anyway.

**Fix:** Retry the DLQ write itself with backoff (3 attempts, 1s/2s/4s). If all retries fail, log at Error level with full event context for manual recovery. Do NOT return error to NATS — that would cause duplicate notifications to channels that already succeeded. The DLQ write is a best-effort audit trail; re-sending to successful channels is worse than losing the audit record.

**Files:** `internal/app/alert.go` (lines 86-104, 109-123)

### 2.4. Wire DLQConsumer in Notifier

**Problem:** `DLQConsumer` is defined but never instantiated in `cmd/notifier/main.go`. After MaxDeliver=10, alerts disappear with no trace.

**Fix:** Create and start `DLQConsumer` in notifier's main, analogous to `alertSub`. It listens for max delivery advisories and persists to `failed_alerts`.

**Files:** `cmd/notifier/main.go`

### 2.5. Circuit Breaker Per-Channel

**Problem:** One CB per channel type (telegram, email). One user's misconfigured channel triggers CB for all users of that type.

**Fix:** Move CB creation to `CreateSenderWithRetry()` with key `channelType:channelID`. Store CBs in `sync.Map` inside Registry. Add periodic cleanup (every 10 minutes) to evict entries for channels that no longer exist in the DB, preventing unbounded map growth from deleted channels.

**Files:** `internal/adapter/channel/registry.go`

### 2.6. Response Body Drain

**Problem:** Telegram and Webhook senders don't drain response body before closing. Prevents HTTP connection reuse.

**Fix:** Add `io.Copy(io.Discard, resp.Body)` before `resp.Body.Close()` in all HTTP-based senders and the HTTP checker.

**Files:** `internal/adapter/telegram/sender.go`, `internal/adapter/webhook/sender.go`, `internal/adapter/checker/http.go`

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

**Problem:** `leaderScheduler.Run()` starts before `monitorSub.Subscribe()`. During the gap between Run and Subscribe, any monitor changes published to NATS are missed. The scheduler has the full set from DB load, so this is a brief window — but on a busy system with frequent monitor updates, it can lose events.

**Fix:** Reorder: `monitorSub.Subscribe()` first, then `go leaderScheduler.Run()`. Subscribe buffers NATS messages while Run starts with complete state.

**Files:** `cmd/scheduler/main.go`

### 3.2. Cleanup Goroutine: WaitGroup + Graceful Stop

**Problem:** Cleanup goroutine is not waited on during shutdown. Can access DB pool after it's closed.

**Fix:** Add `sync.WaitGroup` for cleanup goroutine. Shutdown sequence: cancel context -> `wg.Wait()` -> close DB pool.

**Files:** `cmd/scheduler/main.go`

### 3.3. Cleanup Lock Failure: Error Differentiation

**Problem:** `cleanupMutex.Lock()` failure logged as Debug. If Redis is down, cleanup silently stops. Unbounded check_results growth.

**Fix:** Differentiate error types:
- `redsync.ErrFailed` (lock held by another instance) -> `slog.Debug` (normal)
- All other errors -> `slog.Warn` (infrastructure problem)

**Files:** `cmd/scheduler/main.go`

### 3.4. HTTP Client Caching by Timeout

**Problem:** New `*http.Client` created per check with custom timeout. All share one Transport, causing connection pool fragmentation.

**Fix:** Add `sync.Map` with key `timeout_seconds -> *http.Client`. Typical timeouts are few (5, 10, 15, 30s), so map stays small.

**Files:** `internal/adapter/checker/http.go` (lines 109-116)

---

## Component 4: API

### 4.1. Atomic ProcessCheckResult

**Problem:** Read-modify-write pattern in `ProcessCheckResult()` — GetByID then UpdateStatus. Race condition causes duplicate incidents or missed recovery.

**Fix:** Use a CTE to atomically read the previous status and update in one query:
```sql
WITH prev AS (
    SELECT current_status FROM monitors WHERE id = $1
)
UPDATE monitors SET current_status = $2
WHERE id = $1
RETURNING (SELECT current_status FROM prev) AS previous_status
```

Note: A plain `RETURNING (SELECT ...)` subquery would read the post-update state, which is wrong. The CTE captures the pre-update snapshot.

**Files:** `internal/app/monitoring.go` (lines 227-270), `internal/adapter/postgres/monitor_repo.go`, sqlc queries

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

Apply both to individual handlers in `server.go` and the global error handler in `setup.go` (which also leaks `err.Error()` for non-DomainError cases). Also fix HTML pages in `pages.go` that render raw errors into templates (e.g., `RegisterSubmit`).

**Files:** `internal/adapter/http/server.go`, `internal/adapter/http/setup.go`, `internal/adapter/http/pages.go`

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

**Pre-migration step:** Check for existing duplicate unresolved incidents (`SELECT monitor_id, COUNT(*) FROM incidents WHERE resolved_at IS NULL GROUP BY monitor_id HAVING COUNT(*) > 1`). If any exist, resolve duplicates before applying the index.

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

**Fix:** Restructure the Decrypt method to capture both errors in the outer scope, then return a combined error:
```go
var primaryErr, oldErr error
plaintext, primaryErr := e.decryptWith(e.primary, data)
if primaryErr == nil {
    return plaintext, nil
}
if e.old != nil {
    plaintext, oldErr = e.decryptWith(e.old, data)
    if oldErr == nil {
        return plaintext, nil
    }
}
return nil, fmt.Errorf("decrypt failed: primary: %w; old key: %v", primaryErr, oldErr)
```

**Files:** `internal/crypto/crypto.go` (lines 63-84)

---

## Implementation Order

1. **PR 1: NATS Messaging** (issues 1.1-1.4) — foundation for all inter-service communication
2. **PR 2: Notifier** (issues 2.1-2.8) — depends on NATS fixes being in place
3. **PR 3: Checker** (issues 3.1-3.4) — depends on NATS context fix (1.2)
4. **PR 4a: API Race Conditions** (issues 4.1, 4.3, 4.6) — high-risk DB changes, needs careful testing
5. **PR 4b: API Defensive Coding** (issues 4.2, 4.4, 4.5, 4.7, 4.8, 4.9) — lower-risk, safety improvements

PRs 3, 4a, and 4b can be developed in parallel after PR 1 is merged.

## Shared Utility

The pattern of creating a detached context with timeout is repeated across Issues 1.2, 2.1, and 4.5. Extract a shared utility:
```go
// internal/xcontext/detached.go
func Detached(timeout time.Duration) (context.Context, context.CancelFunc) {
    return context.WithTimeout(context.Background(), timeout)
}
```

## Testing Strategy

- Each fix should include unit tests for the specific behavior change
- Integration tests (testcontainers) for NATS message flow, DB atomicity
- Race condition fixes verified with `go test -race`
- Shutdown behavior tested with context cancellation in tests

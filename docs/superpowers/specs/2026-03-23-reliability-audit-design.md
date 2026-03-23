# PingCast Reliability Audit — Design Document

**Date:** 2026-03-23
**Scope:** Comprehensive reliability audit and fixes across all components
**Approach:** Systematic — grouped by component, one PR per component

## Summary

Preventive reliability audit of PingCast identified 29 issues across 4 components (NATS messaging, Notifier, Checker, API). Issues range from critical (silent message loss, race conditions, hex-arch violations) to medium (missing validation, logging improvements). All fixes are organized into 6 PRs (Component 4 split into two for safer review, plus a standalone encryption overhaul). Issues 1.5, 1.6, and 2.9 address hex-arch compliance — decoupling domain models from events, moving orchestration to the app layer, and cleaning up port interfaces. Issues 1.1 and 1.1a were merged into a single issue (publish errors + ordering are handled in app layer after 1.6).

---

## Component 1: NATS Messaging

### 1.1. Handle Publish Errors + Correct Ordering in App Layer

**Problem:** `PublishMonitorChanged()` errors are ignored, and in `DeleteMonitor` the event is published *before* the DB delete. Both problems exist in `server.go` but will be **relocated to app layer** by Issue 1.6.

**Fix (applied in app layer after 1.6):**
- If Publish fails after DB write, return wrapped `ErrEventPublishFailed`. HTTP adapter maps it to 500 with `"monitor saved but sync failed, retry later"`.
- `DeleteMonitor`: delete from DB first, then publish. If publish fails after successful delete, checker syncs via periodic reload.
- `TogglePause`: two publish paths (pause/resume) — each needs individual error handling.

**Files:** `internal/app/monitoring.go` (after 1.6 relocation)

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

### 1.5. Event DTO Layer — Decouple Events from Domain Models

**Problem:** `MonitorEventPublisher.PublishMonitorChanged()` accepts `*domain.Monitor` directly. The full domain model is serialized to JSON and travels through NATS. This creates tight coupling between publishers and subscribers — any change to `domain.Monitor` fields breaks deserialization on the other side. Event schema versioning is impossible.

**Fix:** Introduce event DTOs in `internal/port/eventbus.go`:
```go
// MonitorChangedEvent is the event envelope — decoupled from domain.Monitor.
type MonitorChangedEvent struct {
    Action    domain.MonitorAction `json:"action"`
    MonitorID uuid.UUID            `json:"monitor_id"`
    // Only fields that subscribers actually need:
    Name           string              `json:"name,omitempty"`
    Type           domain.MonitorType  `json:"type,omitempty"`
    CheckConfig    json.RawMessage     `json:"check_config,omitempty"`
    IntervalSeconds int               `json:"interval_seconds,omitempty"`
    AlertAfterFailures int            `json:"alert_after_failures,omitempty"`
    IsPaused       bool               `json:"is_paused"`
}
```

Update port interface:
```go
type MonitorEventPublisher interface {
    PublishMonitorChanged(ctx context.Context, event MonitorChangedEvent) error
}

type MonitorEventSubscriber interface {
    Subscribe(ctx context.Context, handler func(ctx context.Context, event MonitorChangedEvent) error) error
    Stop()
}
```

The mapping `domain.Monitor → MonitorChangedEvent` happens at the call site (app layer), not inside the adapter. This allows event schema evolution without touching domain models.

**Files:** `internal/port/eventbus.go`, `internal/adapter/nats/subscriber.go`, `internal/adapter/nats/publisher.go`, `internal/app/monitoring.go`

### 1.6. Move Publish to App Layer

**Problem:** `PublishMonitorChanged()` is called in HTTP handlers (`server.go` lines 169, 244, 261, 285, 287). Publishing an event when a monitor changes is **business logic** ("notify the system about a state change"), not HTTP adapter concern. This violates hex-arch: the adapter makes orchestration decisions (publish after DB write, handle publish errors, decide event payload).

**Fix:** Move event publishing into `MonitoringService` methods. Each method that mutates a monitor publishes the corresponding event:
- `CreateMonitor()` → publishes `ActionCreate` event after successful DB write
- `UpdateMonitor()` → publishes `ActionUpdate` event
- `DeleteMonitor()` → publishes `ActionDelete` event (after DB delete, per 1.1a)
- `TogglePause()` → publishes `ActionPause` or `ActionResume`

This requires injecting `MonitorEventPublisher` into `MonitoringService` (it already has `AlertEventPublisher`). The HTTP handler becomes a thin adapter — it parses input, calls the service, maps the response.

Error handling: if publish fails after DB write, `MonitoringService` returns a wrapped error (e.g., `ErrEventPublishFailed`). The HTTP adapter maps it to 500. This keeps error classification in the app layer.

Remove `events port.MonitorEventPublisher` from `Server` struct — it no longer belongs there.

**Files:** `internal/app/monitoring.go`, `internal/adapter/http/server.go`, `cmd/api/main.go` (wiring)

---

## Component 2: Notifier

### 2.1. SMTP Sender: Context + Timeout

**Problem:** `Send(_ context.Context, ...)` explicitly ignores context. `gosmtp.SendMail` has no timeout — can hang indefinitely.

**Fix:** Replace `gosmtp.SendMail` with `net/smtp` using explicit dial timeout and context cancellation. Use the context parameter instead of discarding it.

**Files:** `internal/adapter/smtp/sender.go`

### 2.2. Parallel Channel Delivery

**Problem:** Channels are sent sequentially. N channels x 10s timeout = catastrophic backpressure. Can exceed NATS AckWait.

**Fix:** Use `errgroup.Group` with per-channel goroutines. Each channel gets its own `context.WithTimeout(ctx, 10*time.Second)`. Overall group timeout: 30s. Channels that don't complete within the 30s group deadline will fail with context deadline exceeded and be captured in the DLQ via Issue 2.3's partial-failure handling.

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

**Fix:** Move CB creation to `CreateSender()` (renamed per Issue 2.9) with key `channelType:channelID`. Store CBs in `sync.Map` inside Registry. Add periodic cleanup (every 10 minutes) to evict entries for channels that no longer exist in the DB, preventing unbounded map growth from deleted channels. This requires injecting a channel-existence-check function (e.g., `func(channelID uuid.UUID) bool`) into the Registry to avoid a hard dependency on `port.ChannelRepo`.

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

### 2.9. Rename `CreateSenderWithRetry` → `CreateSender` in Port

**Problem:** The port interface `ChannelRegistry` exposes `CreateSenderWithRetry()` — leaking the retry/circuit-breaker infrastructure detail into the domain contract. The app layer shouldn't know (or care) that the sender wraps retries.

**Fix:** Rename to `CreateSender()` in `port/channel.go`:
```go
type ChannelRegistry interface {
    Get(channelType domain.ChannelType) (ChannelSenderFactory, error)
    CreateSender(channelType domain.ChannelType, config json.RawMessage) (AlertSender, error)
    Types() []ChannelTypeInfo
    ValidateConfig(channelType domain.ChannelType, config json.RawMessage) error
}
```

Update all call sites in `internal/app/alert.go`. The adapter implementation in `registry.go` continues to wrap with retry+CB internally — that's invisible to the port consumer.

**Files:** `internal/port/channel.go`, `internal/app/alert.go`, `internal/adapter/channel/registry.go`

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
    SELECT current_status FROM monitors WHERE id = $1 AND deleted_at IS NULL
)
UPDATE monitors SET current_status = $2
WHERE id = $1 AND deleted_at IS NULL
RETURNING (SELECT current_status FROM prev) AS previous_status
```

Note: A plain `RETURNING (SELECT ...)` subquery would read the post-update state, which is wrong. The CTE captures the pre-update snapshot.

**Files:** `internal/app/monitoring.go` (lines 227-270), `internal/adapter/postgres/monitor_repo.go`, sqlc queries

### 4.2. Nil Check on UserFromCtx

**Problem:** `ListAPIKeys` and potentially other handlers don't nil-check `UserFromCtx()`. Panic if auth middleware fails.

**Fix:** The three handlers missing nil checks are: `ListAPIKeys`, `CreateAPIKey`, `RevokeAPIKey`. Extract a helper `requireUser(c *fiber.Ctx) (*domain.User, error)` that returns 401 on nil, and use it in all handlers that need auth.

**Files:** `internal/adapter/http/server.go`

### 4.3. Atomic TogglePause

**Problem:** Read-modify-write in `TogglePause()`. Concurrent requests can cause lost update.

**Fix:** Single SQL that returns the full monitor row (needed by Issue 1.6 to construct the event DTO):
```sql
UPDATE monitors SET is_paused = NOT is_paused
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
RETURNING id, name, type, check_config, interval_seconds, alert_after_failures, is_paused, current_status, user_id
```

The repo method `TogglePause(ctx, monitorID, userID) (*domain.Monitor, error)` returns the full monitor with the toggled state. The app layer uses the returned `IsPaused` to decide which event to publish (per 1.6).

**Files:** `internal/app/monitoring.go`, `internal/adapter/postgres/monitor_repo.go`, sqlc queries

### 4.4. Error Sanitization

**Problem:** Raw `err.Error()` returned to clients. Leaks internal info (user enumeration, schema details).

**Fix:** Map domain errors to client-safe messages:
- `ErrUserExists` -> `"email already registered"`
- `ErrNotFound` -> `"resource not found"`
- Everything else -> `"internal error"` (details in logs only)

**Prerequisite:** Introduce `ErrUserExists` sentinel error in `internal/domain/errors.go`. Currently the auth service wraps the raw database unique constraint violation; the Register handler can't classify it. The auth service should catch the postgres unique violation and return `ErrUserExists`.

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

### ~~4.9. Crypto Error Reporting for Key Rotation~~ — REMOVED

Originally proposed to report both primary and old key errors on decrypt failure. After code review: both errors are identical (`cipher: message authentication failed` from AES-GCM), so combining them adds no diagnostic value. Not worth the code change.

### 4.10. Replace Encryptor with Key-Versioned Encryption + Hex-Arch Port

**Problem (scalability):** Current `crypto.Encryptor` supports max 2 keys (primary + old). No key ID in ciphertext — decryption brute-forces keys. No re-encryption mechanism — old key can never be retired. Repos depend on concrete `*crypto.Encryptor` type instead of a port interface.

**Fix:** Replace with key-versioned AES-256-GCM encryption and proper hex-arch port.

#### Ciphertext format

```
Binary (pre-base64): [1-byte version][12-byte nonce][ciphertext + 16-byte GCM tag]
AEAD associated data: []byte{version} (binds version to ciphertext, prevents downgrade)
```

Version `0x00` reserved — used to detect legacy unversioned ciphertext during migration.

#### Port interface (`internal/port/crypto.go`)

```go
// Cipher encrypts/decrypts sensitive data with key versioning.
type Cipher interface {
    Encrypt(ctx context.Context, plaintext []byte) (string, error)
    Decrypt(ctx context.Context, encrypted string) ([]byte, error)
    NeedsReEncryption(encrypted string) bool
}
```

#### NoOp implementation for disabled encryption

```go
type NoOpCipher struct{}
func (n NoOpCipher) Encrypt(_ context.Context, p []byte) (string, error) { return string(p), nil }
func (n NoOpCipher) Decrypt(_ context.Context, s string) ([]byte, error) { return []byte(s), nil }
func (n NoOpCipher) NeedsReEncryption(_ string) bool { return false }
```

#### Key-versioned Encryptor (`internal/crypto/encryptor.go`)

```go
type Encryptor struct {
    primary byte              // current key version
    keys    map[byte][32]byte // version -> raw AES-256 key
}

func (e *Encryptor) Encrypt(ctx context.Context, plaintext []byte) (string, error) {
    // [1-byte version][12-byte nonce][ciphertext+tag]
    // AD = []byte{version}
}

func (e *Encryptor) Decrypt(ctx context.Context, encoded string) ([]byte, error) {
    // base64 decode → read version byte → lookup key by version → decrypt
    // O(1) key lookup, no brute-force
}

func (e *Encryptor) NeedsReEncryption(encoded string) bool {
    // base64 decode → check if version byte != e.primary
}
```

#### Configuration

Replace single `ENCRYPTION_KEY` / `ENCRYPTION_KEY_OLD` with:
```
ENCRYPTION_KEYS=1:base64key1,2:base64key2,3:base64key3
ENCRYPTION_PRIMARY_VERSION=3
```

Backward-compatible: if `ENCRYPTION_KEY` is set (old format), treat as version 1.

#### Repos depend on port, not concrete type

```go
// Before:
type MonitorRepo struct {
    enc *crypto.Encryptor  // concrete type, nil = disabled
}

// After:
type MonitorRepo struct {
    cipher port.Cipher  // port interface, NoOpCipher if disabled
}
```

Removes dual constructor (`NewMonitorRepo` / `NewMonitorRepoWithEncryption`) — single constructor always takes `port.Cipher`.

#### Re-encryption strategy

1. **On every Update:** `Encrypt()` always uses primary key, so data migrates naturally on write
2. **Batch CLI command:** `pingcast reencrypt --table monitors --column check_config` for bulk migration
3. **Detection:** `NeedsReEncryption()` checks version byte vs primary

After batch migration completes, old keys can be safely removed from config.

#### Migration from current format

One-time migration job:
1. Read all encrypted fields
2. Detect unversioned ciphertext (try decrypt with current Encryptor's primary/old keys)
3. Re-encrypt with new key-versioned format (version byte prepended)
4. Write back

Run as a DB migration step or CLI command before switching to the new Encryptor.

**Files:** `internal/crypto/encryptor.go` (rewrite), `internal/port/crypto.go` (new), `internal/adapter/postgres/monitor_repo.go`, `internal/adapter/postgres/channel_repo.go`, `internal/config/config.go`, `cmd/api/main.go`

---

## Implementation Order

1. **PR 1: NATS Messaging + Event Architecture** (issues 1.1-1.6) — foundation: event DTO layer, publish moved to app layer, error handling, stream limits
2. **PR 2: Notifier** (issues 2.1-2.9) — depends on NATS fixes; includes port rename `CreateSenderWithRetry` → `CreateSender`
3. **PR 3: Checker** (issues 3.1-3.4) — depends on NATS context fix (1.2) and event DTO (1.5)
4. **PR 4a: API Race Conditions** (issues 4.1, 4.3, 4.6) — high-risk DB changes, needs careful testing
5. **PR 4b: API Defensive Coding** (issues 4.2, 4.4, 4.5, 4.7, 4.8) — lower-risk, safety improvements
6. **PR 5: Encryption Overhaul** (issue 4.10) — key-versioned encryption, hex-arch port, migration

PR 5 is independent of all other PRs and can be developed in parallel at any point. PRs 3, 4a, and 4b can be developed in parallel after PR 1 is merged.

Note: PR 1 is now the largest PR due to issues 1.5 and 1.6 (event DTO + publish relocation). These changes touch port interfaces, app layer, HTTP adapter, and NATS adapter. Consider splitting into PR 1a (event DTO + publish relocation) and PR 1b (error handling + stream limits) if the diff is too large for review.

## Shared Utility

The pattern of creating a detached context with timeout is repeated across Issues 1.2, 2.1, and 4.5. Extract a shared utility that preserves OTel span context for tracing correlation:
```go
// internal/xcontext/detached.go
func Detached(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
    ctx := context.Background()
    // Propagate trace span so detached operations are correlated in Grafana
    if span := trace.SpanFromContext(parent); span.SpanContext().IsValid() {
        ctx = trace.ContextWithSpan(ctx, span)
    }
    return context.WithTimeout(ctx, timeout)
}
```

## Testing Strategy

- Each fix should include unit tests for the specific behavior change
- Integration tests (testcontainers) for NATS message flow, DB atomicity
- Race condition fixes verified with `go test -race`
- Shutdown behavior tested with context cancellation in tests

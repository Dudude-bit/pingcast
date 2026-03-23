# PingCast — Comprehensive Refactoring Design

**Date:** 2026-03-23
**Status:** Draft
**Context:** SaaS-ready deep refactoring. Dev stage, no backwards compatibility constraints. Multi-replica scalability required.

---

## Overview

Full codebase audit of PingCast revealed ~55 issues across security, error handling, data integrity, performance, architecture, observability, testing, and code quality. This design addresses all of them in priority order (P0–P7), with every decision validated for multi-replica SaaS scalability.

### New Infrastructure Dependencies

- **Redis** — rate limiting, distributed locks, session storage, host limiter
- **Grafana + Loki + Tempo** — observability stack (dev docker-compose)
- **OpenTelemetry SDK** — distributed tracing and metrics

---

## P0 — Security

### 0.1 Webhook Signature Bypass

**Problem:** `internal/adapter/http/webhook.go` — if `lemonSqueezySecret` is empty, all webhooks are accepted without signature verification. Empty secret is the default.

**Fix:** If secret is empty, handler returns `503 "webhook processing disabled"`. When secret is set, signature validation is mandatory. Fail-closed, not fail-open.

### 0.2 Empty Secrets at Startup

**Problem:** `cmd/api/main.go` registers Telegram and SMTP channel factories with empty credentials. They fail at runtime when attempting to send.

**Fix:** Register channels only if credentials are provided (matching the pattern already used in `cmd/notifier/main.go`). Log warning at startup for each disabled channel type. Validate that at least one notification channel is active in notifier.

### 0.3 Rate Limiter — Redis-Based

**Problem:** Current in-memory rate limiter has TOCTOU race condition and doesn't work across replicas.

**Fix:** Replace with Redis sliding window counter.
- **Login/register:** 5 requests / 15 minutes per IP
- **API CRUD (monitors, channels):** 60 requests / minute per user
- **Public endpoints (status page):** 120 requests / minute per IP
- Works across all API replicas
- New adapter: `internal/adapter/redis/ratelimit.go`

### 0.4 Redis Infrastructure

- Add Redis to `docker-compose.yml` with healthcheck
- `REDIS_URL` env var in config for all services
- Connection pool: `internal/adapter/redis/client.go`

---

## P1 — Silent Error Handling

**Principle:** No `_` for errors anywhere. Either `return error` or `slog.Warn/Error` with context. Enforce via `errcheck` linter in CI.

### 1.1 Alert Handler (`internal/app/alert.go:30-33`)

`ListForMonitor()` and `ListByUserID()` errors silently ignored. If DB is down, alerts silently don't send.

**Fix:** Return error. NATS subscriber will Nak and retry.

### 1.2 Session Touch (`internal/app/auth.go:93`)

`Touch()` error ignored — sessions don't extend, users get unexpected logouts.

**Fix:** Log as warning (don't block the request, but visible in logs).

### 1.3 JSON Unmarshal in server.go (lines 334, 357, 381, 547)

`json.Unmarshal(m.CheckConfig, &checkConfig)` error ignored in 4 places. Corrupted config returns null to API.

**Fix:** Extract to `Monitor.ParseCheckConfig() (map[string]any, error)` on domain model. Return 500 on failure.

### 1.4 GetUptime / ListByMonitorID (`internal/app/monitoring.go:271, 292-293`)

Dashboard shows 0% uptime instead of error when queries fail.

**Fix:** Return error up the stack. UI shows "unable to calculate" instead of false data.

### 1.5 Checker Monitor Load (`cmd/checker/main.go:96`)

`activeMonitors, _ := monitorRepo.ListActive(ctx)` — checker starts with empty monitor list if DB is unavailable.

**Fix:** `os.Exit(1)` if initial monitor load fails.

---

## P2 — Data Integrity

### 2.1 Transactional Channel Binding

**Problem:** `internal/adapter/http/pages.go:327-333` — monitor created, then channels bound in separate queries. Partial failure leaves monitor without channels.

**Fix:** `MonitoringService.CreateMonitor` accepts channel IDs list. All operations in a single `pgx.Tx`. Rollback on any failure. Extend sqlc-generated `DBTX` interface usage for transaction support in repositories.

### 2.2 Channel Repo Transactions

**Problem:** `internal/adapter/postgres/channel_repo.go` — binding/unbinding without transaction.

**Fix:** `BindChannels(ctx, tx, monitorID, channelIDs)` — atomically deletes old bindings + inserts new ones within provided transaction.

### 2.3 Input Validation

**Problem:** No validation on interval, alert_after_failures, names. Inconsistent logic between Create and Update.

**Fix:** Unified `domain.Validate()`:
- Interval: min 30s, max 24h
- AlertAfterFailures: 1–100
- Name: 1–255 chars, trimmed
- URL/Host: format validation per monitor type

### 2.4 Missing Database Indexes (migration `009_add_missing_indexes.sql`)

```sql
CREATE INDEX idx_check_results_monitor_created ON check_results (monitor_id, created_at DESC);
CREATE INDEX idx_incidents_monitor_started ON incidents (monitor_id, started_at DESC);
CREATE INDEX idx_sessions_expires_at ON sessions (expires_at);
CREATE INDEX idx_monitor_channels_monitor ON monitor_channels (monitor_id);
CREATE INDEX idx_monitor_channels_channel ON monitor_channels (channel_id);
```

### 2.5 Soft Delete

**Problem:** Hard delete loses data permanently. No recovery, no audit trail.

**Fix:**
- Add `deleted_at TIMESTAMPTZ` to `users`, `monitors`, `channels`
- All SELECT queries: `WHERE deleted_at IS NULL`
- Delete operations: `UPDATE SET deleted_at = NOW()`
- Partial index: `CREATE INDEX ... WHERE deleted_at IS NULL`
- `check_results`, `incidents`, `sessions` — remain hard delete (ephemeral data, handled by retention policy)
- Physical cleanup: records with `deleted_at > 30 days` purged by scheduled job with Redis distributed lock

### 2.6 Cascade Verification

Verify `ON DELETE CASCADE` on all foreign keys. With soft delete, cascading behavior changes:
- Soft-deleting a user → soft-deletes their monitors → soft-deletes channel bindings
- Application-level cascade, not DB-level (since soft delete is an UPDATE, not DELETE)

### 2.7 Sessions → Redis

**Problem:** Sessions in Postgres = DB query on every HTTP request. Doesn't scale with replicas.

**Fix:** Move sessions to Redis with TTL. Remove `sessions` table from Postgres. Session lookup becomes O(1) Redis GET. Auto-expiry via Redis TTL replaces manual cleanup.

### 2.8 Distributed Lock for Cleanup

Soft delete cleanup + data retention cleanup: use Redis `SET NX EX` lock. One process runs cleanup, others skip.

---

## P3 — Performance

### 3.1 Batch Uptime Queries

**Problem:** N+1 queries in `GetMonitorDetail` — per-monitor uptime queries for 24h, 7d, 30d + incidents list.

**Fix:** `GetUptimeBatch(ctx, monitorIDs, []Duration)` — single SQL with `GROUP BY monitor_id`, window functions for multi-period uptime calculation.

### 3.2 Uptime Aggregation Table

**Problem:** Uptime calculated from raw `check_results`. At scale (1000 monitors × 1 check/min = 43M rows/month), this is unsustainable.

**Fix:** `monitor_uptime_hourly(monitor_id, hour, total_checks, successful_checks)` table. Checker updates incrementally after each check. Uptime calculated from hourly aggregates, not raw data.

### 3.3 Connection Pool Tuning

**Problem:** pgxpool defaults (4 conns) vs 100 worker pool size.

**Fix:** `MaxConns` from config. Default = `max(4, numCPU * 2)`. Checker gets separate pool size config tied to worker count.

### 3.4 Host Limiter — Redis Semaphore

**Problem:** In-memory host limiter per instance. Multiple replicas bypass limits.

**Fix:** Redis semaphore per host:
- Key: `hostlimit:{host}` — `INCR/DECR` with TTL
- TTL = check timeout (auto-release if worker crashes)
- Configurable per-host concurrency limit (default: 3)
- All checker replicas share the same counter

### 3.5 Worker Pool Backpressure

**Problem:** Submit blocks or drops tasks silently when pool is overwhelmed.

**Fix:** `Submit() bool` — returns accepted/dropped. Scheduler logs dropped checks. Metric `checks_dropped_total` for alerting on overload.

### 3.6 HTTP Checker Body Optimization

**Problem:** Always reads 1MB body, even for status-only checks.

**Fix:** If keyword not configured — read headers only (0 bytes body). If configured — limit from per-monitor config, default 1MB.

---

## P4 — Architecture

### 4.1 Graceful Shutdown with Timeout

**Problem:** `fiberApp.Shutdown()` without timeout. Checker doesn't wait for in-flight checks.

**Fix:** `context.WithTimeout(ctx, 30*time.Second)` for all services. API waits for current requests. Checker waits for current checks. Notifier waits for current alert sends. Force kill after timeout.

### 4.2 Circuit Breaker for External Services

**Problem:** Telegram/SMTP/Webhook senders retry every failed alert. 1000 monitors = 1000 failed HTTP calls when service is down.

**Fix:** Circuit breaker per channel type (open/half-open/closed). After N consecutive errors — circuit opens, requests rejected without real call for configurable period. In-process implementation (each notifier replica independent).

### 4.3 Retry with Exponential Backoff

**Problem:** Senders don't retry. NATS retry is the only fallback with fixed backoff.

**Fix:** Sender retries up to 3 times with exponential backoff (1s, 2s, 4s) before returning error to NATS. NATS retry is second-level protection.

### 4.4 Dead Letter Queue for Alerts

**Problem:** After NATS retries exhausted, messages lost forever. No visibility.

**Fix:**
- NATS consumer with `MaxDeliver` + DLQ stream
- Failed alerts saved to `failed_alerts(id, event, error, attempts, created_at)` table
- UI: "3 alerts failed to deliver" with retry button
- Metric: `alerts_dead_lettered_total`

### 4.5 NATS Streams Retention

**Problem:** 24h TTL — if notifier is down >24h, alerts lost.

**Fix:** Increase to 72h. With DLQ this is less critical but provides buffer.

### 4.6 Scheduler Overload Awareness

**Problem:** Scheduler blindly dispatches even when worker pool is overwhelmed.

**Fix:** Scheduler receives queue depth feedback from worker pool. If queue > 80% — skip tick, log warning. Metric `scheduler_skipped_ticks_total`.

### 4.7 Checker Scaling — NATS Work Queue Architecture

**Problem:** Each checker replica loads ALL monitors and checks them all. 3 replicas = 3x duplicate checks. Not scalable.

**Fix:** Separate scheduling from checking:

- **Scheduler**: single leader-elected process (Redis lock with TTL). Publishes `check.run.{monitor_id}` events to NATS stream at configured intervals.
- **Checker workers**: stateless consumers from NATS consumer group. Pick up tasks, execute checks, write results.

**Benefits:**
- Replica dies → pending checks auto-redelivered by NATS
- Scale → add checker instances, NATS balances load
- Zero per-replica config — all instances identical
- Scheduler leader dies → another instance acquires Redis lock within seconds
- Clean separation: scheduling ≠ checking

**Load:** 1000 monitors × 1 check/min = ~17 msg/sec — trivial for NATS.

---

## P5 — Observability

### 5.1 OpenTelemetry — Distributed Tracing

- OTel SDK in all three services (API, Checker, Notifier)
- Auto-instrumentation: Fiber HTTP middleware, pgxpool, NATS, Redis
- Traces propagated through NATS message headers — full chain from HTTP request to alert delivery
- Export to Tempo

### 5.2 Prometheus Metrics

Exported via OTel metrics exporter (single SDK for traces + metrics):
- **HTTP:** `http_requests_total`, `http_request_duration_seconds` (by route, method, status)
- **Checker:** `checks_total` (by type, status), `check_duration_seconds`, `checks_dropped_total`
- **Alerts:** `alerts_sent_total` (by channel type, status), `alerts_failed_total`, `alerts_dead_lettered_total`
- **Business:** `monitors_active_total`, `incidents_open_total`
- **Infrastructure:** `redis_pool_active_connections`, `nats_pending_messages`, `pg_pool_active_connections`
- Endpoint: `GET /metrics`

### 5.3 Grafana + Loki + Tempo (Dev Stack)

Add to `docker-compose.yml`:
- **Grafana** — dashboards, alerts
- **Loki** — centralized logs from all replicas via Docker log driver
- **Tempo** — distributed traces

Trace ID auto-linked between logs and traces in Grafana.

### 5.4 Structured Logging via OTel

`slog` handler enriches every log entry with `trace_id`, `span_id`. In Grafana: click log → jump to trace → see full request path across services.

### 5.5 Health/Readiness Endpoints

- `GET /healthz` — Postgres ping, Redis ping, NATS connection status
- `GET /readyz` — service ready to accept traffic
- Kubernetes-ready for liveness/readiness probes

### 5.6 Query Logging

pgxpool OTel tracer — every SQL query as a span in trace. Slow queries (>100ms) logged as `slog.Warn` with query text and duration.

---

## P6 — Testing

### 6.1 HTTP Handler Tests

Two levels:
- **Unit:** table-driven tests with mocked service layer via `httptest`. Coverage: auth middleware, rate limiting, input validation, error responses, cookie handling.
- **Integration:** same endpoints with real Postgres/Redis/NATS via testcontainers. Catches mock/reality drift.

### 6.2 Integration Tests — Full Pipeline

Complete cycle: create monitor → check → incident → alert → notification.
Testcontainers for Postgres, Redis, NATS — real dependencies. Located in `tests/integration/`.

### 6.3 Repository Tests

Real Postgres via testcontainers. Coverage: soft delete, cascades, indexes, transactions, edge cases (duplicates, conflicts, concurrent access).

### 6.4 Alert Pipeline Tests

- `AlertService.Handle`: all branches — channels found/not found, sender failed, circuit breaker open, retry exhausted
- DLQ: message lands in `failed_alerts` after retries exhausted
- Channel senders: `httptest` mock server for external APIs (webhook, telegram, smtp)

### 6.5 Checker Tests

Expand existing `http_test.go`, `scheduler_test.go` with edge cases: timeouts, invalid hosts, keyword matching, backpressure, dropped checks.

### 6.6 Load Tests

k6 scripts for key scenarios: dashboard load, monitor CRUD, status page. Baseline benchmarks for performance regression detection.

### 6.7 CI Pipeline (GitHub Actions)

- **On every PR:** `golangci-lint` + `go test ./...` + testcontainers integration tests
- **Separate job:** integration tests (longer running, required to pass)
- **On schedule / by label:** load tests
- **Linters:** `errcheck`, `govet`, `staticcheck`, `gosec`

### 6.8 API Contract Tests

OpenAPI spec as source of truth. Tests validate that actual handler responses match the spec. `oapi-codegen` (already in stack) generates response validators.

---

## P7 — Code Quality

### 7.1 Deduplicate JSON Unmarshal

4+ identical `json.Unmarshal(m.CheckConfig, &checkConfig)` calls with ignored errors.

**Fix:** `Monitor.ParseCheckConfig() (map[string]any, error)` method on domain model. Single source of truth.

### 7.2 Deduplicate Enum Validation

`domain/channel.go` and `domain/monitor.go` — manual `Valid()` copy-paste.

**Fix:** `ValidateEnum[T comparable](value T, allowed []T) bool` generic helper, or `go generate` with stringer/enumer.

### 7.3 Configurable Values (Global vs Per-Monitor)

**Global config (env vars):**
- Worker pool size, retention period (default 90 days), session TTL
- Default check timeout, default body limit

**Per-monitor config (in CheckConfig, user-configurable):**
- HTTP timeout, body limit, custom User-Agent
- DNS resolver timeout
- TCP connect timeout

### 7.4 Domain Error Types

**Problem:** "monitor not found" returned for both non-existent and unauthorized access.

**Fix:** Domain errors: `ErrNotFound`, `ErrForbidden`, `ErrValidation`. HTTP layer maps to status codes (404, 403, 422).

### 7.5 Structured API Error Responses

```json
{
  "error": {
    "code": "MONITOR_NOT_FOUND",
    "message": "Monitor with given ID does not exist"
  }
}
```

Domain errors map to both HTTP status and machine-readable `code`. Documented in OpenAPI spec.

### 7.6 Checker Registry Error Returns

`Target()`, `Host()` return `""` on error — callers can't distinguish error from empty value.

**Fix:** `Target() (string, error)`, `Host() (string, error)`.

### 7.7 Telegram Markdown Escaping

Alert event data interpolated into Markdown without escaping. Monitor names with `*`, `` ` `` break formatting.

**Fix:** Escape special characters before sending, or use HTML parse mode.

### 7.8 golangci-lint Config

`.golangci.yml` with: `errcheck`, `govet`, `staticcheck`, `gosec`. Runs in CI on every PR.

---

## Infrastructure Summary

### New docker-compose services

| Service | Purpose |
|---------|---------|
| Redis | Rate limiting, sessions, distributed locks, host limiter |
| Grafana | Dashboards, alerting |
| Loki | Centralized log aggregation |
| Tempo | Distributed trace storage |

### New Go dependencies

| Dependency | Purpose |
|------------|---------|
| `go-redis/redis` | Redis client |
| `go.opentelemetry.io/otel` | Distributed tracing + metrics |
| `go.opentelemetry.io/contrib` | Auto-instrumentation (Fiber, pgx, NATS) |
| `prometheus/client_golang` | Metrics endpoint (via OTel exporter) |

### New tables / migrations

| Migration | Changes |
|-----------|---------|
| `009_add_missing_indexes.sql` | Indexes on check_results, incidents, sessions, monitor_channels |
| `010_add_soft_delete.sql` | `deleted_at` column + partial indexes on users, monitors, channels |
| `011_create_uptime_hourly.sql` | `monitor_uptime_hourly` aggregation table |
| `012_create_failed_alerts.sql` | DLQ table for failed alert deliveries |
| `013_drop_sessions_table.sql` | Remove sessions from Postgres (moved to Redis) |

### Architectural Changes

- Checker split: Scheduler (leader-elected) + stateless workers (NATS consumer group)
- Sessions: Postgres → Redis
- Rate limiting: in-memory → Redis sliding window
- Host limiting: in-memory semaphore map → Redis semaphore

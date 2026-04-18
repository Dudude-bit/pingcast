# C1 — API contract tests (black-box, TDD)

Status: draft (pending user review)
Date: 2026-04-18
Sub-project of: C (integration test system). Followed by C2 (async
flows), C3 (Playwright journeys), C4 (security edge-cases).

## Purpose

Build a black-box integration test suite over the Go HTTP API that
asserts the *intended* behavior of the system, not the current
implementation. When a test fails, the default outcome is that the code
is fixed to match the behavior-spec — not the other way around.

This is the foundation sub-project for C: it establishes the harness
(testcontainers, fixtures, fakes, clock/rng injection) that C2/C3/C4
will reuse.

## Source of truth

Tests assert against, in order of priority:

1. **OpenAPI spec** (`api/openapi.yaml`) — request/response shapes and
   HTTP statuses. Mechanical contract.
2. **Behavior-spec** (`docs/superpowers/specs/2026-04-18-C1-api-behavior-spec.md`)
   — semantic contract: error envelope, tenant isolation, rate limits,
   validation rules, webhook signatures, per-endpoint business rules.
   Authoritative when OpenAPI is silent.
3. Domain invariants from `internal/domain/` (e.g. `PlanFree.MonitorLimit = 5`)
   — only as defaults for the behavior-spec; spec may override and code
   gets changed.

Tests **MUST NOT** be derived from handler implementations. If a test
passes only because the test matches a handler quirk, the test is
wrong.

## Scope

27 HTTP endpoints total — 22 in OpenAPI + 5 out-of-OpenAPI:

| Group | Endpoints |
|---|---|
| Auth (session) | `POST /api/auth/register`, `POST /api/auth/login`, `POST /api/auth/logout` |
| Monitors | `GET /api/monitor-types`, `GET/POST /api/monitors`, `GET/PUT/DELETE /api/monitors/{id}`, `POST /api/monitors/{id}/pause` |
| Channels | `GET /api/channel-types`, `GET/POST /api/channels`, `PUT/DELETE /api/channels/{id}` |
| Monitor-channel | `POST /api/monitors/{id}/channels`, `DELETE /api/monitors/{id}/channels/{channelId}` |
| API keys | `GET/POST /api/api-keys`, `DELETE /api/api-keys/{id}` |
| Public status | `GET /api/status/{slug}` |
| Health | `GET /health`, `GET /healthz`, `GET /readyz` |
| Logout (page POST) | `POST /logout` |
| Webhooks | `POST /webhook/lemonsqueezy`, `POST /webhook/telegram/:token` |

Per endpoint: happy path + auth failures (missing, expired, revoked,
cross-tenant) + validation errors + not-found + conflict where
applicable. Target ~95 tests.

**Out of scope for C1:**
- Async flows: scheduler → worker → incident → notifier (→ C2)
- Full user journeys via browser (→ C3)
- Security edge-cases: session fixation, CSRF, key rotation (→ C4)

## Findings surfaced during scope analysis

These are code-level inconsistencies that tests will expose. Each must
be resolved by the behavior-spec (not the test adapting to code):

1. **Inconsistent HTTP status for validation.** `ErrorHandler` maps
   `domain.ErrValidation` → 422; direct handler returns use 400
   (e.g. `server.go:515` — "invalid request body"). Spec picks one
   convention; code changes to match.
2. **Two incompatible error envelopes.** Domain errors:
   `{"error":{"code","message"}}`. Handler errors:
   `apigen.ErrorResponse{Error: <string>}`. Spec picks one; code
   changes.
3. **Raw 500s with hardcoded strings** in `ListChannels` (`server.go:499`)
   bypass `httperr.ClassifyHTTPError`. Spec mandates single classifier.
4. **No `GET /api/channels/{id}`** (OpenAPI + code). Asymmetric with
   monitors. Spec decides: add endpoint (code change) or formalize
   absence (test asserts 404 and documents it).
5. **Top-level `POST /logout`** (`setup.go:103`) — page handler from
   pre-Next.js SSR era. Spec decides: dead code (remove) or keep (test
   covers it).

## Harness architecture

**Containers (package-level, `TestMain`):** single Postgres 16 + Redis
7 + NATS JetStream per test binary, started once via
testcontainers-go. Migrations run via goose on boot. Shared across all
tests.

**Isolation between tests:** `TRUNCATE` all user-data tables +
`FLUSHDB` on Redis + random NATS subject prefix per test. Faster than
transaction rollback; survives service-internal commits.

**API server:** in-process via `fiber.App.Test(req)` — no TCP
listener. DI composition lives in `harness.NewApp(...)` which wires
real Postgres/Redis/NATS adapters plus test doubles for
SMTP/Telegram/Webhook and an injectable clock/rng.

**Fakes for outbound network:**
- `FakeSMTP` — captures messages in-memory; assertion API.
- `FakeTelegram` — `httptest.NewServer` impersonating
  `api.telegram.org/sendMessage`.
- `FakeWebhookSink` — `httptest.NewServer` capturing POSTs with HMAC
  validation helpers.

These are scaffolded in C1 but only exercised starting in C2. Their
presence in C1 ensures the harness doesn't need a rewrite.

**Clock + RNG injection:** current code calls `time.Now()` directly in
`app/monitoring.go`, `app/auth.go`. C1 adds a `Clock` port
(`port/clock.go`) and `Random` port with production impls being the
stdlib and test impls being deterministic. This is an explicit
implementation change driven by test needs — legitimate in TDD.

**Fixtures — via API only.** `harness.Session` is a cookie-jar that
speaks HTTP to the in-process server. Helpers: `RegisterAndLogin`,
`MustCreateMonitor`, `MustCreateChannel`, `TwoSessions` (for
cross-tenant). SQL-direct insertion is forbidden in C1 (would leak
schema knowledge into black-box tests). One exception: post-assertion
SELECTs to verify invariants (e.g. session row written).

**Build tag:** `//go:build integration`. Run:
`go test -tags=integration ./tests/integration/api/...`. Short unit
runs (`go test -short ./...`) skip this package.

**Parallelism:** `t.Parallel()` is NOT used — shared containers with
global truncate. Sequential run for ~95 tests is estimated under 3
minutes locally; acceptable. Revisit only if CI time becomes an issue.

## Behavior-spec structure

Written to
`docs/superpowers/specs/2026-04-18-C1-api-behavior-spec.md`.
Eight sections:

1. Error envelope + HTTP status conventions
2. Authentication model (session cookie, API key)
3. Tenant isolation rule (403 on cross-tenant, not 404)
4. Per-endpoint contract (27 entries, 5 fields each)
5. Rate limits (concrete numbers, tier-aware)
6. Validation rules (email, password, URL, interval, timeout)
7. Webhook contract (HMAC scheme, idempotency, token-in-URL)
8. Open product questions (called out; user answers before tests)

## Test file layout

```
tests/integration/api/
  harness/
    testmain.go         TestMain (containers, migrations, shutdown)
    containers.go       testcontainers wrappers
    app.go              SetupApp wiring + in-process Fiber server
    truncate.go         between-test reset
    session.go          Session (cookie jar) + Register/Login helpers
    fakes.go            FakeSMTP, FakeTelegram, FakeWebhookSink
    assert.go           AssertError, AssertJSON, schema checkers
  auth_test.go
  monitors_test.go
  channels_test.go
  apikeys_test.go
  status_test.go
  health_test.go
  webhook_test.go
  common_test.go        auth matrix (each protected endpoint × [no-auth, bad-cookie, revoked-key])
```

`tests/integration/repo_test.go` stays put for now (it tests the
Postgres adapter directly, a different concern). A follow-up step
(tracked in the plan) will move it to
`internal/adapter/postgres/*_integration_test.go` with the same build
tag, consolidating the integration-test package as purely black-box.

## Example test

```go
//go:build integration

func TestCreateMonitor_DuplicateName_Returns409(t *testing.T) {
    h := harness.New(t)
    s := h.RegisterAndLogin(t, "alice@test.local", "pw12345678")
    h.MustCreateMonitor(t, s, harness.MonitorInput{
        Name: "api", URL: "https://x.io",
    })

    resp := s.POST(t, "/api/monitors", harness.MonitorInput{
        Name: "api", URL: "https://y.io",
    })
    harness.AssertError(t, resp, 409, "already exists")
}
```

One behavior per test. Test name is the documentation.

## Acceptance criteria

C1 is complete when:

1. `go test -tags=integration ./tests/integration/api/...` passes
   with 0 failures on a clean checkout (Docker running).
2. Behavior-spec document exists and covers all 27 endpoints + the 8
   structural sections.
3. Test count is within ±20% of 95 (roughly 75–115 tests).
4. Each of the 5 findings from "Findings surfaced during scope
   analysis" is **resolved** — either by a code change that aligns
   behavior with the spec, or by an explicit spec decision to
   preserve current behavior (documented with rationale).
5. `Clock` and `Random` ports exist in `internal/port/`, production
   wiring uses stdlib impls, test harness uses deterministic impls.
6. CI job `integration.yml` runs the suite on PRs targeting `main`
   (5-minute timeout).
7. README "Testing" table updated with the new layer and run command.

## Execution plan (handed to writing-plans skill)

writing-plans will break this design into sequential implementation
steps with verification checkpoints. Rough shape:

1. Author behavior-spec (product decisions locked, user approval).
2. Scaffold harness package (containers, truncate, in-process app).
3. Add `Clock`/`Random` ports + wire through DI.
4. Move `repo_test.go` into `internal/adapter/postgres/`.
5. Write tests per endpoint group (auth → health → monitors →
   channels → monitor-channel → api-keys → status → webhooks),
   fixing code as tests fail.
6. CI workflow + README update.

## Risks

- **Behavior-spec authoring dominates C1.** Writing tests is
  mechanical once the spec is locked. If the user is slow to resolve
  open product questions, C1 stalls. Mitigation: spec ships with
  sensible defaults (based on current code where invariant, best-guess
  where not) marked "default — revisit"; tests run on defaults.
- **Code changes demanded by tests may cascade** (e.g. switching error
  envelope affects the frontend). Mitigation: frontend type generation
  (`pnpm gen:types`) will catch envelope schema drift; Playwright tests
  in C3 catch runtime drift. If blast radius balloons, we pause code
  changes and ship spec + tests that currently fail, tracking the fix
  as a separate follow-up.
- **testcontainers boot time** adds ~30s to every `go test` run.
  Mitigation: `-short` tag skips integration; developers iterate on
  unit tests and run integration on commit.

# PingCast

Uptime monitoring that doesn't suck. HTTP, TCP, and DNS checks every 30
seconds. Instant alerts to Telegram, email, or webhooks. Public status
pages with SSR + ISR. Scoped API keys for programmatic access.

## Architecture

PingCast ships as **two containers**: a Go API (`pingcast-api`) and a
Next.js frontend (`pingcast-web`). Four background Go services —
scheduler, worker, notifier — share the same repo.

```
┌─────────────────────────┐        ┌─────────────────────────┐
│  pingcast-web (:3000)   │  /api  │   pingcast-api (:8080)  │
│  Next.js 16 · SSR + ISR │──────▶│   Fiber · apigen        │
│  Tailwind · shadcn/ui   │  JSON  │   sqlc · pgx            │
└─────────────────────────┘        └───┬─────────────────────┘
                                       │
         ┌─────────────────────────────┼────────────────────┐
         ▼                             ▼                    ▼
┌────────────────┐           ┌────────────────┐    ┌────────────────┐
│   PostgreSQL   │           │   NATS         │    │   Redis        │
│   (sqlc)       │           │   JetStream    │    │   sessions,    │
│   monitors,    │           │   MONITORS,    │    │   rate-limit,  │
│   incidents,   │           │   CHECKS,      │    │   circuit-     │
│   check_results│           │   ALERTS       │    │   breaker      │
└────────────────┘           └────────────────┘    └────────────────┘
         ▲                             ▲
         │                             │
┌────────┴──────────┐     ┌───────────┴──────────┐    ┌──────────────────┐
│ pingcast-scheduler│     │ pingcast-worker      │    │ pingcast-notifier│
│ leader-elected,   │     │ pull-based consumer, │    │ alert delivery   │
│ fans out checks   │     │ runs HTTP/TCP/DNS    │    │ with retry+CB    │
└───────────────────┘     └──────────────────────┘    └──────────────────┘
```

Frontend talks to the Go API via JSON only. Session auth flows through a
cookie set by Go on login; Next.js forwards it on SSR fetches via
`lib/session.ts`, and the browser attaches it automatically for
client-side queries.

## Stack

**Backend (Go 1.26):**
- **HTTP:** Fiber v2, oapi-codegen (OpenAPI → handlers), apigen types
- **Database:** PostgreSQL 16, pgx/v5, sqlc (compile-time SQL safety)
- **Messaging:** NATS JetStream (durable streams, DLQ on max-deliver)
- **Cache & rate-limit:** Redis 7, redis_rate, redsync (distributed locks)
- **Encryption:** AES-256-GCM with key-versioned ciphertext (`[version][nonce][ct+tag]`)
- **Observability:** OpenTelemetry traces → Tempo, logs → Loki, Grafana

**Frontend (Node 20):**
- Next.js 16 App Router, TypeScript 5, Tailwind CSS v4
- shadcn/ui (base-nova style on top of @base-ui/react + Radix primitives)
- TanStack Query (15s polling on live views), React Hook Form
- Framer Motion (hero animations), Recharts (response-time chart),
  Lucide icons, next-themes (system / light / dark)
- openapi-typescript (Go OpenAPI → TS types)
- Playwright E2E

## Running locally

Requires: Docker, Go 1.26+, Node 20+, pnpm 8+.

```bash
# Start all services
docker compose up -d

# App lives at:
#   http://localhost:3001  — web (Next.js)
#   http://localhost:8080  — API (Go)
#   http://localhost:3000  — Grafana dashboards (observability stack)
```

If Docker reclaims are low, NATS JetStream may fail to initialize with
"insufficient storage". `docker system prune -a --volumes -f` frees
build cache and fixes it.

### Frontend dev loop

```bash
cd frontend
pnpm install
pnpm dev        # http://localhost:3000 — will conflict with Grafana's 3000; stop it first or run Next on another port
pnpm gen:types  # regenerate TS types from ../api/openapi.yaml
pnpm test:e2e   # Playwright (requires docker compose up)
```

### Go dev loop

```bash
# Quick unit-only run
go test -short ./...

# Integration tests (black-box API + postgres repos, testcontainers, Docker required)
make test-integration

# Everything (unit + integration + lint)
make test && make lint
```

## Repo layout

```
api/openapi.yaml               # OpenAPI 3 spec — source of truth for types
cmd/                           # service entrypoints (api, scheduler, worker, notifier)
docs/superpowers/              # architectural specs & implementation plans
frontend/                      # Next.js 16 app
  app/                         # App Router routes
    (main)/                    # routes with Navbar + Footer
    status/[slug]/             # public status page (no Navbar)
  components/
    ui/                        # shadcn/ui (generated)
    features/                  # feature-specific composed components
    site/                      # navbar, footer, theme toggle
  lib/                         # api client, queries, mutations, session helper
  tests/                       # Playwright E2E
  Dockerfile                   # multi-stage Alpine build
internal/
  adapter/                     # hex-arch adapters (Postgres, Redis, NATS, SMTP, Telegram, Webhook, Checker, HTTP)
  api/gen/                     # oapi-codegen output
  app/                         # application services (MonitoringService, AlertService, AuthService)
  bootstrap/                   # composition-root helpers (cipher DI)
  config/                      # env parsing
  crypto/                      # AES-256-GCM Encryptor + NoOpCipher
  database/migrations/         # SQL migrations, goose-managed
  domain/                      # domain types, errors, validation
  port/                        # port interfaces (hex-arch)
  sqlc/                        # SQL queries + generated Go
  xcontext/                    # Detached context helper (OTel link)
```

## Testing

| Layer | Tool | Count |
|---|---|---|
| Go unit | `go test -short ./...` | 9 packages |
| Go repo integration | testcontainers + Postgres | ~20 tests |
| Go API integration (black-box, C1) | testcontainers + Postgres + Redis + NATS | ~80 tests |
| Frontend unit | — (skipped for now) | 0 |
| Frontend E2E | Playwright | 11 specs |

The API integration suite is black-box: each test drives the Fiber
app in-process via HTTP, asserts the canonical error envelope
`{"error":{"code","message"}}` from spec §1, and resets Postgres +
Redis state between runs. The behavior spec lives at
`docs/superpowers/specs/2026-04-18-C1-api-behavior-spec.md` —
tests assert against that document, not against current code.

Run the full suite:

```bash
make lint              # golangci-lint, must be 0 findings
make test-short        # unit-only, ~seconds
make test-integration  # black-box API + repo, ~30s first boot
cd frontend && pnpm test:e2e   # Playwright (requires docker compose up)
```

## Production deployment

The repo targets Dokploy, but any docker-compose / Kubernetes host works.
The web service is a Next.js standalone build (~200 MB Alpine image);
the API is a scratch image with only the Go binary + ca-certificates
(~30 MB).

Environment variables for the API service (`cmd/api/main.go`):

| Var | Required | Default | Purpose |
|---|---|---|---|
| `DATABASE_URL` | yes | — | Postgres connection |
| `REDIS_URL` | no | `redis://localhost:6379` | sessions, rate-limit |
| `NATS_URL` | no | `nats://localhost:4222` | JetStream |
| `PORT` | no | `8080` | HTTP listen port |
| `ENCRYPTION_KEYS` | no | (disables encryption) | `1:base64key,2:base64key` |
| `ENCRYPTION_PRIMARY_VERSION` | no | `1` | which key to encrypt with |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | no | (OTel off) | OpenTelemetry collector |
| `BASE_URL` | no | `http://localhost:8080` | absolute URL for emails |
| `LEMONSQUEEZY_WEBHOOK_SECRET` | no | — | HMAC-verify billing webhooks |

Web service variables (`frontend/`):

| Var | Required | Default | Purpose |
|---|---|---|---|
| `INTERNAL_API_URL` | yes | `http://api:8080/api` | SSR-side API calls |
| `NEXT_PUBLIC_API_URL` | yes | `http://localhost:8080/api` | client-side rewrite target |
| `NEXT_PUBLIC_SITE_URL` | no | `http://localhost:3000` | used in OpenGraph URLs |

## Contributing

- Never break lint. `golangci-lint run` must be 0.
- Never commit behind-the-fold: every PR builds, tests, lints clean.
- Prefer shadcn patterns over one-off Tailwind utilities on the frontend.
- Generated code (`internal/sqlc/gen/`, `internal/api/gen/`,
  `frontend/lib/openapi-types.ts`) is checked in; regenerate after
  touching `*.sql` or `openapi.yaml`.

## Licence

TBD.

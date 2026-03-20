# PingCast — Uptime & API Monitoring SaaS

## Overview

PingCast is a lightweight, affordable uptime monitoring service targeting developers and small teams. Users add URLs to monitor — the system checks them on a schedule and sends alerts (Telegram, email) when a service goes down. Each user gets a public status page to share with their customers.

**Core value proposition:** Simpler and cheaper than UptimeRobot/BetterStack, with a clean developer-friendly UX.

## MVP Features

### Monitors
- HTTP(S) checks with configurable interval (30s / 1m / 5m)
- Configurable expected status code
- Keyword check in response body (Pro only)
- GET/POST method support
- Pause/resume monitors

### Alerts
- Telegram notifications (primary channel) — user links account via bot /start command
- Email notifications (Pro only)
- Configurable alert threshold: alert after N consecutive failures (to reduce false positives)
- Notifications on both down and recovery events

### Dashboard
- List of monitors with current status (up/down)
- Uptime percentage: 24h / 7d / 30d
- Response time chart per monitor
- Web interface built with HTMX + Go templates

### Public Status Page
- URL: `status.pingcast.io/<slug>` where slug is a unique username chosen at registration
- User selects which monitors to display publicly
- Shows current status, uptime %, incident history
- "Powered by PingCast" branding on Free plan (removed on Pro)

### Not in MVP (deferred)
- Multi-region checks
- TCP/UDP/DNS/ICMP monitoring
- Incident management workflows
- Team/organization accounts
- Slack/Discord/webhook alert channels
- Public API for external integrations
- Custom domains for status pages (requires DNS verification + per-domain SSL)

## Architecture

Single Go binary (monolith). Internal separation into packages, but one deployment unit.

```
┌─────────────────────────────────────────────┐
│                 PingCast                     │
│                                             │
│  ┌───────────┐  ┌───────────┐  ┌─────────┐ │
│  │  HTTP API  │  │ Scheduler │  │ Notifier│ │
│  │   (Chi)    │  │           │  │(TG/SMTP)│ │
│  └─────┬─────┘  └─────┬─────┘  └────┬────┘ │
│        │               │              │      │
│        └───────┬───────┘              │      │
│                │                      │      │
│         ┌──────▼──────┐              │      │
│         │   Workers   │──────────────┘      │
│         │ (goroutines)│                      │
│         └──────┬──────┘                      │
│                │                             │
│         ┌──────▼──────┐                      │
│         │ PostgreSQL  │                      │
│         └─────────────┘                      │
└─────────────────────────────────────────────┘
```

### Components

- **HTTP API (Chi)** — REST API for CRUD monitors, stats, authentication. Serves HTMX frontend via Go templates.
- **Scheduler** — Single goroutine managing a timer heap (priority queue). When a check is due, dispatches work to the worker pool. Reloads timers on monitor CRUD operations.
- **Workers** — Bounded goroutine pool (e.g., 100 workers). Receives check jobs from the scheduler, executes HTTP requests, writes results to DB, and publishes events to the Notifier.
- **Notifier** — Subscribes to PostgreSQL LISTEN/NOTIFY on `monitor_events` channel. Receives JSON payloads `{"monitor_id": "...", "event": "down|up", "details": "..."}` published by workers after status transitions. Sends Telegram messages and emails. Auto-reconnects on connection loss with exponential backoff.
- **PostgreSQL** — Single data store. Monitors, check results, users, alert settings.

### Why this architecture
- Monolith is appropriate for a solo developer and early-stage product
- PostgreSQL LISTEN/NOTIFY for internal events — no need for Redis or message queues yet
- Goroutines handle concurrency natively — Go is ideal for thousands of parallel checks
- HTMX + Go templates: one language, one deploy, server-rendered HTML with SEO support
- Graceful shutdown via `signal.NotifyContext` — drains in-flight checks and closes DB connections on SIGTERM

## Data Model

### users
| Column | Type | Notes |
|---|---|---|
| id | uuid | PK |
| email | varchar | unique |
| slug | varchar | unique, used in status page URL, chosen at registration, validated: `^[a-z0-9-]{3,30}$` |
| password_hash | varchar | bcrypt |
| tg_chat_id | bigint | nullable, set via Telegram bot |
| plan | varchar | 'free' or 'pro' |
| lemon_squeezy_customer_id | varchar | nullable, set after first checkout |
| lemon_squeezy_subscription_id | varchar | nullable, active subscription |
| created_at | timestamptz | |

### monitors
| Column | Type | Notes |
|---|---|---|
| id | uuid | PK |
| user_id | uuid | FK → users |
| name | varchar | display name |
| url | varchar | target URL |
| method | varchar | GET or POST |
| interval_seconds | int | 30, 60, or 300 |
| expected_status | int | e.g. 200 |
| keyword | varchar | nullable, Pro only |
| alert_after_failures | int | default 3 |
| is_paused | bool | default false |
| is_public | bool | show on status page |
| current_status | varchar | 'up', 'down', 'unknown' |
| created_at | timestamptz | |

### check_results
| Column | Type | Notes |
|---|---|---|
| id | bigserial | PK |
| monitor_id | uuid | FK → monitors |
| status | varchar | 'up' or 'down' |
| status_code | int | nullable |
| response_time_ms | int | |
| error_message | text | nullable |
| checked_at | timestamptz | |

**Partitioning:** Monthly partitions on `checked_at`. Auto-drop old partitions via `pg_cron` job running daily.
**Retention:** Free plan — 7 days, Pro plan — 90 days. On downgrade, data older than 7 days is purged at next cleanup run. On upgrade, only new data benefits from extended retention (already-deleted data is not recoverable).
**Index:** `(monitor_id, checked_at)` for dashboard queries.
**Aggregation:** For chart display, pre-aggregate to 5-minute averages (24h view), hourly averages (7d view), daily averages (30d view). Aggregation computed at query time via SQL GROUP BY — no materialized views in MVP.

**Capacity estimate:** One Pro user with 50 monitors at 30s interval = 144,000 rows/day (~14MB/day at ~100 bytes/row). 1,000 Pro users = ~14GB/day before cleanup. With 90-day retention = ~1.26TB max. VPS disk must be sized accordingly as the product scales.

### incidents
| Column | Type | Notes |
|---|---|---|
| id | bigserial | PK |
| monitor_id | uuid | FK → monitors |
| started_at | timestamptz | first failed check |
| resolved_at | timestamptz | nullable, recovery time |
| cause | text | error message from first failure |

### sessions
| Column | Type | Notes |
|---|---|---|
| id | varchar(64) | PK, secure random token |
| user_id | uuid | FK → users |
| expires_at | timestamptz | 30 days from creation, rolling renewal |
| created_at | timestamptz | |

**Indexes:** `expires_at` for cleanup job (delete expired sessions via `pg_cron` daily).

**Incident state machine:**
- **Created** when `alert_after_failures` consecutive check failures occur for a monitor. Not on the first failure — only after the threshold is reached.
- **Resolved** when the first successful check occurs after an incident was created. `resolved_at` is set to the timestamp of that check.
- **Flap protection:** A new incident cannot be created within 5 minutes of resolving the previous one for the same monitor. During this cooldown, failures increment a counter but do not create a new incident.
- The `cause` field captures the error message from the check that triggered the incident (the Nth consecutive failure).

## Monetization

### Plans

| | Free | Pro ($5/month) |
|---|---|---|
| Monitors | 5 | 50 |
| Check interval | 5 min | 30 sec |
| Data retention | 7 days | 90 days |
| Status page | 1 (PingCast branding) | 1 (no branding) |
| Telegram alerts | yes | yes |
| Email alerts | no | yes |
| Keyword check | no | yes |

### Payment
- **Lemon Squeezy** — handles checkout, subscriptions, VAT/tax compliance
- Webhook events handled: `subscription_created`, `subscription_updated`, `subscription_cancelled`, `subscription_payment_failed`
- Webhook signature verification via `X-Signature` header (HMAC)
- On `subscription_created/updated`: update user's `plan` to 'pro', store `lemon_squeezy_subscription_id`
- On `subscription_cancelled`: downgrade user to 'free' at period end
- On `subscription_payment_failed`: 7-day grace period, then downgrade to 'free'
- Plan limits enforced at API level (monitor creation, feature gating)

### Unit economics
- Infrastructure cost per Pro user: ~$0.5-1/month
- Price: $5/month
- Margin: ~80-90%

## Infrastructure & Deployment

### Existing setup (user's)
- **Server:** Hostkey VPS (already available)
- **Deployment:** Dokploy (self-hosted PaaS, already configured)
- **SSL:** Managed by Dokploy via Traefik (automatic)

### Deployment model
- Docker container deployed via Dokploy
- PostgreSQL as a separate Dokploy service
- Domain pointed to Dokploy, status page subdomains via Traefik
- CI/CD: GitHub push → Dokploy auto-build and deploy

### Backups
- pg_dump via cron to S3-compatible storage
- Daily backup, 7-day retention

### Self-monitoring
- Health check endpoint + free UptimeRobot account
- Logs via Docker stdout → Dokploy log viewer

### Cost
- Domain: ~$10/year
- Everything else: already available

## Growth & Marketing Strategy

### Phase 1 — Launch (Month 1-2)
- **Product Hunt / Hacker News** — "Show HN" post. Open-source core for trust and visibility.
- **Reddit** — r/selfhosted, r/webdev, r/sideproject, r/SaaS. Story-driven posts, not ads.
- **Dev.to / Habr** — Technical article about building PingCast in Go. Dual value: content + traffic.

### Phase 2 — Organic growth (Month 3-6)
- **Status pages as marketing** — Every public status page has "Powered by PingCast" link. Clients of your users discover PingCast organically.
- **SEO** — Landing page targeting: "free uptime monitoring", "website monitoring tool", "status page for startups". Blog with guides.
- **Telegram channel** — Changelog, tips, internet uptime stats. Community around the product.

### Phase 3 — Scale (Month 6+)
- **Integrations** — Slack, Discord, PagerDuty. Each integration = new acquisition channel.
- **Referral program** — "Invite a friend, get +5 free monitors."
- **Comparison pages** — "PingCast vs UptimeRobot", "PingCast vs BetterStack" — strong SEO play.

### Marketing budget: $0
All channels above are free. Paid ads only after unit economics are validated (people are already paying).

## Authentication

### Registration
- Email + password + slug (username for status page URL)
- Slug validation: `^[a-z0-9-]{3,30}$`, unique, reserved slugs blocked: `login, logout, register, api, admin, status, health, webhook, pricing, docs, app, dashboard, settings, billing`
- Email verification deferred to post-MVP — register and use immediately
- Password requirements: minimum 8 characters

### Login
- Form POST with email + password, returns session cookie
- Rate limiting: 5 failed attempts per email per 15 minutes (in-memory counter, reset on success)
- Session stored in PostgreSQL `sessions` table (id, user_id, expires_at)
- Cookie: `HttpOnly`, `Secure`, `SameSite=Lax`, 30-day expiry, rolling renewal on activity

### Password reset
- Deferred to post-MVP. Users can contact support via Telegram.

## HTTP Client Configuration

The HTTP client is the core product — its behavior must be well-defined:

- **Timeout:** 10 seconds (connect + read). If exceeded, check result is "down" with error "timeout".
- **Redirects:** Follow up to 5 redirects. Final response is evaluated.
- **TLS verification:** Enabled by default. Expired/invalid cert = "down" with error describing the TLS issue.
- **User-Agent:** `PingCast/1.0 (uptime monitor; https://pingcast.io)`
- **Max response body read:** 1MB (for keyword checks). Body is read but not stored.
- **POST body:** Empty body. POST method is for APIs that require POST to return a health response.
- **Rate limiting outbound:** Max 1 concurrent check per target host (in-memory per-host semaphore map). If a worker picks up a job for a host that is already being checked, it waits on the semaphore (does not re-queue).

## Logging & Observability

- Structured JSON logs via `slog` (Go stdlib)
- Log fields: request_id, user_id, monitor_id where applicable
- Log levels: ERROR for failures, WARN for degraded states, INFO for key events (monitor created, alert sent)
- Logs to stdout → Docker/Dokploy log viewer

## Tech Stack Summary

| Layer | Technology |
|---|---|
| Language | Go |
| HTTP framework | Chi |
| Frontend | HTMX + Go templates + Chart.js |
| Database | PostgreSQL |
| Auth | Session-based (cookies) |
| Payments | Lemon Squeezy |
| Telegram | Telegram Bot API |
| Email | SMTP (Resend or similar) |
| Deployment | Docker + Dokploy |
| CI/CD | GitHub Actions → Dokploy |
| Server | Hostkey VPS |

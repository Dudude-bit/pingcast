# C1 — API behavior specification

Status: draft (pending user product decisions on §8)
Date: 2026-04-18
Companion to: `2026-04-18-C1-api-contract-tests-design.md`

This document is the authoritative contract for integration tests.
Where the current Go code diverges from this spec, the **code** gets
fixed. Where this spec is silent, OpenAPI schemas apply.

## §1 Error envelope and HTTP status conventions

**Canonical envelope** (every 4xx/5xx response):

```json
{
  "error": {
    "code": "STRING_IDENTIFIER",
    "message": "Human-readable description."
  }
}
```

No top-level `error` strings. Nothing outside `error`.

**Status codes** (one convention, used uniformly):

| Status | When |
|---|---|
| 400 | Request body is not valid JSON, or path/query params fail type-parse (e.g. malformed UUID). |
| 401 | No authentication provided, or session/key is invalid, expired, or revoked. |
| 403 | Authenticated, but target resource belongs to another tenant. |
| 404 | Authenticated, resource does not exist for this tenant (or public resource does not exist). |
| 409 | Duplicate / conflict (e.g. monitor name already used by this user). |
| 422 | Business-validation failure on well-formed JSON (e.g. interval below tier minimum, bad URL protocol). |
| 429 | Rate limit exceeded. Response MUST include `Retry-After` header. |
| 500 | Internal error. Message MUST be a fixed safe string — never a raw error. |

**Error codes** (machine-readable, stable):
`MALFORMED_JSON`, `MALFORMED_PARAM`, `UNAUTHORIZED`,
`FORBIDDEN_TENANT`, `NOT_FOUND`, `CONFLICT`, `VALIDATION_FAILED`,
`RATE_LIMITED`, `INTERNAL_ERROR`.

## §2 Authentication model

Two mutually exclusive mechanisms. Both verified by middleware before
any business logic.

**Session cookie** (`pingcast_session`):
- Opaque 32-byte random token, base64url-encoded.
- `HttpOnly`, `Secure` (in prod), `SameSite=Lax`, `Path=/`.
- Lifetime: 30 days, sliding — each authenticated request extends
  expiry via `sessions.Touch`.
- Revoked on `POST /api/auth/logout`: server deletes the session row
  AND sets an expired cookie on the response.

**API key** (`Authorization: Bearer pck_<base64>`):
- Token prefix `pck_` identifies the auth scheme.
- Plaintext stored only once (at creation) — server returns it in
  the 201 response body. Never retrievable again.
- Stored server-side as a SHA-256 hash.
- Revoked via `DELETE /api/api-keys/{id}`. A revoked key returns 401
  on next use.

**Endpoint auth matrix:**

| Endpoint | Public | Session | API key |
|---|---|---|---|
| `POST /api/auth/register` | ✓ |  |  |
| `POST /api/auth/login` | ✓ |  |  |
| `POST /api/auth/logout` |  | ✓ |  |
| `GET /api/status/{slug}` | ✓ |  |  |
| `GET /health`, `/healthz`, `/readyz` | ✓ |  |  |
| `POST /webhook/lemonsqueezy` | ✓ (HMAC) |  |  |
| `POST /webhook/telegram/:token` | ✓ (URL secret) |  |  |
| `GET /api/monitor-types`, `/api/channel-types` |  | ✓ | ✓ |
| `/api/api-keys/*` (list, create, delete) |  | ✓ |  |
| All other `/api/*` |  | ✓ | ✓ |
| `POST /logout` (page) — *pending §8.6* |  | ✓ |  |

When both a session cookie and an API key are sent, the API key wins
and the session is ignored (prevents accidental privilege confusion).

## §3 Tenant isolation

For any endpoint accepting `{id}` (or `{slug}` for non-public
resources) referring to a user-owned resource: if the authenticated
principal is not the owner, return **403 FORBIDDEN_TENANT**, not 404.

Rationale: 404 would leak existence. Consistent 403 on all
cross-tenant probes.

Public endpoints (`GET /api/status/{slug}`) are exempt — the status
page is intentionally discoverable and returns 404 for non-existent
slugs.

Owner check applies transitively: `DELETE
/api/monitors/{id}/channels/{channelId}` requires that BOTH the
monitor and the channel belong to the caller.

## §4 Per-endpoint contract

Each entry: **Purpose / Auth / Inputs / Outputs / Rules**. Statuses
listed are the authoritative set; anything else is a bug.

### Auth

**`POST /api/auth/register`** — Public. Body `{email, password}`.
- 201 `{user: {id, email, slug, plan}}` + sets session cookie.
- 400 if body not valid JSON.
- 422 `VALIDATION_FAILED` if email format bad, password < 8 chars,
  or email already taken.
- 429 if more than 10 attempts/hour from one IP.
- Slug: derived from email local-part, collision suffix `-2`, `-3`, …
- Plan: always `free` on registration.
- Password stored as bcrypt (cost ≥ 10).

**`POST /api/auth/login`** — Public. Body `{email, password}`.
- 200 `{user}` + sets session cookie.
- 401 `UNAUTHORIZED` if credentials invalid. Timing of wrong-email vs
  wrong-password MUST be indistinguishable (existing code has
  `auth_timing_test.go` — keep invariant).
- 422 if body malformed (missing fields).
- 429 after 5 failed attempts/15min **keyed by email** (brute-force
  lock on specific account, not IP). A successful login resets the
  counter.

**`POST /api/auth/logout`** — Session. No body.
- 204. Session row deleted, cookie expired.
- 401 if not authenticated.
- Idempotent: a second call from a now-logged-out client is still 401
  (no session to log out).

### Monitor types / Channel types

**`GET /api/monitor-types`, `GET /api/channel-types`** — Session or
API key.
- 200 array of type descriptors. Content is static enum data; never
  empty; no pagination.

### Monitors

**`GET /api/monitors`** — Session or key.
- 200 array of `MonitorWithUptime` (last-24h uptime % per monitor).
- Empty array for users with no monitors — never 404.
- Sorted by `created_at DESC`.

**`POST /api/monitors`** — Session or key. Body `CreateMonitorRequest`.
- 201 `{monitor}`.
- 422 if:
  - `name` empty or > 100 chars
  - `url` not a parseable URL with scheme in `{http, https}` (for HTTP
    type), or not a valid `host:port` (TCP), or not a valid hostname
    (DNS)
  - `interval_seconds` below tier minimum (§5)
  - `timeout_seconds` > `interval_seconds`
  - `type` not in `{http, tcp, dns}`
- 409 `CONFLICT` if a non-deleted monitor with the same name already
  exists for this user.
- 422 `VALIDATION_FAILED` code `MONITOR_LIMIT_REACHED` if the user has
  reached their plan's monitor limit.

**`GET /api/monitors/{id}`** — Session or key.
- 200 `MonitorDetail` — monitor + uptime (24h/7d/30d) + chart (last 24h
  of check_results bucketed) + recent incidents (last 50).
- 400 if `id` not a UUID.
- 401 if unauthenticated.
- 403 if monitor belongs to another user.
- 404 if monitor does not exist at all.

**`PUT /api/monitors/{id}`** — Session or key. Body
`UpdateMonitorRequest` (partial).
- 200 `{monitor}`.
- 400 malformed UUID or JSON.
- 403 cross-tenant.
- 404 not-found.
- 409 if renamed to an existing name within the user's monitors.
- 422 if new interval/timeout/URL fail validation.

**`DELETE /api/monitors/{id}`** — Session or key.
- 204.
- 403/404 per isolation rule.
- **Cascade:** all `monitor_channels` rows referencing this monitor
  are deleted; `check_results` and `incidents` are preserved for
  audit (to be deleted by the retention job, §5).
- Idempotent: deleting an already-deleted monitor returns 404.

**`POST /api/monitors/{id}/pause`** — Session or key.
- 200 `{monitor}` reflecting the new paused state.
- Toggle semantics: if currently running → paused; if paused →
  running. Each call flips. (This mirrors current behavior and avoids
  a separate `/resume`. Verified intentional in §8.1.)
- 403/404 per isolation rule.
- Scheduler MUST skip paused monitors (not in C1 scope to verify —
  only API-observable state matters here).

### Channels

**`GET /api/channels`** — Session or key.
- 200 array of `NotificationChannel`. Empty array for new users.
- Config secrets (telegram bot token, webhook URL with query params)
  are **redacted** in response: structural keys preserved, sensitive
  values masked as `"***"` (last 4 chars may be preserved for UX).

**`POST /api/channels`** — Session or key. Body
`CreateChannelRequest`.
- 201 `{channel}` with secrets redacted as above.
- 422 if:
  - `name` empty
  - `type` not in `{telegram, email, webhook}`
  - `config` shape invalid for the type (e.g. telegram without
    `bot_token` or `chat_id`)
  - User on Free plan and `type == "email"` (email is Pro-only per
    `PlanFree.CanUseEmail() == false`; spec formalizes this) →
    `VALIDATION_FAILED` code `PLAN_UPGRADE_REQUIRED`.

**`GET /api/channels/{id}`** — Session or key.
- (Spec default per §8.5 — **add the endpoint**. Tests assume this
  unless user overrides.)
- 200 `{channel}` with secrets redacted (same rules as list).
- 400 if `id` not a UUID.
- 401 unauthenticated.
- 403 cross-tenant.
- 404 not-found.

**`PUT /api/channels/{id}`** — Session or key. Body
`UpdateChannelRequest`.
- 200 `{channel}` redacted. Partial update: unset fields unchanged.
- 422 on same validation rules as create (for the fields provided).
- 403/404 per isolation rule.

**`DELETE /api/channels/{id}`** — Session or key.
- 204.
- 403/404 per isolation rule.
- Cascade: `monitor_channels` rows deleted. No soft-delete.

### Monitor-channel binding

**`POST /api/monitors/{id}/channels`** — Session or key. Body
`{channel_id}`.
- 201 (or 204 — **§8.2**).
- 400 malformed UUIDs.
- 403 if the monitor OR the channel belongs to another user.
- 404 if either does not exist.
- 409 if already bound (idempotent or conflict — **§8.3**).

**`DELETE /api/monitors/{id}/channels/{channelId}`** — Session or key.
- 204.
- 403 per isolation (both monitor and channel must be caller's).
- 404 if the binding does not exist (not the monitor or channel — the
  binding specifically).

### API keys

**`GET /api/api-keys`** — Session only (API key CANNOT list keys).
- 200 array of `{id, name, created_at, last_used_at, prefix}`. Full
  token never returned.

**`POST /api/api-keys`** — Session only.
- 201 `{id, name, created_at, token}` — `token` present here and only
  here.
- 422 on bad name (empty, > 100 chars).
- Limit: 10 active keys per user — 422 `KEY_LIMIT_REACHED` on 11th.

**`DELETE /api/api-keys/{id}`** — Session only.
- 204.
- 403/404 per isolation rule.
- Effective immediately: next request with the revoked key → 401.

### Public status page

**`GET /api/status/{slug}`** — Public.
- 200 `{user_slug, monitors: [{name, type, status, uptime_24h}]}`.
  Only `is_public=true` monitors are listed.
- 404 if slug does not exist.
- 429 if > 60 requests/min from one IP.
- `Cache-Control: public, max-age=30` header.

### Health

**`GET /health`** — Public. 200 `{"status":"ok"}`. Liveness only.

**`GET /healthz`** — Public. 200 `{"status":"ok"}`. Alias for `/health`.

**`GET /readyz`** — Public.
- 200 `{"status":"ok","deps":{"postgres":"ok","redis":"ok","nats":"ok"}}`
  when all deps reachable.
- 503 `{"status":"unready","deps":{...}}` when any dep is down. Each
  dep has its own boolean.

### Webhooks

**`POST /webhook/lemonsqueezy`** — Public, HMAC-protected.
- Request header `X-Signature`: hex-encoded HMAC-SHA256 of the raw
  request body keyed by `LEMONSQUEEZY_WEBHOOK_SECRET`.
- 200 `{"ok":true}` on accepted event (even for no-op events — Lemon
  Squeezy requires 2xx or it retries).
- 401 `UNAUTHORIZED` if signature missing or mismatches.
- 400 if body is not parseable JSON.
- **Idempotency:** requests with a previously-seen `event_id` return
  200 without re-applying side effects. Dedup key stored in Redis
  with 7-day TTL.
- Handled events: `subscription_created`, `subscription_updated`,
  `subscription_cancelled`. Other event types are ignored with 200.

**`POST /webhook/telegram/:token`** — Public, URL-path secret.
- `:token` is the channel's `webhook_secret` (per-channel, not
  global). Constant-time compare against the channel row.
- 200 on accepted update.
- 401 if the token does not match any channel.
- 400 if body is not a valid Telegram Update payload.
- Rate-limit: 60/min/channel (abuse shield).

### Page logout

**`POST /logout`** — Session, form-POST.
- (Spec default per §8.6 — **remove the route**. Tests assert that
  `POST /logout` returns 404. If user overrides, update tests to
  assert a 303 redirect + expired cookie + CSRF-token requirement.)

## §5 Rate limits

All rate limits are enforced at the HTTP layer using `redis_rate`
sliding windows. 429 responses include `Retry-After` in seconds.

| Scope | Key | Limit |
|---|---|---|
| Register | IP | 10 / hour |
| Login | email (lowercased) | 5 / 15 min; resets on success |
| Public status page | IP + slug | 60 / min |
| Write API (create/update/delete) | user | 120 / min |
| Read API | user | 600 / min |
| Webhook `/lemonsqueezy` | — | none (LS controls volume) |
| Webhook `/telegram/:token` | channel | 60 / min |

Retention / quotas:
- `check_results`: retained **30 days**. Older rows pruned by a daily
  job. C1 does not test the job directly but tests that the API
  respects the window (e.g. chart data never older than 30 days).

## §6 Validation rules

**Email:** RFC 5321 local-part limits; must contain `@` and a
non-empty domain with at least one dot. Max length 254.

**Password:** ≥ 8 chars, ≤ 128 chars. No composition requirements
(entropy comes from length).

**Monitor URL (type=http):** absolute URL with scheme `http` or
`https`, a non-empty host, no userinfo component (`user:pass@`
rejected).

**Monitor target (type=tcp):** `host:port`; port in `[1, 65535]`.

**Monitor target (type=dns):** valid DNS hostname (RFC 1123 labels).

**Monitor interval:** Free ≥ 300s, Pro ≥ 30s. Max 86400s (1 day).

**Monitor timeout:** 1 ≤ timeout ≤ interval. If > interval, 422.

**Monitor name:** 1–100 chars, no control characters.

**Channel name:** 1–100 chars.

**API key name:** 1–100 chars.

## §7 Webhook contract details

### LemonSqueezy signature format

```
X-Signature: <hex(hmac_sha256(secret, raw_body))>
```

Compare with `hmac.Equal` (constant-time). Secret comes from
`LEMONSQUEEZY_WEBHOOK_SECRET`. If the env var is unset, every request
returns 500 `INTERNAL_ERROR` (misconfiguration — not 401, because the
receiver is broken, not the sender).

### Idempotency store

Redis key: `lemonsqueezy:event:{event_id}`, value: timestamp, TTL: 7d.
`SET NX` semantics — if the key existed, we dedupe.

### Telegram webhook

Token is generated at channel creation, 32 bytes base64url. Path is
`/webhook/telegram/{token}`. Tokens are unique across all channels.
On constant-time mismatch: 401.

## §8 Open product questions

These are the decisions the spec needs before tests are authored.
Defaults listed are what the tests will assume if unanswered.

**§8.1 Pause/resume semantics.** Toggle (current) or explicit
`/pause` + `/resume`?
*Default:* Toggle — matches current code. Endpoint is idempotent by
target state (paused twice stays paused, returns 200).

**§8.2 Bind-channel response code.** 201 with binding object, or 204?
*Default:* 204 — binding has no entity identity worth returning.

**§8.3 Bind-already-bound.** 409 conflict, or 204 idempotent?
*Default:* 204 idempotent — client doesn't care, re-binding is a
no-op.

**§8.4 DELETE monitor cascade.** Hard-delete monitor, keep
`check_results` and `incidents` for 30 days retention, then prune?
*Default:* Yes (as specified in §4 and §5). Alternative — full
cascade delete — would lose historical data for post-mortems.

**§8.5 `GET /api/channels/{id}`.** Add the endpoint, or document
asymmetry and test the 404?
*Default:* Add it. Symmetry with `/api/monitors/{id}` is expected by
any API consumer, and the frontend will eventually want it for
channel-detail views. Code change is small.

**§8.6 Top-level `POST /logout` page endpoint.** Keep (for future
SSR needs) or remove (dead after Next.js migration)?
*Default:* Remove. Frontend logs out via `POST /api/auth/logout`
exclusively; dead code is a maintenance tax.

**§8.7 Free-tier email channels.** Currently blocked at domain level
(`PlanFree.CanUseEmail == false`). Keep this restriction at the API
boundary?
*Default:* Keep. Blocks at API with 422 `PLAN_UPGRADE_REQUIRED` —
more informative than silent domain-layer filtering.

**§8.8 Rate-limit numbers in §5.** Are the proposed numbers correct,
or should any be tighter/looser?
*Default:* as written. Tune after observing real traffic.

**§8.9 Channel config redaction.** Redact all secrets, or return as
stored?
*Default:* Redact as described in §4 (`***` with optional last-4).
Current code likely returns raw values — this will be a code change.

---

## Change log

- 2026-04-18 — initial draft (Claude, via brainstorming skill).

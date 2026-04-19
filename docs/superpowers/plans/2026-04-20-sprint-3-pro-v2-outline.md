# Sprint 3 — Pro v2 + Distribution Hooks · Outline

> **Status:** outline. Refine to full TDD plan via the writing-plans skill before execution.

**Goal:** Ship the most-asked-about Pro features (custom domain, email subscribers) and the two viral distribution hooks (incident widget, monitor groups).

**Effort:** ~2 weeks of evening work. Custom domain and email subscriptions are the heaviest items.

**Source spec:** `docs/superpowers/specs/2026-04-20-seo-landing-sales-design.md` §6 Sprint 3.

---

## Task 1: Custom domain — DB + service

**Files:**
- New migration `022_create_custom_domains.sql` — `(id, user_id, hostname, dns_validated_at, cert_issued_at, status enum(pending/active/failed))`.
- New `internal/sqlc/queries/custom_domains.sql`.
- New `internal/domain/custom_domain.go`.
- Port + repo + service `CustomDomainService` with methods `Request(userID, hostname)`, `MarkValidated(domainID)`, `MarkCertIssued(domainID)`.
- Hostname validation: must be a valid DNS name, must NOT be on our apex, must NOT collide with another customer's domain.

**Test gates:** unit (validator + state machine), integration repo tests.

**Effort:** ~2 days.

---

## Task 2: Custom domain — DNS validation worker

**Files:**
- New `cmd/dnsvalidator/main.go` (or fold into existing worker).
- Periodically scan `custom_domains` with `status=pending`, do an HTTP GET on `https://hostname/.well-known/pingcast/<token>` (token written to DB at request time, served by our edge for any custom domain). If the request resolves to our IP and returns the token, mark `dns_validated_at`.
- Tests: integration with a httptest.Server posing as the customer's site.

**Effort:** ~2 days.

---

## Task 3: Custom domain — ACME provisioning queue

**Files:**
- Use Traefik's existing ACME provider. Add per-customer-subdomain Traefik route configuration via the dynamic API (Traefik file provider with hot reload, OR docker-labels via a sidecar).
- New `internal/adapter/acme/queue.go` — bounded queue (max 10 concurrent ACME challenges) so we don't blow the Let's Encrypt 300/3hr authorisation rate limit.
- On `dns_validated_at`, enqueue cert provisioning. Worker drains queue.
- Tests: integration (mock Traefik provider; assert queue respects concurrency limit).

**Effort:** ~3 days. Highest risk in Sprint 3 — pin a day to validate against real LE staging.

---

## Task 4: Custom domain — host header routing

**Files:**
- Modify `internal/adapter/http/setup.go` — for `Host` headers that match a custom-domain row, route to the status-page handler with the resolved `slug`.
- Frontend: dashboard page `/dashboard/custom-domain` — request a domain, see DNS instructions, see live status (pending/validated/cert-issued/active).
- Playwright: simulated full flow with a wildcard DNS pointing at localhost.

**Effort:** ~2 days.

---

## Task 5: Email subscriptions

**Files:**
- New migration `023_create_status_subscribers.sql` — `(id, slug, email, components[] jsonb, confirmed_at, unsubscribe_token, created_at)`.
- New sqlc queries.
- Service: `SubscribeEmail` → sends double-opt-in confirm email; `Confirm(token)`; `Unsubscribe(token)`; `NotifyAll(slug, incidentUpdate)`.
- New routes (public): `POST /status/:slug/subscribe`, `GET /status/:slug/confirm?token=`, `GET /status/:slug/unsubscribe?token=`.
- On `IncidentUpdate.Create` (from Sprint 1 Task 4), publish event; new `notifier` consumer fires emails to confirmed subs of that slug.
- Frontend: subscription form on `/status/[slug]/`. Confirm + unsubscribe pages (no auth).
- GDPR: unsubscribe link in every email body, double opt-in mandatory.
- Tests: integration end-to-end (subscribe → confirm → fire incident → assert email goes out → unsubscribe → fire again → assert no email).

**Effort:** ~3 days.

---

## Task 6: Multi-monitor groups

**Files:**
- New migration `024_create_monitor_groups.sql` — `(id, user_id, name, ordering)`.
- Add `group_id` nullable foreign key to `monitors`.
- API: CRUD on `/groups` (Pro-gated).
- Status page: render monitors grouped, collapsible blocks per group.
- Frontend dashboard: drag-and-drop to assign monitors to groups (dnd-kit).
- Tests: integration + Playwright.

**Effort:** ~2 days.

---

## Task 7: Embeddable JS widget

**Files:**
- New `frontend/public/widget.js` — vanilla JS, < 5 KB minified, no framework dependencies. On load: fetch `/api/status/<slug>/widget.json`, if any open incident → inject a fixed-position banner at top of page with title + state + link to the full status page.
- New `internal/adapter/http/widget_data.go` — `GET /api/status/:slug/widget.json` returning `{open_incidents: [{title, state, started_at, url}]}`. Cached 30s.
- Build pipeline: minify the widget on build; serve with `Cache-Control: public, max-age=86400, immutable`.
- Documentation page on `/docs/widget` showing the script tag to copy + customisation hooks (`data-position`, `data-theme`).
- Tests: Playwright — load a static HTML page that includes the widget script, assert the banner appears when an incident is open.

**Effort:** ~2 days.

---

## Task 8: Sprint 3 acceptance gates

- [ ] `make lint` clean, `make test` clean, `pnpm test:e2e` clean
- [ ] Manual: provision a custom domain end-to-end against LE staging (use a real domain you own)
- [ ] Manual: subscribe an email to your test status page, fire an incident, confirm email arrives
- [ ] Manual: drop the widget script into a static HTML file, simulate an incident, see the banner
- [ ] Spec §12 acceptance criteria for Sprint 3 ticked

---

## Out-of-sprint deferrals

- All SEO content → Sprint 4
- All distribution → Sprint 5

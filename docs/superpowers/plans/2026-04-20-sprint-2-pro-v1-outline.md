# Sprint 2 — Pro v1 + Viral Hooks · Outline

> **Status:** outline. Refine to full TDD plan via the writing-plans skill before execution. Effort estimates assume Sprint 1 has shipped and the Pro-gate middleware exists.

**Goal:** Make the Pro tier worth $9 by shipping the differentiating features promised on the pricing page — branded status pages, SSL warnings, retention + export, maintenance windows, SVG badge.

**Effort:** ~1.5–2 weeks of evening work.

**Source spec:** `docs/superpowers/specs/2026-04-20-seo-landing-sales-design.md` §6 Sprint 2.

---

## Task 1: Branded status page (Pro-only)

**Files:**
- New migration `internal/database/migrations/020_add_branding.sql` — add `logo_url`, `accent_color`, `custom_footer_text` to `users` (or a new `status_page_branding` table if multiple status pages per user are anticipated; current model is one slug per user, so columns on `users` is simpler).
- Modify `internal/sqlc/queries/users.sql` — `UpdateBranding`, `GetBranding`.
- Modify `api/openapi.yaml` — add `StatusPageBranding` schema, PATCH `/me/branding` (Pro-gated), include branding in existing `/status/:slug` response.
- Modify `frontend/app/status/[slug]/page.tsx` — render logo (if present), apply accent color via CSS var, hide "powered by PingCast" watermark when Pro.
- New `frontend/app/(main)/dashboard/branding/page.tsx` — form to upload logo (file → base64 or to object storage; for v1 keep it small + base64 in DB), pick color, save.
- Playwright: `frontend/tests/branding.spec.ts` — happy path + free-user 402.

**Test gates:** unit (color validation), integration (PATCH 402 for free, 200 for Pro), Playwright (visual smoke — logo + accent applied).

**Effort:** ~3 days.

---

## Task 2: SSL expiry warnings

**Files:**
- New `internal/adapter/checker/ssl_checker.go` — fetches the cert, parses `NotAfter`, emits a result with `expires_at` metadata.
- Modify `internal/adapter/checker/registry.go` (or wherever the checker registry lives) — register `ssl` as a check kind.
- New scheduler hook in `cmd/scheduler/` — once daily, scan monitors with `type=http` (Pro users only), fire `ssl.expiring` alerts at T-14d, T-7d, T-1d.
- Reuse the existing alert pipeline (`internal/app/alert.go`) — new alert type `ssl_expiring`.
- Frontend: render SSL expiry chip on monitor detail page.
- Tests: unit on the parser, integration covering "alert fires on a cert with NotAfter < 14d".

**Test gates:** unit (parser), integration (alert pipeline end-to-end with a test cert that's about to expire — use a httptest.Server with a generated short-lived cert).

**Effort:** ~2 days.

---

## Task 3: 1-year retention + CSV export

**Files:**
- Modify retention worker (find it via `grep -r "DeleteOlderThan"` — likely in a cron-like service in `internal/`). Change cutoff to `365d` for Pro users, keep `30d` for Free.
- New `internal/sqlc/queries/check_results.sql` — `DeleteOlderThanByPlan` joining users → monitors → check_results, branching on plan.
- New `GET /api/incidents/export.csv` — Pro-gated, streams CSV of incidents for the user's monitors.
- Frontend: "Export CSV" button on monitor detail page (Pro-gated visually + 402 gracefully).
- Tests: integration (user with 100d-old data; Free deletes, Pro keeps).

**Test gates:** integration covering retention + CSV format (header row + at least one row).

**Effort:** ~2 days.

---

## Task 4: Maintenance windows

**Files:**
- New migration `021_create_maintenance_windows.sql` — `(id, monitor_id, starts_at, ends_at, reason, created_at)`.
- New `internal/sqlc/queries/maintenance_windows.sql`.
- New `internal/domain/maintenance_window.go`.
- New `internal/port` repo interface, `internal/adapter/postgres` impl.
- Modify worker so checks during a window do not produce a check-failure alert (still record the result, but mark `during_maintenance=true`).
- Modify status page so it shows "Scheduled maintenance" for any monitor inside a window.
- API: `POST /maintenance-windows`, `GET /maintenance-windows`, `DELETE /maintenance-windows/{id}` — all Pro-gated.
- Frontend: dashboard page to schedule a window with date pickers.
- Playwright: schedule a window in the next 5 minutes, force a failed check, assert no alert fires (poll the alert log).

**Test gates:** unit (window-overlap predicate), integration (alert suppression).

**Effort:** ~3 days.

---

## Task 5: SVG status badge

**Files:**
- New `internal/adapter/http/badge.go` — `GET /status/:slug/badge.svg` — generate a small shields.io-styled SVG. No auth. Cache `Cache-Control: public, max-age=60`.
- The SVG should compute current state from the most recent check result for any monitor under the slug. States: `Operational` (green), `Degraded` (amber, if some monitors down), `Down` (red, if all down).
- Free tier: include "via PingCast" link via SVG `<a href>`. Pro: omit.
- Tests: integration — fetch badge for slug with all-up monitors, assert SVG content + cache header.

**Test gates:** integration covering 3 status colour states + free-tier link presence.

**Effort:** ~1 day.

---

## Task 6: Sprint 2 acceptance gates

- [ ] `make lint` — 0 findings
- [ ] `make test` — all unit + integration pass
- [ ] `pnpm test:e2e` — all Playwright pass
- [ ] Manual: as a Pro user, upload a logo + colour, see them on `/status/<slug>`
- [ ] Manual: as a Free user, hit `/api/branding` → 402; visit a public `/status/<slug>/badge.svg` → SVG renders with "via PingCast"
- [ ] Spec acceptance criteria §12 lines crossed off for Sprint-2 items

---

## Out-of-sprint deferrals

- Custom domain → Sprint 3
- Email subscriptions → Sprint 3
- Multi-monitor groups → Sprint 3
- JS widget → Sprint 3

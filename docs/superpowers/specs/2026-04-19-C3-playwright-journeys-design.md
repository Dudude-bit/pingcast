# C3 — Playwright Cross-Feature Journeys

Status: draft (pending user review)
Date: 2026-04-19
Follows: C2 (async pipeline tests). Precedes C4 (security + rate-limit).

## Purpose

Complement the backend-focused C1 (API contract) + C2 (async pipeline)
suites with browser-driven end-to-end checks of things that only a
browser can verify: cross-feature user journeys, client-side UI state
transitions, form rendering, dark mode, mobile viewport, SPA routing.

## Scope

**Existing Playwright coverage** (11 specs already in
`frontend/tests/`) is retained as-is. C3 adds NEW specs that cover
flows currently untested by either E2E or backend tests:

| # | Spec file | Scenario | What only a browser can verify |
|---|---|---|---|
| 1 | `onboarding.spec.ts` | signup → create monitor → add channel → bind → see bound channel in monitor detail | Full happy-path across three features end-to-end |
| 2 | `monitor_detail.spec.ts` | Chart renders, uptime % shown, incident list visible | Recharts / chart-data rendering |
| 3 | `dashboard_polling.spec.ts` | Insert monitor via DB → dashboard eventually shows it | TanStack Query refetchInterval actually working |
| 4 | `session_expiry.spec.ts` | Clear session cookie in mid-session → next protected click redirects to /login | Client-side redirect UX on 401 |
| 5 | `theme.spec.ts` | Toggle theme → localStorage → reload → persists | Dark mode state persistence |
| 6 | `mobile.spec.ts` | iPhone 14 viewport against landing + dashboard | Responsive breakpoints |
| 7 | `form_validation.spec.ts` | Submit register form with empty fields → inline UI error without 422 round-trip | Client-side form validation |
| 8 | `channel_types.spec.ts` | Create Telegram + Webhook via UI; verify email channel shows plan-upgrade hint on Free | UI surface of CreateChannel per type |
| 9 | `status_mobile.spec.ts` | Public `/status/{slug}` renders cleanly on iPhone viewport | Public-page mobile UX |

**Target count:** +9 specs. Combined Playwright suite: ~20.

## Out of scope

- Visual regression (screenshot diffing) — YAGNI.
- Performance budgeting / LCP — separate concern.
- Firefox / WebKit — Chromium-only for now; multi-browser is a
  future pass.
- Load / concurrency — covered by C1 + C2 at the API layer.

## Harness extensions

Everything reused from the existing Playwright setup:

- `global-setup.ts` — docker-compose Redis FLUSHDB (unchanged).
- `helpers.ts` — `registerFreshUser`, `uniqueEmail`, `uniqueSlug` (unchanged).
- `playwright.config.ts` — add a second project for mobile:

  ```ts
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
    { name: "mobile-chromium", use: { ...devices["iPhone 14"] } },
  ],
  ```

  `mobile.spec.ts` and `status_mobile.spec.ts` will be gated via
  `test.use({ viewport: devices["iPhone 14"].viewport })` or the
  mobile-chromium project, so desktop specs aren't re-run on mobile
  and vice-versa. Test tags or separate project filtering both work;
  we'll use `test.skip` based on project name to keep config simple.

- **New helpers** in `helpers.ts`:
  - `uiCreateMonitor(page, { name, url })` — navigates, fills form, submits, asserts
  - `uiCreateChannel(page, { name, type, config })` — same for channels
  - `uiBindChannel(page, monitorName, channelName)` — monitor detail → bind dropdown

## Test data strategy

Same as today: each test creates its own user via `registerFreshUser`
so no shared state between runs. Redis FLUSHDB in `beforeEach` (per
existing convention in `helpers.ts::flushRedis`).

## Where to run

Playwright needs a live frontend + API. Two options:

1. **Local** — `docker compose up` + `pnpm test:e2e`. Existing
   workflow.
2. **Against production** — `PLAYWRIGHT_BASE_URL=https://pingcast.kirillin.tech pnpm test:e2e`.
   Possible but creates real data in prod DB; not recommended for CI.

CI runs option 1: a new job in `.github/workflows/` that boots
docker-compose, waits for health, runs Playwright.

## Acceptance criteria

1. 9 new spec files under `frontend/tests/` — each passes on the
   current prod code.
2. Existing 11 Playwright specs still pass (no regressions).
3. `frontend/tests/helpers.ts` grows 3 UI-action helpers without
   removing existing exports.
4. `playwright.config.ts` declares a `mobile-chromium` project.
5. CI workflow runs both projects (chromium + mobile-chromium) in
   parallel, ~3-5 min total.
6. No flakes on the 3rd consecutive run (run 3x locally to verify).

## Risks / decisions deferred

- **Session-expiry test** requires `page.context().clearCookies({ name: "session_id" })`
  mid-test. Standard Playwright API.
- **Dashboard polling test** inserts directly into Postgres (the test
  stack already exposes it via docker-compose). This is the single
  place where a UI test reaches behind the API — acceptable because
  the goal is to verify the FETCH happens, and inserting via API
  would immediately show up in the next refetch too. Either path is
  fine; direct INSERT is slightly stronger because it simulates
  out-of-band state change.
- **Mobile viewport tests** will share most of the navigation logic
  with desktop tests. If duplication grows, extract common flows
  into helpers and parametrize viewport via `test.use`. First 2
  mobile specs are small enough that inline is fine.
- **Frontend unit tests** (vitest) — not in scope, but worth flagging
  for a future C-sub or A-track task.

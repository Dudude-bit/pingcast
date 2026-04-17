# C4 — Public Status Page + Frontend Cleanup — Design & Plan

**Date:** 2026-04-17
**Parent:** `2026-04-17-C-frontend-modernization-design.md` (C4 slice, final)

## Scope

Migrate `/status/[slug]` — the last Go-served HTML page — to Next.js with
SSR + Incremental Static Regeneration (ISR). After C4 the Go service
serves **only** `/api/*` routes. Delete `internal/web/` entirely. Minor
landing-page polish.

Response-time chart is still deferred (needs backend aggregation endpoint
— a dedicated tiny task later, not in C4).

## Routes shipped

| Path | Next.js file | Go handler deleted |
|---|---|---|
| `/status/[slug]` | `frontend/app/status/[slug]/page.tsx` — SSR + ISR revalidate=30s | `pageHandler.StatusPage` |

Go JSON endpoint `/api/status/{slug}` stays — it is what Next.js SSR
calls on the server side.

## Architecture decisions

### 1. ISR with `revalidate = 30`

Next.js App Router supports route-level `export const revalidate = 30`.
Each request serves cached HTML; if older than 30 s, a background
regeneration is triggered. Cache is per-URL, so each customer's status
slug gets independent caching.

This gives:
- Static-fast page loads for viewers (CDN + Next cache)
- Fresh data every 30 s without hammering Go
- SEO-indexable HTML with real content

### 2. Server-side fetching via `apiFetch` + `forwardSession()`

The status page is **public** — no session needed. So we call
`/api/status/{slug}` server-side without cookies. The existing
`apiFetch` on the server uses `INTERNAL_API_URL` pointing to Docker-
internal `http://api:8080/api`. Perfect.

### 3. Brand polish: make it look trustworthy

The status page is the most public-facing Next.js route after the
landing. It goes on customer subdomains. Invest in:
- Large hero "All systems operational" / "Some services degraded" with
  big status dot and timestamp "Updated just now"
- Per-monitor row with uptime % over 90d + coloured status pill
- Incident list (recent outages) if any
- "Powered by PingCast" footer for free tier only

### 4. Delete `internal/web/` entirely

After removing `statuspage.html` and its Go handler + route, `internal/web/`
contains only `static/css/style.css`, `static/js/chart-init.js`,
`static/favicon.*`, and the template loader shim. All are now dead —
Next.js owns the frontend. Delete the directory, `embed.go`, and static-
serving route in `setup.go`.

### 5. `cmd/api/main.go` drops page-handler wiring

After C4, the Go API doesn't render HTML. `PageHandler` itself still
exists because `Logout` handler lives on it — but it has almost nothing
left. Keep it for now; a future cleanup can migrate `POST /logout` to
the apigen-generated Server and delete `PageHandler` + `SetupApp` signature
cleanup. Out of scope for C4.

## Implementation plan

### Task 1 — Branch + shadcn needed

- [ ] `git checkout -b c4-status-cleanup`
- [ ] No new shadcn components needed.

### Task 2 — Add query + server-fetch helpers

- [ ] Extend `frontend/lib/queries.ts`:

```ts
export type StatusPage = components["schemas"]["StatusPageResponse"];
```

- [ ] No hook — status page is server-rendered. Direct `apiFetch` call.

### Task 3 — Status page (SSR + ISR)

- [ ] Create `frontend/app/status/[slug]/page.tsx`:

```tsx
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { apiFetch, ApiError } from "@/lib/api";
import type { components } from "@/lib/openapi-types";

export const revalidate = 30;

type StatusPage = components["schemas"]["StatusPageResponse"];

async function fetchStatus(slug: string): Promise<StatusPage | null> {
  try {
    return await apiFetch<StatusPage>(`/status/${slug}`);
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) return null;
    throw e;
  }
}

export async function generateMetadata(
  { params }: { params: Promise<{ slug: string }> },
): Promise<Metadata> {
  const { slug } = await params;
  const data = await fetchStatus(slug).catch(() => null);
  if (!data) return { title: "Status" };
  const title = data.all_up ? "All Systems Operational" : "System Status — Degraded";
  return { title: `${title} — ${data.slug}`, robots: { index: true, follow: true } };
}

export default async function StatusPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const data = await fetchStatus(slug);
  if (!data) notFound();

  // ... render ...
}
```

Full render includes:
- Hero card with allUp state
- Monitor rows (name, uptime %, status badge)
- Incident list if incidents
- "Powered by PingCast" footer when showBranding=true
- Revalidation note: "Updated every 30 seconds" (not showing last-fetch time — would flicker between cached variants)

- [ ] Create `frontend/app/status/[slug]/not-found.tsx` — clean 404 for
  unknown slugs.

### Task 4 — Status page header/layout

The status page should NOT have the main app Navbar+Footer (those link
to /dashboard etc., which this visitor doesn't have). Use a route-group
to isolate: `frontend/app/status/[slug]/layout.tsx` that overrides the
root layout minimally. Actually the existing root layout is wrapped at
`frontend/app/layout.tsx` and includes Navbar/Footer.

**Decision:** add a nested `app/status/layout.tsx` that returns just
`{children}` — but that doesn't remove the root navbar (Next.js nests
layouts). Instead: make the root-layout Navbar return null when path is
`/status/*`. Cleanest: put `Navbar` inside a route-group `(app)/` and
leave `status/` outside it. That's a larger refactor.

**Pragmatic choice:** leave the Navbar + Footer visible on the status
page, but style the status content to be visually dominant. The Navbar
has a "Login" / "Sign up" CTA which is fine for public visitors too —
it's gentle promotion of PingCast.

Document this deviation; revisit in a future polish pass.

### Task 5 — Delete Go status page handler + template

- [ ] Remove `pageHandler.StatusPage` from `pages.go`.
- [ ] Remove `/status/:slug` route from `setup.go`.
- [ ] Remove the `statuspage.html` from template loader (pages.go now
  has zero HTML templates — remove the templates map + render method too).
- [ ] Remove `/static/*` route from `setup.go` (nothing serves it any more).
- [ ] Delete `internal/web/templates/statuspage.html`.
- [ ] Delete `internal/web/templates/layout.html`.
- [ ] Delete `internal/web/static/` directory entirely.
- [ ] Delete `internal/web/embed.go`.
- [ ] Remove `internal/web` import from `pages.go` + anywhere else.
- [ ] `go build / vet / test / golangci-lint run` — all green.
- [ ] Commit.

### Task 6 — E2E for status page

- [ ] Append `frontend/tests/status.spec.ts`:

```ts
import { test, expect } from "@playwright/test";

test("unknown slug shows 404", async ({ page }) => {
  await page.goto("/status/does-not-exist-xyz");
  await expect(page).toHaveTitle(/status|404/i);
  // Next.js not-found may return 404 or a 404 page body.
  // Safe assertion: page has no status content.
  await expect(page.getByRole("heading", { name: /system status/i })).not.toBeVisible();
});

test("valid slug renders status page", async ({ page }) => {
  // Reuse a registered user from the auth test suite. Slug pattern: e2e*.
  // Create a user just for this test:
  const slug = `stat-${Date.now().toString(36).slice(-6)}`;
  const email = `stat-${slug}@example.com`;

  // Register via UI
  await page.goto("/register");
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Status page slug").fill(slug);
  await page.getByLabel("Password").fill("password123");
  await page.getByRole("button", { name: /create account/i }).click();
  await expect(page).toHaveURL(/\/dashboard/);

  // Visit the public status page
  await page.goto(`/status/${slug}`);
  await expect(page.getByRole("heading", { name: /system status/i })).toBeVisible();
});
```

- [ ] Run E2E.
- [ ] Commit.

### Task 7 — Final gate + merge main

- [ ] Docker stack up; smoke all routes; E2E.
- [ ] `go test -count=1 ./...` (full integration); `golangci-lint run`.
- [ ] `docker compose down`.
- [ ] ff-merge to main; delete branch.

## Success criteria

1. `/status/[slug]` served by Next.js, SSR, with `revalidate=30`.
2. Unknown slug → Next.js not-found page (404).
3. `internal/web/` deleted entirely.
4. Go `pages.go` has only `Logout` left (or PageHandler reduced /
   removed if trivial).
5. Go side: build/vet/test/race/lint all green, 0 lint findings.
6. E2E: 6+ passing (4 auth + 2 status).

## Out of scope

- Response-time chart on monitor detail (needs Go aggregation endpoint —
  future small task)
- Route-group refactor to hide Navbar on status pages (future polish)
- Custom domain support for status pages (`status.customer.com`) — future
- Status page subscribe/email notifications — future

## Risks

| Risk | Mitigation |
|---|---|
| ISR cache staleness vs 30s target | First request after cache expiry triggers regeneration; next request gets fresh HTML. Acceptable. |
| SSR fetch fails because Go container unreachable | `notFound()` path handles 404; other errors surface as Next.js error boundary (500). |
| Removing static/ breaks existing deployments that relied on `/static/favicon.svg` URL | Favicons already copied to `frontend/public/` in C1. No external callers. |
| `PageHandler` becomes almost empty (only Logout) | Acceptable — Logout can move to apigen Server in a future tiny PR. |

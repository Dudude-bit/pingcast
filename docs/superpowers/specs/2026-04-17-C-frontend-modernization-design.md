# C — Frontend Modernization — Architecture Design

**Date:** 2026-04-17
**Parent:** Sub-project C of the main project-wide decomposition
**Scope:** Replace the Go `html/template` server-rendered frontend with a
Next.js 14 SPA (App Router + SSR + ISR) hosted alongside the Go API.
Deliver max UI quality, best-in-class UX, and competitive SEO.

## Goal

pingcast today: a single-binary Go service with `html/template` HTML +
941 lines of hand-written CSS + minimal JS. Users get a functional but
dated experience; landing page lacks the polish needed to attract
signups. SEO is served by server-rendered HTML but there's no
content-first marketing structure.

The goal: a frontend that feels like Linear / Vercel / Planetscale —
polished, fast, animated, mobile-first, with SSR/SSG for SEO, and a
component library we own and can iterate on.

## Stack Decision

| Concern | Choice | Rationale |
|---|---|---|
| Framework | **Next.js 14 (App Router)** | Largest ecosystem, best AI-tooling support, RSC + streaming SSR, SSG + ISR |
| Styling | **Tailwind CSS v4** | Co-located utility classes; fastest iteration; plays with shadcn/ui |
| Component library | **shadcn/ui** | 50+ polished accessible components, copy-owned (no npm dep drift), built on Radix primitives |
| Additional blocks | **Tremor + shadcn blocks** | Dashboard-ready chart/card blocks that match shadcn |
| Charts | **Recharts** (shadcn default) + **uPlot** if performance matters | Initial: Recharts for ease; if dashboard needs high-frequency updates, swap to uPlot |
| Animations | **Framer Motion** | Industry-standard for React app animations; reduced-motion respected |
| Icons | **Lucide** | 1000+ icons, consistent stroke width, matches shadcn aesthetic |
| Data fetching | **TanStack Query** | Cache + background refetch + optimistic updates; replaces ad-hoc fetch |
| Types | **openapi-typescript** | Generates TS types from pingcast's existing OpenAPI spec — one source of truth |
| Forms | **React Hook Form** + **Zod** | Fast, declarative, shadcn wraps them |
| Runtime | **Node.js 20** in `node:20-alpine` Docker image | Stable LTS; small image |

## Deployment Architecture

Two containers in `docker-compose.yml`, fronted by the existing Traefik
reverse proxy (already in use for SSL / host routing):

```
            ┌──────────────────────────┐
            │  Traefik (existing)      │
            │  pingcast.io → router    │
            └──┬──────────────────┬───┘
               │                   │
      Host=*  path=/api/*         everything else
               │                   │
               ▼                   ▼
      ┌────────────────┐  ┌────────────────┐
      │ pingcast-api   │  │ pingcast-web   │
      │ Go, :8080      │  │ Next.js, :3000 │
      │ (unchanged)    │  │ (new)          │
      └────┬───────────┘  └────┬───────────┘
           │                    │  (server actions)
           │ ◄──────────────────┘
           │
           ▼
      Postgres / Redis / NATS (unchanged)
```

**Session auth preserved:** Next.js pages and server actions forward the
`session_id` cookie to the Go API verbatim — no new auth code. The
browser's cookie is set by Go on login as today. Next.js never holds
credentials directly.

**API requests:** all data calls (`/api/monitors`, `/api/channels`, etc.)
go through Traefik to Go. Next.js does not proxy; the browser calls the
Go API directly via `NEXT_PUBLIC_API_URL=/api`. Same-origin, cookies
attach automatically.

**SSR requests (Next.js → Go):** server-side components that need data
(e.g., `/status/[slug]` rendering) call Go API at the internal Docker
network address `http://pingcast-api:8080/api` — no Traefik round trip.

## Repo layout

New top-level directory:

```
pingcast/
  frontend/                 # Next.js project
    app/                    # App Router pages
      (marketing)/          # public routes (landing, pricing, blog?)
      (app)/                # authenticated routes (dashboard, monitors, channels)
      status/[slug]/        # public status pages (SSR with ISR)
      api-keys/
      login/
      register/
      layout.tsx
    components/
      ui/                   # shadcn components (copied in)
      features/             # feature-specific composed components
    lib/
      api.ts                # fetch wrapper with session-cookie auth
      types.ts              # re-export openapi-typescript generated
    styles/
      globals.css           # Tailwind + CSS vars
    package.json
    Dockerfile
    next.config.ts
  docker-compose.yml        # updated: adds pingcast-web service
  internal/web/             # DELETED in C4 (after all pages migrated)
```

## Decomposition into slices C1-C4

Each slice lands as a working increment on `main`; the Go html/template
frontend stays active until **C4** when it's deleted.

### C1 — Foundation + auth + landing
- Scaffold Next.js project in `frontend/`
- Tailwind v4, shadcn/ui init, Tremor install
- Docker image + docker-compose.yml update + Traefik rules
- API client (`lib/api.ts`) with session-cookie forwarding
- Types generated from OpenAPI spec
- Layout shell (navbar, footer) matching design system
- **Landing page** — marketing-grade hero, features, CTA, SEO metadata
- **Login page**, **Register page** (forms posting to existing Go endpoints)
- Deletes: corresponding Go templates (`landing.html`, `login.html`,
  `register.html`, `layout.html` partials used only by these)

Parallel access: old Go frontend still serves `/dashboard`, `/monitors`,
etc.; Next.js handles `/`, `/login`, `/register` during C1. Traefik
routing rule evolves in each subsequent slice.

### C2 — Dashboard + Monitors
- `/dashboard` — monitor list with live-refreshing status (TanStack Query
  `refetchInterval: 15s`)
- `/monitors/new`, `/monitors/[id]`, `/monitors/[id]/edit` — typed forms
  with React Hook Form + Zod schema generated from OpenAPI
- Response-time chart — Tremor or Recharts, reading from
  `/api/monitors/:id/check-results` (new endpoint if not there)
- Incident list with polish
- Deletes: `dashboard.html`, `monitor_form.html`, `monitor_detail.html`,
  `monitor_config_fields.html`

### C3 — Channels + API keys + settings
- `/channels`, `/channels/new`, `/channels/[id]/edit`
- `/api-keys` with "copy key on create" toast pattern
- Future: `/settings` for profile, plan, billing
- Deletes: `channels.html`, `channel_form.html`, `api_keys.html`,
  `api_key_form.html`

### C4 — Public status page + cleanup
- `/status/[slug]` with SSG + ISR (Incremental Static Regeneration —
  revalidate every 30s so SEO-friendly static HTML is always recent)
- Custom domain support (future: `status.customer.com` via CNAME +
  Traefik wildcard)
- Delete `internal/web/` entirely
- Migrate any remaining Go `pages.go` routes
- Final: `cmd/api` only serves `/api/*` + healthz; all HTML is Next.js

## Shared infrastructure details (C1 lays this down)

### API client (`frontend/lib/api.ts`)

```ts
// Wraps fetch with:
// - credentials: "include" (session cookie)
// - automatic JSON parsing
// - typed responses via generated types
// - server-side: uses internal Docker DNS; client-side: uses /api relative
import type { paths } from "./openapi-types";

const baseUrl = typeof window === "undefined"
  ? process.env.INTERNAL_API_URL ?? "http://pingcast-api:8080"
  : "/api";

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${baseUrl}${path}`, {
    credentials: "include",
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
  });
  if (!res.ok) throw new ApiError(res.status, await res.text());
  return res.json();
}
```

### Type generation

CI step (or postinstall hook) runs:
```
openapi-typescript ../api/openapi.yaml -o frontend/lib/openapi-types.ts
```

The generated file is committed — engineers see types in PRs. Matches
how `sqlc` generates types from SQL.

### Auth flow

1. User visits `/login` (Next.js)
2. Submits form → Next.js server action → POST to Go `/api/auth/login`
3. Go responds with `Set-Cookie: session_id=…` + 200
4. Next.js server action forwards the `Set-Cookie` header to the browser
5. Next.js redirects to `/dashboard`
6. `/dashboard` server component fetches `/api/monitors` with
   `credentials: "include"` — browser attaches the session cookie
7. Logout is symmetric

`middleware.ts` in Next.js guards `/dashboard/*` etc. by checking if
`session_id` cookie exists; if not, redirect to `/login`. Full session
validity is still Go's responsibility — middleware is only a fast-path
gate.

### Design tokens

`tailwind.config.ts` defines CSS variables mirroring the shadcn
convention (`--primary`, `--muted`, `--destructive`, etc.) plus a small
pingcast-specific palette for status colors (up/down/unknown). Dark mode
is supported via the `class` strategy; a toggle lands in C2.

## Migration strategy

**Parallel coexistence per slice.** Traefik has explicit routing rules
so `/login` → Next.js, `/dashboard` → Go (in C1). Each subsequent slice
moves more routes to Next.js and deletes corresponding Go templates in
the same commit.

**No big-bang rewrite.** Each C-slice is an independent working
increment that produces real user value.

**Rollback:** each slice is a feature branch → fast-forward to main. If
a slice regresses, `git revert` the merge commit — Traefik routes revert
too.

## SEO strategy

- **Landing page** (`/`): fully static HTML with LCP < 1s; metadata for
  Open Graph, Twitter Cards, structured data (FAQ schema).
- **Status pages** (`/status/[slug]`): ISR with 30s revalidation;
  rendered as if static from Google's POV; fast TTFB from CDN cache.
- **App pages** (`/dashboard`, etc.): not indexed (robots.txt disallow +
  `<meta name="robots" content="noindex">`). SEO isn't a concern.
- **Sitemap.xml** + **robots.txt** generated by Next.js built-ins.

## Testing strategy

- **Unit:** components with Vitest + React Testing Library.
- **E2E:** Playwright, hits real Go API in test mode (testcontainers
  postgres). Runs in CI.
- **Lighthouse CI:** landing + status pages gated at LCP < 1.5s, CLS <
  0.1, performance > 90.
- **Visual regression:** Storybook + Chromatic optional; defer until C4.

## Out of scope (for now)

- Mobile native app — JSON API is ready when we want it.
- Internationalisation — all English for launch.
- Real-time push (WebSocket / SSE) for dashboard — polling is fine at
  MVP scale. If we hit >1k concurrent users, revisit with SSE from Go.
- Payment UI refresh — LemonSqueezy hosted checkout stays.

## Risks

| Risk | Mitigation |
|---|---|
| Next.js Docker build is heavy (~500 MB image) | Multi-stage build: builder (1.2 GB) → runtime (200 MB Alpine). Use `output: 'standalone'` to strip non-essential deps. |
| Session cookie doesn't forward correctly through Next.js server actions | C1 Task includes an integration test: login → dashboard round-trip, both via Next.js. |
| Tailwind class churn makes diffs hard to review | Use shadcn patterns; don't invent one-off utilities. Class-variance-authority for variants. |
| openapi-typescript drift vs Go OpenAPI | Regenerate on every Go handler change; CI checks diff is clean. |
| Node runtime adds DevOps complexity | `docker-compose up` is still one command. Production deploy on Dokploy — Dokploy supports multi-service compose natively. |
| Bundle size inflates over time | Bundle analyzer in CI; gate first-load JS < 150 KB on landing, < 250 KB on dashboard. |

## Success criteria (project-wide)

1. All 13 Go HTML templates deleted by end of C4.
2. Lighthouse: landing + status page ≥ 95 Performance, ≥ 100 SEO.
3. First-load JS: landing < 100 KB gzip; dashboard < 250 KB gzip.
4. Core Web Vitals: LCP < 1.5s, INP < 200ms, CLS < 0.1.
5. Design consistency: zero inline styles; every color/spacing uses
   Tailwind tokens.

---

## C1 detailed scope (this design committed now covers C1; C2-C4 get
their own specs at their turn)

### Deliverables

- [ ] `frontend/` scaffold with Next.js 14 App Router + Tailwind v4 + shadcn init
- [ ] `frontend/Dockerfile` (multi-stage, `output: standalone`)
- [ ] `docker-compose.yml` — new `pingcast-web` service
- [ ] Traefik routing update — `/login`, `/register`, `/` → `pingcast-web`; everything else → `pingcast-api`
- [ ] `frontend/lib/api.ts` + generated `openapi-types.ts`
- [ ] `frontend/app/layout.tsx` — global layout (navbar + footer)
- [ ] `frontend/app/(marketing)/page.tsx` — landing page
- [ ] `frontend/app/login/page.tsx` — login form + server action
- [ ] `frontend/app/register/page.tsx` — register form + server action
- [ ] `frontend/middleware.ts` — minimal auth gate for `/dashboard` etc. (redirects to `/login`, even though dashboard still served by Go — so when user hits `/dashboard` directly we still work)
- [ ] Delete `internal/web/templates/{landing,login,register}.html` from the Go side after Traefik routing is live
- [ ] Update `cmd/api/main.go` — stop registering handlers for `/login`, `/register`, `/` (keep them JSON-only under `/api/auth/*`)

### Files that change in Go

- `cmd/api/main.go` — drop page handler registrations for migrated routes
- `internal/adapter/http/pages.go` — delete `Landing`, `LoginPage`, `LoginSubmit`, `RegisterPage`, `RegisterSubmit` methods (move logic to API JSON handlers if not already there)
- `internal/adapter/http/setup.go` — route list shrinks
- `internal/web/templates/` — delete 3 .html files

### Critical integration tests (Playwright)

- `frontend/tests/auth.spec.ts`:
  - `user can register, gets redirected to /dashboard`
  - `user can login with correct credentials`
  - `login with wrong password shows 'invalid email or password'`
  - `registration with existing email shows 'registration failed' (enumeration-safe check)`

### Out of scope for C1

- Dashboard, monitors, channels, api-keys pages (→ C2/C3)
- Status page (→ C4)
- Dark mode toggle (→ C2)
- Animations beyond the baseline Framer Motion setup

### C1 commit structure

Probably **3 commits** on branch `c1-frontend-foundation`:
1. `feat: C1a — scaffold Next.js frontend + Docker + Traefik routing`
2. `feat: C1b — landing + login + register pages`
3. `refactor: C1c — delete migrated Go templates + handlers`

### C1 rollback

Standard: fast-forward merge into `main`; `git revert` on failure.
Because Go templates stay present until commit 3, a partial regression
in commits 1-2 is a no-op (nothing is deleted yet).

---

## What happens after C1 is approved

I'll invoke `superpowers:writing-plans` to produce a bite-sized
implementation plan for C1 (task by task, TDD where testable). C2-C4
each get their own design spec when we get to them — they can be
brainstormed then, since user feedback on C1 may reshape them.

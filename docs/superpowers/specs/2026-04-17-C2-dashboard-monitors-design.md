# C2 — Dashboard + Monitors — Design & Plan

**Date:** 2026-04-17
**Parent:** `2026-04-17-C-frontend-modernization-design.md` (C2 slice)
**Stack established in C1:** Next.js 16 + Tailwind v4 + shadcn/ui (base-nova) + lucide-react + framer-motion + @tanstack/react-query + openapi-typescript

## Goal

Replace the Go-rendered `/dashboard`, `/monitors/*` HTML pages with polished Next.js routes that call the existing JSON API. Dashboard auto-refreshes every 15 s. Forms use React Hook Form + Zod. Monitor detail shows uptime stats + incidents; real-time chart is scoped to C4 (needs Go-side aggregation work).

## Routes shipped

| Path | Next.js file | Go handler deleted |
|---|---|---|
| `/dashboard` | `frontend/app/dashboard/page.tsx` (client) | `pageHandler.Dashboard` |
| `/monitors/new` | `frontend/app/monitors/new/page.tsx` (client) | `pageHandler.MonitorNewForm` + `MonitorCreate` |
| `/monitors/[id]` | `frontend/app/monitors/[id]/page.tsx` (client) | `pageHandler.MonitorDetail` |
| `/monitors/[id]/edit` | `frontend/app/monitors/[id]/edit/page.tsx` (client) | `pageHandler.MonitorEditForm` + `MonitorUpdate` |
| (action) pause/delete | Server actions in Next.js | `pageHandler.MonitorDelete` + `MonitorTogglePause` |

Go `/api/monitors*` JSON endpoints stay — they are what Next.js calls.

## Components added (shadcn + local)

shadcn add: `dropdown-menu`, `alert-dialog`, `badge`, `skeleton`, `separator`, `select`, `checkbox`, `switch`, `toast` (via Sonner which shadcn ships).

Local `frontend/components/features/monitors/`:
- `monitor-list.tsx` — client component with TanStack Query polling
- `monitor-row.tsx` — single row with status dot, uptime, actions dropdown
- `status-badge.tsx` — up/down/unknown badge
- `monitor-form.tsx` — RHF + Zod form for create/edit
- `config-fields.tsx` — dynamic fields per monitor type (HTTP URL, TCP host:port, DNS record)
- `uptime-stats.tsx` — 24h/7d/30d stat cards
- `incident-list.tsx` — monitor detail incident section
- `delete-monitor-dialog.tsx` — shadcn AlertDialog confirmation

Local `frontend/lib/`:
- `queries.ts` — TanStack Query hook factories (useMonitors, useMonitor, useMonitorTypes, useChannels)
- `mutations.ts` — mutation hooks (createMonitor, updateMonitor, deleteMonitor, toggleMonitorPause)

## Architecture decisions

### 1. All C2 pages are client components (`"use client"`)

Rationale: every page is authenticated + interactive. SSR gives zero SEO benefit on `/dashboard`. Client components enable TanStack Query's cache + polling without SSR→CSR handover headaches. RSC-streaming optimisations are C4 scope if ever needed.

### 2. TanStack Query is the data layer

A single `<QueryClientProvider>` wraps the app at `app/providers.tsx`, imported from `app/layout.tsx`. Every fetch goes through typed query hooks; the raw `apiFetch` is only used inside hook implementations, not directly in components.

### 3. Forms: React Hook Form + Zod + shadcn

The Zod schema for create/update is hand-written to match the OpenAPI `CreateMonitorRequest` / `UpdateMonitorRequest` shapes. The generated `openapi-types.ts` types the hook return values; Zod types the form.

### 4. Real-time chart deferred to C4

Current Go `GetMonitor` handler returns `chart_data: null`. Populating it requires a new Go aggregation query (hourly avg response time over 24 h). Instead, C2 ships uptime-stat bars and a "Response-time chart coming soon" placeholder. C4 (status page rework) will add the backend aggregation and the chart simultaneously.

### 5. Pause / Delete via DropdownMenu

Per-row dropdown with "Edit", "Pause"/"Resume", "Delete". Delete opens AlertDialog. Mutations use TanStack Query's optimistic updates + Sonner toast for success/error.

## Implementation plan (bite-sized tasks)

### Task 1 — Branch + shadcn add extras

- [ ] `git checkout -b c2-dashboard-monitors`
- [ ] `cd frontend && pnpm dlx shadcn@latest add dropdown-menu alert-dialog badge skeleton separator select checkbox switch sonner -y`
- [ ] Commit: `feat: C2 — shadcn extras (menu, dialog, badge, skeleton, etc.)`

### Task 2 — TanStack Query provider + wiring

- [ ] Create `frontend/app/providers.tsx`:

```tsx
"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Toaster } from "@/components/ui/sonner";
import { useState } from "react";

export function Providers({ children }: { children: React.ReactNode }) {
  const [client] = useState(() => new QueryClient({
    defaultOptions: {
      queries: { refetchOnWindowFocus: false, staleTime: 5_000 },
    },
  }));
  return (
    <QueryClientProvider client={client}>
      {children}
      <Toaster position="bottom-right" richColors />
    </QueryClientProvider>
  );
}
```

- [ ] Update `frontend/app/layout.tsx` to wrap `{children}` in `<Providers>`.
- [ ] Build check; commit.

### Task 3 — Query + mutation hooks

- [ ] Create `frontend/lib/queries.ts`:

```ts
"use client";

import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "./api";
import type { components } from "./openapi-types";

type Monitor = components["schemas"]["Monitor"];
type MonitorWithUptime = components["schemas"]["MonitorWithUptime"];
type MonitorDetail = components["schemas"]["MonitorDetail"];
type MonitorTypeInfo = components["schemas"]["MonitorTypeInfo"];
type Channel = components["schemas"]["NotificationChannel"];

export function useMonitors() {
  return useQuery({
    queryKey: ["monitors"],
    queryFn: () => apiFetch<MonitorWithUptime[]>("/monitors"),
    refetchInterval: 15_000,
  });
}

export function useMonitor(id: string | undefined) {
  return useQuery({
    queryKey: ["monitors", id],
    queryFn: () => apiFetch<MonitorDetail>(`/monitors/${id}`),
    enabled: Boolean(id),
    refetchInterval: 15_000,
  });
}

export function useMonitorTypes() {
  return useQuery({
    queryKey: ["monitor-types"],
    queryFn: () => apiFetch<MonitorTypeInfo[]>("/monitor-types"),
    staleTime: Infinity,
  });
}

export function useChannels() {
  return useQuery({
    queryKey: ["channels"],
    queryFn: () => apiFetch<Channel[]>("/channels"),
  });
}

export type { Monitor, MonitorWithUptime, MonitorDetail, MonitorTypeInfo, Channel };
```

- [ ] Create `frontend/lib/mutations.ts`:

```ts
"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "./api";
import { toast } from "sonner";
import type { components } from "./openapi-types";

type CreateReq = components["schemas"]["CreateMonitorRequest"];
type UpdateReq = components["schemas"]["UpdateMonitorRequest"];
type Monitor = components["schemas"]["Monitor"];

export function useCreateMonitor() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: CreateReq) => apiFetch<Monitor>("/monitors", { method: "POST", body }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["monitors"] });
      toast.success("Monitor created");
    },
    onError: (e) => toast.error(`Create failed: ${e.message}`),
  });
}

export function useUpdateMonitor(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: UpdateReq) => apiFetch<Monitor>(`/monitors/${id}`, { method: "PUT", body }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["monitors"] });
      qc.invalidateQueries({ queryKey: ["monitors", id] });
      toast.success("Monitor updated");
    },
    onError: (e) => toast.error(`Update failed: ${e.message}`),
  });
}

export function useDeleteMonitor() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => apiFetch<void>(`/monitors/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["monitors"] });
      toast.success("Monitor deleted");
    },
    onError: (e) => toast.error(`Delete failed: ${e.message}`),
  });
}

export function useTogglePause() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => apiFetch<Monitor>(`/monitors/${id}/pause`, { method: "POST" }),
    onSuccess: (_, id) => {
      qc.invalidateQueries({ queryKey: ["monitors"] });
      qc.invalidateQueries({ queryKey: ["monitors", id] });
    },
    onError: (e) => toast.error(`Toggle failed: ${e.message}`),
  });
}
```

- [ ] Build check; commit.

### Task 4 — Dashboard page (monitor list with live refresh)

- [ ] Create `frontend/app/dashboard/page.tsx` as a client component using `useMonitors()` hook.
- [ ] Create `frontend/components/features/monitors/monitor-list.tsx` — renders list of `<MonitorRow>` or empty state.
- [ ] Create `frontend/components/features/monitors/monitor-row.tsx` — status dot, name, target, interval, uptime %, actions dropdown.
- [ ] Create `frontend/components/features/monitors/status-badge.tsx` — up (green) / down (red) / unknown (gray).
- [ ] Create `frontend/components/features/monitors/delete-monitor-dialog.tsx` — AlertDialog wrapping delete mutation.
- [ ] Skeleton while loading (shadcn Skeleton); empty-state CTA "+ New Monitor".
- [ ] Build check; Docker rebuild; smoke `curl http://localhost:3001/dashboard` with session cookie → 200.
- [ ] Commit: `feat: C2 — /dashboard with live-refreshing monitor list`

### Task 5 — Monitor create form

- [ ] Create `frontend/components/features/monitors/monitor-form.tsx` — RHF + Zod, fields: name, type (Select), dynamic config fields, interval (Select), alert_after_failures (Input), is_public (Switch), channel_ids (Checkbox list).
- [ ] Create `frontend/components/features/monitors/config-fields.tsx` — given a `type`, renders the right fields (HTTP: url/method/expected_status/keyword; TCP: host/port; DNS: domain/record_type/expected_value).
- [ ] Create `frontend/app/monitors/new/page.tsx` — renders `<MonitorForm mode="create">`.
- [ ] Zod schema shared in `frontend/lib/schemas/monitor.ts`.
- [ ] On submit → `useCreateMonitor().mutate()` → router.push(`/monitors/${data.id}`).
- [ ] Build check; commit.

### Task 6 — Monitor detail page

- [ ] Create `frontend/components/features/monitors/uptime-stats.tsx` — three stat cards (24h / 7d / 30d) with coloured %.
- [ ] Create `frontend/components/features/monitors/incident-list.tsx` — list of incidents, "No incidents" empty state.
- [ ] Create `frontend/app/monitors/[id]/page.tsx` — uses `useMonitor(id)`, renders header (name, status badge, edit/delete buttons), stats grid, chart placeholder card, incidents.
- [ ] Chart placeholder: card with muted text "Response-time chart — coming soon" (full chart in C4).
- [ ] Build check; commit.

### Task 7 — Monitor edit form

- [ ] Create `frontend/app/monitors/[id]/edit/page.tsx` — loads monitor via `useMonitor(id)`, renders `<MonitorForm mode="edit" initial={...}>`.
- [ ] In edit mode, `type` field is disabled (can't change monitor type after creation).
- [ ] On submit → `useUpdateMonitor(id).mutate()` → toast + router.push(`/monitors/${id}`).
- [ ] Build check; commit.

### Task 8 — E2E tests

- [ ] Append `frontend/tests/dashboard.spec.ts`:
  - Register+login → empty dashboard empty state present.
  - Create monitor → redirect to detail → name visible → back to dashboard shows the row.
  - Edit monitor → rename → detail shows new name.
  - Toggle pause → row meta changes.
  - Delete monitor → confirm in dialog → dashboard empty state returns.
- [ ] Run E2E: `cd frontend && pnpm test:e2e`.
- [ ] Commit.

### Task 9 — Delete migrated Go handlers + templates

- [ ] Remove `pageHandler.Dashboard`, `MonitorNewForm`, `MonitorCreate`, `MonitorDetail`, `MonitorEditForm`, `MonitorUpdate`, `MonitorDelete`, `MonitorTogglePause`, `ChannelConfigFields` from `internal/adapter/http/pages.go`.
- [ ] Drop route registrations in `internal/adapter/http/setup.go`:
  - `/dashboard`, `/monitors/new`, `/monitors`, `/monitors/:id/edit`, `/monitors/:id`, `/monitors/:id/edit` (POST), `/monitors/:id/pause`, `/monitors/:id/delete`, `/monitors/config-fields`.
- [ ] Remove from template loader slice: `"dashboard.html"`, `"monitor_form.html"`, `"monitor_detail.html"`, `"monitor_config_fields.html"`.
- [ ] Delete those 4 template files.
- [ ] Remove `buildCheckConfigFromForm`, `parseIntInRange` (pages-only helper), `renderMonitorFormError` if they're only used by removed handlers.
- [ ] Drop any handler tests in `handler_test.go` referencing these routes.
- [ ] `go build ./... && go vet ./... && go test -count=1 -short ./...` — green.
- [ ] `golangci-lint run` — still 0 findings.
- [ ] Commit.

### Task 10 — Final gate + merge main

- [ ] `docker compose up -d api db redis nats web`; wait; full E2E.
- [ ] `go test -count=1 ./...` (full suite incl. integration); `golangci-lint run` — green.
- [ ] `docker compose down`.
- [ ] `git checkout main && git merge --ff-only c2-dashboard-monitors && git branch -d c2-dashboard-monitors`.
- [ ] Do NOT push.

## Success criteria

1. `/dashboard`, `/monitors/new`, `/monitors/[id]`, `/monitors/[id]/edit` all served by Next.js and return 200 with valid session.
2. Dashboard auto-refreshes every 15 s (verified by watching TanStack Query devtools or by seeing a check result land without navigation).
3. Create / edit / delete / pause all work end-to-end with toast notifications.
4. 5+ new Playwright tests pass.
5. Go side still builds, tests pass, zero lint findings.
6. 4 Go templates removed; ~8 Go handlers removed.

## Out of scope for C2

- Real-time response-time chart (→ C4 alongside Go aggregation endpoint)
- Dark mode toggle (future)
- Channel management pages (→ C3)
- API keys pages (→ C3)
- Public status page (→ C4)

## Risks

| Risk | Mitigation |
|---|---|
| TanStack Query polling causes visible flashes | `placeholderData: previousData` or `keepPreviousData` on list queries |
| Dynamic config fields desync with Go validator | Zod schema is loose (`z.any()` for check_config); Go rejects invalid config; toast shows server error |
| Delete AlertDialog gets confused with multiple rows | Each row owns its own dialog state with `useState` |
| Empty `chart_data` from Go looks broken | Explicit "coming soon" placeholder card, not a broken chart |

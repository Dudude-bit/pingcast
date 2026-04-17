# C3 — Channels + API Keys — Design & Plan

**Date:** 2026-04-17
**Parent:** `2026-04-17-C-frontend-modernization-design.md` (C3 slice)

## Scope

Migrate the remaining authenticated HTML pages — notification channels CRUD
and API keys CRUD — to Next.js. After C3 the only Go-served HTML left is
the public `/status/:slug` page, which C4 will handle. `/logout` stays as
a Go JSON endpoint (POST /api/auth/logout).

## Routes shipped

| Path | Next.js file | Go handler deleted |
|---|---|---|
| `/channels` | `frontend/app/channels/page.tsx` | `pageHandler.ChannelList` |
| `/channels/new` | `frontend/app/channels/new/page.tsx` | `pageHandler.ChannelNewForm` + `ChannelCreate` |
| `/channels/[id]/edit` | `frontend/app/channels/[id]/edit/page.tsx` | `pageHandler.ChannelEditForm` + `ChannelUpdate` |
| (action) delete channel | mutation | `pageHandler.ChannelDelete` |
| `/api-keys` | `frontend/app/api-keys/page.tsx` | `pageHandler.APIKeyList` + `APIKeyCreate` + `APIKeyCreateSubmit` + `APIKeyRevoke` |

Go JSON API (`/api/channels*`, `/api/api-keys*`, `/api/channel-types`) unchanged.

## Architecture decisions

### 1. "Copy the raw key once" modal for API key creation

API key creation returns the raw key **once only** (the server stores only
SHA-256 hash). The UI must show the key in a dialog with a Copy button and
a warning that it won't be shown again. Closing the dialog navigates back
to the list.

### 2. Channel form reuses the dynamic ConfigFields pattern from C2

The C2 `ConfigFields` component is generic over any `MonitorTypeInfo` /
`ChannelTypeInfo` shape — both expose `schema.fields[]`. We'll
slightly rename the component or introduce a generic variant.

**Decision:** introduce `frontend/components/features/common/dynamic-config-fields.tsx`
that accepts a `fields` prop directly (not the whole TypeInfo). Both
monitors and channels pass `typeInfo?.schema?.fields` to it. No breaking
change to existing C2 usage.

### 3. DELETE confirmations reuse pattern

`DeleteMonitorDialog` was monitor-specific. For C3 we generalise:
`frontend/components/features/common/confirm-destructive-dialog.tsx` —
controlled AlertDialog with generic title/body/onConfirm props. Monitor's
existing file is refactored to use the common component.

## Implementation plan (tasks)

### Task 1 — Branch + shadcn extras needed

- [ ] `git checkout -b c3-channels-apikeys`
- [ ] `cd frontend && pnpm dlx shadcn@latest add textarea -y`  (for API key scope hints if needed; also required by channel webhook URL)
- [ ] Commit minor deps.

### Task 2 — Refactor: extract common ConfirmDestructiveDialog + DynamicConfigFields

- [ ] Create `frontend/components/features/common/confirm-destructive-dialog.tsx`:

```tsx
"use client";

import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from "@/components/ui/alert-dialog";

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: React.ReactNode;
  confirmLabel?: string;
  pending?: boolean;
  onConfirm: () => void | Promise<void>;
}

export function ConfirmDestructiveDialog({
  open, onOpenChange, title, description,
  confirmLabel = "Delete", pending, onConfirm,
}: Props) {
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          <AlertDialogDescription>{description}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            onClick={onConfirm}
            disabled={pending}
            className="bg-red-600 text-white hover:bg-red-700"
          >
            {pending ? `${confirmLabel}…` : confirmLabel}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
```

- [ ] Refactor `DeleteMonitorDialog` to use it internally (retains its
  specific monitor-deletion + redirect logic).
- [ ] Create `frontend/components/features/common/dynamic-config-fields.tsx` —
  the same code as `monitors/config-fields.tsx` but takes `fields` directly
  (not typeInfo). Update `monitors/config-fields.tsx` to re-export or remove.
- [ ] Build check; commit.

### Task 3 — Channels query + mutation hooks

- [ ] Extend `frontend/lib/queries.ts` with `useChannelTypes()`.
- [ ] Extend `frontend/lib/mutations.ts` with
  `useCreateChannel`, `useUpdateChannel`, `useDeleteChannel`.
- [ ] Build; commit.

### Task 4 — Channels list page

- [ ] Create `frontend/components/features/channels/channel-row.tsx` —
  status dot (enabled/disabled), name, type, edit/delete dropdown.
- [ ] Create `frontend/components/features/channels/channel-list.tsx` —
  skeleton + empty-state + list of rows (same pattern as monitor list).
- [ ] Create `frontend/app/channels/page.tsx` — header with "+ New channel",
  renders the list.
- [ ] Commit.

### Task 5 — Channel create + edit forms

- [ ] Create `frontend/components/features/channels/channel-form.tsx` —
  RHF form: name, type (Select, disabled on edit), dynamic `config.*` via
  `DynamicConfigFields`, is_enabled (Switch, edit-only).
- [ ] Create `frontend/app/channels/new/page.tsx`.
- [ ] Create `frontend/app/channels/[id]/edit/page.tsx` — loads channel via
  `useChannels()`, filters to id. (No dedicated GET /channels/:id endpoint —
  reuse list.)
- [ ] Commit.

### Task 6 — API keys query + mutation hooks

- [ ] Extend `queries.ts`: `useAPIKeys()`.
- [ ] Extend `mutations.ts`: `useCreateAPIKey` (returns `APIKeyCreated` with
  rawKey for the dialog), `useRevokeAPIKey`.
- [ ] Commit.

### Task 7 — API keys page with reveal-once dialog

- [ ] Create `frontend/components/features/api-keys/api-key-row.tsx` —
  shows name, scopes, last-used, revoke dropdown.
- [ ] Create `frontend/components/features/api-keys/create-api-key-form.tsx`
  — name + scopes checkboxes + expires-in-days Select.
- [ ] Create `frontend/components/features/api-keys/reveal-key-dialog.tsx`
  — shadcn Dialog (not AlertDialog) that shows the raw key + a Copy button
  + a warning banner "This key will not be shown again". Closing navigates
  back to the list view.
- [ ] Create `frontend/app/api-keys/page.tsx` — tabs-free single screen:
  create form (collapsed/expanded), list below, reveal-dialog overlay.
- [ ] Commit.

### Task 8 — E2E tests (optional — small addition)

- [ ] Append `frontend/tests/channels.spec.ts`:
  - Login → navigate to /channels → empty state visible.
  - Create Telegram channel with chat_id → row appears in list.
  - Edit channel name → row updates.
  - Delete channel → empty state returns.
- [ ] Append `frontend/tests/api-keys.spec.ts`:
  - Login → /api-keys → empty.
  - Create key → reveal dialog shows raw key prefixed `pc_live_` + Copy button.
  - Close dialog → list shows the key with only `name` and `created_at`.
  - Revoke → list empty again.
- [ ] Run E2E.
- [ ] Commit.

### Task 9 — Delete migrated Go handlers + templates

- [ ] Remove `pageHandler.ChannelList`, `ChannelNewForm`, `ChannelCreate`,
  `ChannelEditForm`, `ChannelUpdate`, `ChannelDelete`,
  `ChannelConfigFields`, `APIKeyList`, `APIKeyCreate`,
  `APIKeyCreateSubmit`, `APIKeyRevoke` from `pages.go`.
- [ ] Drop the corresponding routes from `setup.go`.
- [ ] Remove from template loader map: `channels.html`, `channel_form.html`,
  `api_keys.html`, `api_key_form.html`.
- [ ] Delete those 4 template files.
- [ ] `buildCheckConfigFromForm` was the last Go-side form-to-JSON helper —
  remove it (only channel handlers used it).
- [ ] `go build / vet / test / golangci-lint run` — all green.
- [ ] Commit.

### Task 10 — Final gate + merge main

- [ ] Docker stack up; E2E run; `docker compose down`.
- [ ] `git checkout main && git merge --ff-only c3-channels-apikeys`.

## Success criteria

1. `/channels`, `/channels/new`, `/channels/[id]/edit`, `/api-keys` — all
   Next.js, return 200 with valid session.
2. Channel CRUD + API key create (with reveal-once) + revoke work end-to-end.
3. 4 Go templates removed; ~11 Go handlers removed.
4. Go side still builds, tests green, zero lint findings.

## Out of scope

- `/settings` profile page (future)
- Channel test-send button (future)
- Monitor-channel binding UI (buried, defer)

## Risks

| Risk | Mitigation |
|---|---|
| API key raw value lost if dialog is dismissed before copy | Big "Copy" button as primary action + warning banner; Copy animates to checkmark on success |
| No GET /channels/:id endpoint → edit page does two round trips | Use list cache: `useChannels()` + `find(id)`; fresh enough |
| Channel types have per-type sensitive fields (Telegram chat_id, SMTP password) | Fields marked `type: password` in ConfigSchema render as `<Input type="password">` |

# D1 — Dark Mode + Layout Plumbing — Design & Plan

**Date:** 2026-04-17
**Parent:** Sub-project D (UX improvements), D1 slice

## Scope

1. Dark mode toggle (system / light / dark) via `next-themes`.
2. Route-group refactor so `/status/[slug]` has no navbar/footer
   (fixes the C4 deviation).
3. Global `error.tsx` + generic `not-found.tsx` for production polish.
4. Navigation loading bar (`nextjs-toploader`).

No new Go work. All changes are frontend.

## Architecture decisions

### 1. `next-themes` with `class` strategy

Tailwind v4 + shadcn expect dark mode via `class="dark"` on `<html>`.
`next-themes` handles system preference + localStorage + FOUC-free
hydration. Adds `<ThemeProvider>` to `providers.tsx`.

Toggle component lives in the Navbar — `<Sun>`/`<Moon>` icon, dropdown
with `light / dark / system` options. shadcn's `mode-toggle` pattern.

### 2. Route-group refactor

Current structure mounts Navbar/Footer in root `layout.tsx`. Problem:
`/status/[slug]` is public, shouldn't show "Dashboard" / "Login" nav.

Move Navbar/Footer to a nested route group layout:

```
app/
  layout.tsx                 # minimal: <html>, <body>, Providers, NextTopLoader
  (main)/                    # route group — no URL segment
    layout.tsx               # Navbar + Footer + <main> wrapper
    page.tsx                 # landing (was app/page.tsx)
    login/page.tsx
    register/page.tsx
    dashboard/page.tsx
    monitors/…
    channels/…
    api-keys/…
  status/[slug]/             # outside the group → root layout only
    page.tsx
    not-found.tsx
  error.tsx                  # global error boundary
  not-found.tsx              # global 404
```

Route groups in Next.js 16 are parentheses; they don't affect URLs.
`app/(main)/login/page.tsx` still serves `/login`.

### 3. Global error / not-found

- `app/error.tsx` — client-component catch-all for uncaught errors. Shows
  a clean apology card + "Try again" (resets the error) + "Go home" link.
- `app/not-found.tsx` — for any unmatched route outside `status/`. Same
  visual language as status's own not-found.
- `app/status/[slug]/not-found.tsx` stays (status-specific copy).

### 4. Navigation loading bar

`nextjs-toploader` is the popular drop-in. Tiny, no config surprises.
Colour matches primary.

Rendered once in root layout above children.

## Implementation plan

### Task 1 — Branch + deps

- [ ] `git checkout -b d1-dark-mode-plumbing`
- [ ] `cd frontend && pnpm add next-themes nextjs-toploader && cd ..`
- [ ] shadcn `add dropdown-menu` already present; ensure `pnpm dlx shadcn@latest add --help` is working (no new component needed since Sun/Moon icons come from lucide-react already installed).
- [ ] Commit deps.

### Task 2 — Route-group refactor

- [ ] Create `frontend/app/(main)/layout.tsx`:

```tsx
import { Navbar } from "@/components/site/navbar";
import { Footer } from "@/components/site/footer";

export default function MainLayout({ children }: { children: React.ReactNode }) {
  return (
    <>
      <Navbar />
      <main className="flex-1">{children}</main>
      <Footer />
    </>
  );
}
```

- [ ] Move pages into the group:
  - `app/page.tsx` → `app/(main)/page.tsx`
  - `app/login/page.tsx` → `app/(main)/login/page.tsx`
  - `app/register/page.tsx` → `app/(main)/register/page.tsx`
  - `app/dashboard/page.tsx` → `app/(main)/dashboard/page.tsx`
  - `app/monitors/*` → `app/(main)/monitors/*` (entire tree)
  - `app/channels/*` → `app/(main)/channels/*`
  - `app/api-keys/page.tsx` → `app/(main)/api-keys/page.tsx`
- [ ] Update root `app/layout.tsx` to drop Navbar/Footer (keep Providers,
  add NextTopLoader):

```tsx
import type { Metadata } from "next";
import "./globals.css";
import { Providers } from "./providers";
import NextTopLoader from "nextjs-toploader";

export const metadata: Metadata = { /* unchanged */ };

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className="h-full antialiased" suppressHydrationWarning>
      <body className="min-h-full flex flex-col bg-background font-sans">
        <NextTopLoader color="hsl(var(--primary))" height={2} showSpinner={false} />
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
```

- [ ] Build check — `cd frontend && pnpm build` — all routes still listed.
- [ ] Commit.

### Task 3 — Dark mode wiring

- [ ] Update `frontend/app/providers.tsx`:

```tsx
"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "next-themes";
import { Toaster } from "@/components/ui/sonner";
import { useState } from "react";

export function Providers({ children }: { children: React.ReactNode }) {
  const [client] = useState(
    () => new QueryClient({
      defaultOptions: { queries: { refetchOnWindowFocus: false, staleTime: 5_000 } },
    }),
  );
  return (
    <ThemeProvider attribute="class" defaultTheme="system" enableSystem>
      <QueryClientProvider client={client}>
        {children}
        <Toaster position="bottom-right" richColors />
      </QueryClientProvider>
    </ThemeProvider>
  );
}
```

- [ ] Add `suppressHydrationWarning` to `<html>` in root layout (already above).
- [ ] Create `frontend/components/site/theme-toggle.tsx`:

```tsx
"use client";

import { Moon, Sun, Laptop } from "lucide-react";
import { useTheme } from "next-themes";
import { buttonVariants } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

export function ThemeToggle() {
  const { setTheme } = useTheme();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        className={buttonVariants({ variant: "ghost", size: "icon-sm" })}
        aria-label="Toggle theme"
      >
        <Sun className="h-4 w-4 rotate-0 scale-100 transition-all dark:-rotate-90 dark:scale-0" />
        <Moon className="absolute h-4 w-4 rotate-90 scale-0 transition-all dark:rotate-0 dark:scale-100" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onClick={() => setTheme("light")}>
          <Sun className="mr-2 h-4 w-4" /> Light
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => setTheme("dark")}>
          <Moon className="mr-2 h-4 w-4" /> Dark
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => setTheme("system")}>
          <Laptop className="mr-2 h-4 w-4" /> System
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
```

- [ ] Add `<ThemeToggle />` to `Navbar` (right side, before auth buttons).
- [ ] Build check; commit.

### Task 4 — Global error + not-found pages

- [ ] Create `frontend/app/error.tsx`:

```tsx
"use client";

import { useEffect } from "react";
import Link from "next/link";
import { AlertTriangle } from "lucide-react";
import { Button, buttonVariants } from "@/components/ui/button";

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    // Log client-side errors; server errors already logged by Go side.
    console.error(error);
  }, [error]);

  return (
    <div className="container mx-auto px-4 py-24 max-w-md text-center">
      <AlertTriangle className="mx-auto h-10 w-10 text-red-500" />
      <h1 className="mt-4 text-2xl font-bold tracking-tight">
        Something went wrong
      </h1>
      <p className="mt-2 text-sm text-muted-foreground">
        An unexpected error occurred. Our team has been notified.
      </p>
      <div className="mt-6 flex items-center justify-center gap-3">
        <Button onClick={reset}>Try again</Button>
        <Link href="/" className={buttonVariants({ variant: "ghost" })}>
          Go home
        </Link>
      </div>
    </div>
  );
}
```

- [ ] Create `frontend/app/not-found.tsx`:

```tsx
import Link from "next/link";
import { Compass } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";

export default function NotFound() {
  return (
    <div className="container mx-auto px-4 py-24 max-w-md text-center">
      <Compass className="mx-auto h-10 w-10 text-muted-foreground/60" />
      <h1 className="mt-4 text-2xl font-bold tracking-tight">Page not found</h1>
      <p className="mt-2 text-sm text-muted-foreground">
        The page you&rsquo;re looking for doesn&rsquo;t exist or has moved.
      </p>
      <Link href="/" className={`${buttonVariants()} mt-6`}>
        Back to home
      </Link>
    </div>
  );
}
```

- [ ] Build check; commit.

### Task 5 — Final gate + merge

- [ ] Docker rebuild web; smoke all routes (including dark-mode toggle
  visible in navbar, status page has no navbar).
- [ ] E2E run — existing 6 tests should still pass.
- [ ] Commit + ff-merge main.

## Success criteria

1. Dark mode toggle in navbar; page honours `system / light / dark`
   preference; no FOUC.
2. `/status/[slug]` has no navbar/footer.
3. Unknown route anywhere → clean 404 page.
4. Server-side error → error boundary with Try Again / Go Home.
5. Navigation between pages shows a slim top-loader bar.
6. Existing 6 E2E pass.
7. Go side untouched — still 0 lint findings.

## Out of scope

- Animations on dark-mode transition (future polish)
- Per-user stored theme preference (localStorage is plenty)
- Theme for status page specifically (uses the same tokens)

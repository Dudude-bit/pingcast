import Link from "next/link";
import { ChevronDown } from "lucide-react";
import { sessionCookie } from "@/lib/session";
import { buttonVariants } from "@/components/ui/button";
import { ThemeToggle } from "./theme-toggle";
import { LogoutButton } from "./logout-button";

// Compare menu is hover/click-opened via <details>. Zero JS, no client
// component needed — keeps the navbar a pure server component. The
// summary is still keyboard-accessible (Space/Enter toggles).
const COMPARE_LINKS = [
  { label: "vs Atlassian Statuspage", href: "/alternatives/atlassian-statuspage" },
  { label: "vs Instatus", href: "/alternatives/instatus" },
  { label: "vs Openstatus", href: "/alternatives/openstatus" },
  { label: "vs UptimeRobot", href: "/alternatives/uptimerobot" },
  { label: "vs Uptime Kuma", href: "/alternatives/uptime-kuma" },
  { label: "Best status pages 2026", href: "/best-status-page-software-2026" },
];

export async function Navbar() {
  const isLoggedIn = Boolean(await sessionCookie());

  return (
    <header className="border-b border-border/40 bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60 sticky top-0 z-50">
      <nav className="container mx-auto flex h-16 items-center justify-between px-4">
        <Link href="/" className="font-bold text-lg tracking-tight">
          PingCast
        </Link>

        <div className="flex items-center gap-2 sm:gap-4">
          <Link
            href="/pricing"
            className="hidden sm:inline text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            Pricing
          </Link>

          <details className="hidden md:block relative group [&[open]>summary_svg]:rotate-180">
            <summary className="flex cursor-pointer items-center gap-1 list-none text-sm text-muted-foreground hover:text-foreground transition-colors">
              Compare
              <ChevronDown className="h-3.5 w-3.5 transition-transform" />
            </summary>
            <div className="absolute right-0 top-full mt-2 w-64 rounded-md border border-border/60 bg-popover shadow-lg p-2">
              {COMPARE_LINKS.map((l) => (
                <Link
                  key={l.href}
                  href={l.href}
                  className="block rounded-md px-3 py-2 text-sm text-muted-foreground hover:text-foreground hover:bg-accent/50 transition-colors"
                >
                  {l.label}
                </Link>
              ))}
            </div>
          </details>

          <Link
            href="/blog"
            className="hidden sm:inline text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            Blog
          </Link>
          <Link
            href="/docs/api"
            className="hidden sm:inline text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            API
          </Link>
          <ThemeToggle />
          {isLoggedIn ? (
            <>
              <Link
                href="/dashboard"
                className="text-sm text-muted-foreground hover:text-foreground transition-colors"
              >
                Dashboard
              </Link>
              <LogoutButton />
            </>
          ) : (
            <>
              <Link
                href="/login"
                className="text-sm text-muted-foreground hover:text-foreground transition-colors"
              >
                Login
              </Link>
              <Link href="/register" className={buttonVariants({ size: "sm" })}>
                Sign up
              </Link>
            </>
          )}
        </div>
      </nav>
    </header>
  );
}

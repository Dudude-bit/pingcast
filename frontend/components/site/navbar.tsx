import Link from "next/link";
import { sessionCookie } from "@/lib/session";
import { buttonVariants } from "@/components/ui/button";
import { ThemeToggle } from "./theme-toggle";
import { LogoutButton } from "./logout-button";

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

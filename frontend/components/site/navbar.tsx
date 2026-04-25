import Link from "next/link";
import { ChevronDown } from "lucide-react";
import { sessionCookie } from "@/lib/session";
import { buttonVariants } from "@/components/ui/button";
import { ThemeToggle } from "./theme-toggle";
import { LogoutButton } from "./logout-button";
import { LanguageSwitcher } from "./language-switcher";
import { getDictionary, hasLocale, type Locale } from "@/lib/i18n";

// Navbar is a server component — receives the locale via params from
// app/[lang]/(main)/layout.tsx. Compare/Solutions menus are <details>
// dropdowns, no client JS needed; the LanguageSwitcher is the only
// client child here.
export async function Navbar({ lang }: { lang: Locale }) {
  const dict = await getDictionary(lang);
  const isLoggedIn = Boolean(await sessionCookie());

  const compareLinks = [
    {
      label: dict.footer.links.vs_atlassian,
      href: `/${lang}/alternatives/atlassian-statuspage`,
    },
    {
      label: dict.footer.links.vs_instatus,
      href: `/${lang}/alternatives/instatus`,
    },
    {
      label: dict.footer.links.vs_openstatus,
      href: `/${lang}/alternatives/openstatus`,
    },
    {
      label: dict.footer.links.vs_uptimerobot,
      href: `/${lang}/alternatives/uptimerobot`,
    },
    {
      label: dict.footer.links.vs_kuma,
      href: `/${lang}/alternatives/uptime-kuma`,
    },
    {
      label: dict.footer.links.best_2026,
      href: `/${lang}/best-status-page-software-2026`,
    },
  ];

  return (
    <header className="border-b border-border/40 bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60 sticky top-0 z-50">
      <nav className="container mx-auto flex h-16 items-center justify-between px-4">
        <Link href={`/${lang}`} className="font-bold text-lg tracking-tight">
          PingCast
        </Link>

        <div className="flex items-center gap-2 sm:gap-4">
          <Link
            href={`/${lang}/pricing`}
            className="hidden sm:inline text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            {dict.nav.pricing}
          </Link>

          <details className="hidden md:block relative group [&[open]>summary_svg]:rotate-180">
            <summary className="flex cursor-pointer items-center gap-1 list-none text-sm text-muted-foreground hover:text-foreground transition-colors">
              {dict.nav.compare}
              <ChevronDown className="h-3.5 w-3.5 transition-transform" />
            </summary>
            <div className="absolute right-0 top-full mt-2 w-64 rounded-md border border-border/60 bg-popover shadow-lg p-2">
              {compareLinks.map((l) => (
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
            href={`/${lang}/blog`}
            className="hidden sm:inline text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            {dict.nav.blog}
          </Link>
          <Link
            href={`/${lang}/docs/api`}
            className="hidden sm:inline text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            {dict.nav.docs}
          </Link>
          <LanguageSwitcher current={lang} />
          <ThemeToggle />
          {isLoggedIn ? (
            <>
              <Link
                href={`/${lang}/dashboard`}
                className="text-sm text-muted-foreground hover:text-foreground transition-colors"
              >
                {dict.nav.dashboard}
              </Link>
              <LogoutButton />
            </>
          ) : (
            <>
              <Link
                href={`/${lang}/login`}
                className="text-sm text-muted-foreground hover:text-foreground transition-colors"
              >
                {dict.nav.login}
              </Link>
              <Link
                href={`/${lang}/register`}
                className={buttonVariants({ size: "sm" })}
              >
                {dict.nav.register}
              </Link>
            </>
          )}
        </div>
      </nav>
    </header>
  );
}

// hasLocale re-exported for callers that derive Navbar from a string;
// kept here so navbar.tsx is the one stop the layout imports.
export { hasLocale };

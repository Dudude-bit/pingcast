"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Globe } from "lucide-react";
import { SUPPORTED_LOCALES, type Locale } from "@/lib/i18n-shared";

// LanguageSwitcher swaps the leading /<lang> segment in the current
// pathname for the chosen locale. Uses <details> so it stays
// no-script-friendly and zero-state — re-renders happen client-side
// because Link prefetches the alt-locale route.
export function LanguageSwitcher({ current }: { current: Locale }) {
  const pathname = usePathname();
  const stripped = stripLocale(pathname);

  return (
    <details className="relative [&[open]>summary]:text-foreground">
      <summary className="flex cursor-pointer items-center gap-1 list-none text-sm text-muted-foreground hover:text-foreground transition-colors px-1.5 py-1 rounded-md">
        <Globe className="h-3.5 w-3.5" />
        <span className="uppercase">{current}</span>
      </summary>
      <div className="absolute right-0 top-full mt-2 w-32 rounded-md border border-border/60 bg-popover shadow-lg p-1">
        {SUPPORTED_LOCALES.map((l) => (
          <Link
            key={l}
            href={`/${l}${stripped}`}
            className={
              "block rounded-md px-2 py-1.5 text-sm transition-colors " +
              (l === current
                ? "bg-accent/40 text-foreground"
                : "text-muted-foreground hover:text-foreground hover:bg-accent/50")
            }
          >
            {LABELS[l]}
          </Link>
        ))}
      </div>
    </details>
  );
}

const LABELS: Record<Locale, string> = {
  en: "English",
  ru: "Русский",
};

function stripLocale(pathname: string): string {
  for (const l of SUPPORTED_LOCALES) {
    if (pathname === `/${l}`) return "";
    if (pathname.startsWith(`/${l}/`)) return pathname.slice(`/${l}`.length);
  }
  return pathname;
}

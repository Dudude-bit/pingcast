"use client";

import { useRef } from "react";
import { usePathname, useRouter } from "next/navigation";
import { Globe } from "lucide-react";
import { SUPPORTED_LOCALES, type Locale } from "@/lib/i18n-shared";

// LanguageSwitcher swaps the leading /<lang> segment in the current
// pathname for the chosen locale.
//
// Why router.replace + manual close instead of <Link>:
// previously the dropdown stayed open during the locale-segment
// navigation, and because [lang] re-renders the entire layout tree
// (different dictionary → different navbar → different children) the
// open <details> visibly jumped/flashed before disappearing. Replacing
// the URL via router (no history entry — the user wasn't really
// navigating, just translating) and closing the <details> first lets
// the new locale paint without the dropdown artefact.
export function LanguageSwitcher({ current }: { current: Locale }) {
  const pathname = usePathname();
  const router = useRouter();
  const detailsRef = useRef<HTMLDetailsElement>(null);
  const stripped = stripLocale(pathname);

  const switchTo = (l: Locale) => (e: React.MouseEvent) => {
    e.preventDefault();
    if (l === current) return;
    if (detailsRef.current) detailsRef.current.open = false;
    router.replace(`/${l}${stripped}`);
  };

  return (
    <details
      ref={detailsRef}
      className="relative [&[open]>summary]:text-foreground"
    >
      <summary className="flex cursor-pointer items-center gap-1 list-none text-sm text-muted-foreground hover:text-foreground transition-colors px-1.5 py-1 rounded-md">
        <Globe className="h-3.5 w-3.5" />
        <span className="uppercase">{current}</span>
      </summary>
      <div className="absolute right-0 top-full mt-2 w-32 rounded-md border border-border/60 bg-popover shadow-lg p-1">
        {SUPPORTED_LOCALES.map((l) => (
          <a
            key={l}
            href={`/${l}${stripped}`}
            onClick={switchTo(l)}
            className={
              "block rounded-md px-2 py-1.5 text-sm transition-colors " +
              (l === current
                ? "bg-accent/40 text-foreground"
                : "text-muted-foreground hover:text-foreground hover:bg-accent/50")
            }
          >
            {LABELS[l]}
          </a>
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

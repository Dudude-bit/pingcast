import { headers } from "next/headers";
import { SUPPORTED_LOCALES, DEFAULT_LOCALE, type Locale } from "./i18n-shared";

// Server-only locale picker for routes that aren't under app/[lang]/
// (status page, well-known endpoints, fallback layouts, not-found,
// error). Picks in priority order:
//
//   1. /<lang>/ prefix in the URL — proxy.ts injects x-pathname for
//      this. Matters when the visitor's Accept-Language disagrees
//      with the URL they typed (RU URL but EN browser → RU wins).
//   2. Accept-Language header.
//   3. DEFAULT_LOCALE.
export async function pickLocaleFromHeaders(): Promise<Locale> {
  const h = await headers();

  // 1. URL-prefix wins.
  const path = h.get("x-pathname") ?? "";
  for (const l of SUPPORTED_LOCALES) {
    if (path === `/${l}` || path.startsWith(`/${l}/`)) {
      return l;
    }
  }

  // 2. Accept-Language fallback.
  const accept = h.get("accept-language") ?? "";
  for (const part of accept.split(",")) {
    const tag = part.trim().split(";")[0]!.toLowerCase();
    const primary = tag.split("-")[0]!;
    if ((SUPPORTED_LOCALES as readonly string[]).includes(primary)) {
      return primary as Locale;
    }
  }
  return DEFAULT_LOCALE;
}

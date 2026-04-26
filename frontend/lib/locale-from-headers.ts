import { headers } from "next/headers";
import { SUPPORTED_LOCALES, DEFAULT_LOCALE, type Locale } from "./i18n-shared";

// pickLocale is the pure resolver used by both the header-reading
// wrapper below and unit tests. Priority order:
//
//   1. /<lang>/ prefix in the URL. Matters when the visitor's
//      Accept-Language disagrees with the URL they typed
//      (RU URL but EN browser → RU wins).
//   2. Accept-Language header.
//   3. DEFAULT_LOCALE.
export function pickLocale(pathname: string, acceptLanguage: string): Locale {
  // 1. URL-prefix wins.
  for (const l of SUPPORTED_LOCALES) {
    if (pathname === `/${l}` || pathname.startsWith(`/${l}/`)) {
      return l;
    }
  }

  // 2. Accept-Language fallback.
  for (const part of acceptLanguage.split(",")) {
    const tag = part.trim().split(";")[0]!.toLowerCase();
    const primary = tag.split("-")[0]!;
    if ((SUPPORTED_LOCALES as readonly string[]).includes(primary)) {
      return primary as Locale;
    }
  }
  return DEFAULT_LOCALE;
}

// Server-only locale picker for routes that aren't under app/[lang]/
// (status page, well-known endpoints, fallback layouts, not-found,
// error). Thin wrapper over pickLocale for easy mocking in tests.
export async function pickLocaleFromHeaders(): Promise<Locale> {
  const h = await headers();
  return pickLocale(h.get("x-pathname") ?? "", h.get("accept-language") ?? "");
}

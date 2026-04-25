import { headers } from "next/headers";
import { SUPPORTED_LOCALES, DEFAULT_LOCALE, type Locale } from "./i18n-shared";

// Server-only locale picker for routes that aren't under app/[lang]/
// (status page, well-known endpoints, fallback layouts). Mirrors the
// edge proxy logic so the same Accept-Language → locale rules apply
// everywhere.
//
// Falls back to DEFAULT_LOCALE if no supported language is offered.
export async function pickLocaleFromHeaders(): Promise<Locale> {
  const h = await headers();
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

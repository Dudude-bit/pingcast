import "server-only";
import {
  type Locale,
  hasLocale,
  SUPPORTED_LOCALES,
  DEFAULT_LOCALE,
} from "./i18n-shared";

// Re-export shared constants so existing imports of "@/lib/i18n" don't
// break in server components. The actual JSON loaders live below — they
// can't be reached from the client because of the `server-only` import.
export { hasLocale, SUPPORTED_LOCALES, DEFAULT_LOCALE };
export type { Locale };

const dictionaries = {
  en: () => import("../dictionaries/en.json").then((m) => m.default),
  ru: () => import("../dictionaries/ru.json").then((m) => m.default),
} as const;

// Dictionary type derived from the EN file so a missing RU key is a
// type error, not a runtime crash.
export type Dictionary = Awaited<ReturnType<(typeof dictionaries)["en"]>>;

export async function getDictionary(locale: Locale): Promise<Dictionary> {
  return dictionaries[locale]();
}

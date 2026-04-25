// Locale constants + type. Importable from both server and client
// components (no JSON imports here, so the bundler can tree-shake).
// Dictionary loading lives in lib/i18n.ts (server-only).

export const SUPPORTED_LOCALES = ["en", "ru"] as const;
export type Locale = (typeof SUPPORTED_LOCALES)[number];
export const DEFAULT_LOCALE: Locale = "en";

export function hasLocale(value: string): value is Locale {
  return (SUPPORTED_LOCALES as readonly string[]).includes(value);
}

"use client";

import { createContext, useContext } from "react";
import type { Locale } from "@/lib/i18n-shared";

// Dictionary is the static EN-shape. Imported as a type-only reference
// so this client component never pulls the server-only dictionary
// loader into the browser bundle.
type Dictionary = typeof import("@/dictionaries/en.json");

// LocaleProvider lets client components reach into the dictionary +
// know which locale they're rendering. Server components prefer
// passing dict directly via props — context is for client-only
// surfaces (forms, buttons, headers with state).

type LocaleContextValue = {
  locale: Locale;
  dict: Dictionary;
};

const LocaleContext = createContext<LocaleContextValue | null>(null);

export function LocaleProvider({
  locale,
  dict,
  children,
}: {
  locale: Locale;
  dict: Dictionary;
  children: React.ReactNode;
}) {
  return (
    <LocaleContext.Provider value={{ locale, dict }}>
      {children}
    </LocaleContext.Provider>
  );
}

export function useLocale(): LocaleContextValue {
  const ctx = useContext(LocaleContext);
  if (!ctx) {
    throw new Error("useLocale must be used within a LocaleProvider");
  }
  return ctx;
}

// localeHref prefixes a path with the current locale. Use for client
// links inside [lang]; server components should compose `/${locale}${path}`
// inline since they already have the param.
export function useLocaleHref() {
  const { locale } = useLocale();
  return (path: string) => {
    if (path.startsWith("http://") || path.startsWith("https://")) return path;
    if (path.startsWith("#")) return path;
    if (!path.startsWith("/")) return `/${locale}/${path}`;
    return `/${locale}${path}`;
  };
}

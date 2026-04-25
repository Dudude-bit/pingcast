import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { getDictionary, hasLocale, SUPPORTED_LOCALES } from "@/lib/i18n";
import { LocaleProvider } from "@/components/i18n/locale-provider";

// Per-locale layout. Owns the lang attribute hint via per-page metadata
// (the actual <html lang> stays on the root layout — Next 16 disallows
// nested <html>). Each route receives `params: { lang }` and looks up
// its dictionary; we surface the dict to client components via
// LocaleProvider context.

export async function generateStaticParams() {
  return SUPPORTED_LOCALES.map((lang) => ({ lang }));
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ lang: string }>;
}): Promise<Metadata> {
  const { lang } = await params;
  if (!hasLocale(lang)) return {};
  const dict = await getDictionary(lang);
  return {
    title: {
      default: dict.meta.default_title,
      template: `%s · PingCast`,
    },
    description: dict.meta.default_description,
    alternates: {
      canonical: `/${lang}`,
      languages: {
        en: "/en",
        ru: "/ru",
        "x-default": "/en",
      },
    },
    openGraph: {
      title: dict.meta.default_title,
      description: dict.meta.default_description,
      type: "website",
      locale: lang === "ru" ? "ru_RU" : "en_US",
    },
  };
}

export default async function LocaleLayout({
  children,
  params,
}: {
  children: React.ReactNode;
  params: Promise<{ lang: string }>;
}) {
  const { lang } = await params;
  if (!hasLocale(lang)) notFound();
  const dict = await getDictionary(lang);
  return (
    <LocaleProvider locale={lang} dict={dict}>
      {children}
    </LocaleProvider>
  );
}

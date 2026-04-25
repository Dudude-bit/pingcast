import type { Metadata } from "next";
import "@fontsource-variable/inter";
import "@fontsource-variable/jetbrains-mono";
import "./globals.css";
import { Providers } from "./providers";
import NextTopLoader from "nextjs-toploader";
import { PlausibleScript } from "@/components/analytics/plausible";
import { OrganizationJsonLd } from "@/components/seo/jsonld";

// Root layout — lightweight wrapper that survives below the locale
// segment. The actual lang attribute is owned by app/[lang]/layout.tsx
// because that's where we know the locale; here we ship a sane "en"
// default so non-localized routes (the public status page at
// /status/<slug>, sitemap, robots) still render with valid markup.
//
// Self-hosted fonts via @fontsource-variable so builds don't call out
// to fonts.googleapis.com (Dokploy build env can't reach it). The CSS
// imports above add @font-face rules; globals.css references them via
// --font-inter / --font-jetbrains.

export const metadata: Metadata = {
  metadataBase: new URL(
    process.env.NEXT_PUBLIC_SITE_URL ?? "http://localhost:3000",
  ),
  twitter: { card: "summary_large_image" },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="h-full antialiased" suppressHydrationWarning>
      <body className="min-h-full flex flex-col bg-background font-sans">
        <NextTopLoader
          color="hsl(221 83% 53%)"
          height={2}
          showSpinner={false}
        />
        <Providers>{children}</Providers>
        <OrganizationJsonLd />
        <PlausibleScript />
      </body>
    </html>
  );
}

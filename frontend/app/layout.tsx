import type { Metadata } from "next";
import "@fontsource-variable/inter";
import "@fontsource-variable/jetbrains-mono";
import "./globals.css";
import { Providers } from "./providers";
import NextTopLoader from "nextjs-toploader";
import { PlausibleScript } from "@/components/analytics/plausible";
import { OrganizationJsonLd } from "@/components/seo/jsonld";

// Self-hosted via @fontsource-variable so builds don't call out to
// fonts.googleapis.com (Dokploy build env can't reach it). The CSS
// imports above add @font-face rules for 'Inter Variable' and
// 'JetBrains Mono Variable'; globals.css references them through
// --font-inter / --font-jetbrains vars.

export const metadata: Metadata = {
  title: {
    default: "PingCast — uptime monitoring that doesn't suck",
    template: "%s · PingCast",
  },
  description:
    "Lightweight uptime monitoring with instant Telegram alerts and public status pages. Built for developers who ship fast.",
  metadataBase: new URL(
    process.env.NEXT_PUBLIC_SITE_URL ?? "http://localhost:3000",
  ),
  openGraph: {
    title: "PingCast — uptime monitoring that doesn't suck",
    description:
      "Lightweight uptime monitoring with instant Telegram alerts and public status pages.",
    type: "website",
  },
  twitter: { card: "summary_large_image" },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang="en"
      className="h-full antialiased"
      suppressHydrationWarning
    >
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

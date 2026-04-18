import type { Metadata } from "next";
import { Inter, JetBrains_Mono } from "next/font/google";
import "./globals.css";
import { Providers } from "./providers";
import NextTopLoader from "nextjs-toploader";

// `display: "swap"` shows the system fallback until the webfont lands,
// which keeps LCP fast and prevents FOIT. Variables are consumed via
// `--font-sans` / `--font-mono` in globals.css.
const fontSans = Inter({
  subsets: ["latin"],
  variable: "--font-inter",
  display: "swap",
});

const fontMono = JetBrains_Mono({
  subsets: ["latin"],
  variable: "--font-jetbrains",
  display: "swap",
});

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
      className={`h-full antialiased ${fontSans.variable} ${fontMono.variable}`}
      suppressHydrationWarning
    >
      <body className="min-h-full flex flex-col bg-background font-sans">
        <NextTopLoader
          color="hsl(221 83% 53%)"
          height={2}
          showSpinner={false}
        />
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}

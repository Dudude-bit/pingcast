import { NextRequest, NextResponse } from "next/server";
import { SUPPORTED_LOCALES, DEFAULT_LOCALE } from "@/lib/i18n-shared";

/**
 * Next 16 edge middleware (file is `proxy.ts` in Next 16, was
 * `middleware.ts` in 14/15). Three responsibilities:
 *
 *   1. Locale routing — strip-or-redirect URLs to ensure every page
 *      lives under /<lang>/. Visitors landing on / get redirected to
 *      /en or /ru based on Accept-Language; bots are sent to /en for
 *      a stable canonical.
 *   2. Fast-path auth gate — redirect unauthenticated users off
 *      protected routes before they hit the page handler. The Go API
 *      still re-checks server-side; this only saves a round-trip when
 *      the cookie is absent.
 *   3. Custom-domain host routing — if the incoming Host header is a
 *      registered Pro-tier custom domain (e.g. status.customer.com),
 *      rewrite the request to /status/<slug> so the public status
 *      page renders.
 */

// Paths that bypass locale routing — backend, static assets, the
// public status page (its slug already encodes the tenant), and the
// .well-known validation probe.
const LOCALE_BYPASS_PREFIXES = [
  "/api/",
  "/_next/",
  "/.well-known/",
  "/widget.js",
  "/status/",
  "/sitemap.xml",
  "/robots.txt",
  "/favicon.ico",
  "/favicon.svg",
  "/favicon.png",
  "/opengraph-image",
  "/twitter-image",
];

function pickLocale(req: NextRequest): string {
  const accept = req.headers.get("accept-language") ?? "";
  // Cheap header parse: "ru,en;q=0.9" → ["ru", "en"]. Good enough; we
  // only need to detect "ru" vs everything else.
  for (const part of accept.split(",")) {
    const tag = part.trim().split(";")[0]!.toLowerCase();
    const primary = tag.split("-")[0]!;
    if ((SUPPORTED_LOCALES as readonly string[]).includes(primary)) {
      return primary;
    }
  }
  return DEFAULT_LOCALE;
}

// Hosts that are ours. Never do a custom-domain lookup for these, even
// if a malicious client spoofs the Host header.
const CANONICAL_HOSTS = new Set([
  "pingcast.io",
  "www.pingcast.io",
  "status.pingcast.io",
  "pingcast.kirillin.tech",
  "localhost",
  "localhost:3000",
  "localhost:3001",
]);

// Bot / infra paths that should skip custom-domain routing even on
// foreign hosts. /.well-known/pingcast/<token> is the validation probe;
// widget.js / badge.svg serve from our apex and shouldn't be rewritten.
const BYPASS_PREFIXES = [
  "/.well-known/pingcast/",
  "/widget.js",
  "/api/",
  "/_next/",
];

export async function proxy(req: NextRequest) {
  const { pathname } = req.nextUrl;
  const host = req.headers.get("host") ?? "";

  // --- Custom-domain routing ---
  // Cheap bail-outs first so canonical-host requests don't pay anything.
  if (
    host &&
    !CANONICAL_HOSTS.has(host) &&
    !host.endsWith(".pingcast.io") &&
    !host.endsWith(".vercel.app") &&
    !BYPASS_PREFIXES.some((p) => pathname.startsWith(p))
  ) {
    const slug = await lookupCustomDomain(host);
    if (slug) {
      const url = req.nextUrl.clone();
      url.pathname = `/status/${slug}`;
      return NextResponse.rewrite(url);
    }
    // Unknown host + no slug mapping — let the request fall through.
    // A 404 from the real route beats a misleading redirect.
  }

  // --- Locale routing ---
  // If the URL doesn't already start with a supported locale and isn't
  // bypassed, redirect to the user's preferred locale. The locale
  // prefix is mandatory under the new app/[lang] tree.
  const isBypassed = LOCALE_BYPASS_PREFIXES.some((p) => pathname.startsWith(p));
  if (!isBypassed) {
    const hasLocale = SUPPORTED_LOCALES.some(
      (l) => pathname === `/${l}` || pathname.startsWith(`/${l}/`),
    );
    if (!hasLocale) {
      const target = `/${pickLocale(req)}${pathname === "/" ? "" : pathname}`;
      const url = req.nextUrl.clone();
      url.pathname = target;
      return NextResponse.redirect(url);
    }
  }

  // Pass the resolved pathname downstream as a request header so
  // not-found.tsx and other Next special files (which don't receive
  // route params) can read the URL via headers() and pick the right
  // locale from a /<lang>/ prefix.
  const downstreamHeaders = new Headers(req.headers);
  downstreamHeaders.set("x-pathname", pathname);

  // --- Auth gate ---
  // Strip the leading /<lang> for the protected-prefix check so adding
  // a locale to the URL doesn't unexpectedly bypass auth.
  let bareForAuth = pathname;
  for (const l of SUPPORTED_LOCALES) {
    if (pathname === `/${l}` || pathname.startsWith(`/${l}/`)) {
      bareForAuth = pathname.slice(`/${l}`.length) || "/";
      break;
    }
  }
  const isProtected = [
    "/dashboard",
    "/monitors",
    "/channels",
    "/api-keys",
    "/settings",
  ].some(
    (p) => bareForAuth === p || bareForAuth.startsWith(`${p}/`),
  );

  if (!isProtected) {
    return NextResponse.next({ request: { headers: downstreamHeaders } });
  }

  const hasSession = req.cookies.has("session_id");
  if (hasSession) {
    return NextResponse.next({ request: { headers: downstreamHeaders } });
  }

  // Redirect to /<lang>/login preserving locale.
  const lang =
    SUPPORTED_LOCALES.find(
      (l) => pathname === `/${l}` || pathname.startsWith(`/${l}/`),
    ) ?? DEFAULT_LOCALE;
  const url = req.nextUrl.clone();
  url.pathname = `/${lang}/login`;
  return NextResponse.redirect(url);
}

// lookupCustomDomain calls the Go API's public lookup endpoint. The
// API response is Cache-Control public max-age=300, so the platform
// edge cache + upstream HTTP cache keep the DB roundtrip count low.
// Per-request-instance cache would help further but middleware state
// doesn't survive between invocations on most edge runtimes.
async function lookupCustomDomain(host: string): Promise<string | null> {
  try {
    const base =
      process.env.INTERNAL_API_URL ?? "http://api:8080/api";
    const url = `${base}/public/lookup-domain?hostname=${encodeURIComponent(host)}`;
    const res = await fetch(url, {
      // Short timeout — don't let a slow API stall every incoming
      // request on a non-matching host.
      signal: AbortSignal.timeout(2_000),
    });
    if (!res.ok) return null;
    const body = (await res.json()) as { slug?: string };
    return body.slug ?? null;
  } catch {
    return null;
  }
}

export const config = {
  matcher: [
    // Exclude Next.js statics, favicons. /api/* is still caught by the
    // proxy function but short-circuits inside via BYPASS_PREFIXES.
    "/((?!_next/static|_next/image|favicon\\.svg|favicon\\.png).*)",
  ],
};

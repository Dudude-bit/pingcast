import type { NextConfig } from "next";

// Baseline security headers applied to every route. CSP is intentionally
// omitted here because Recharts, Framer Motion, and our JSON-LD payload
// all emit inline styles/scripts; a correct CSP needs a nonce pipeline
// that the ingress (Traefik / Dokploy) is better positioned to inject.
// The headers below are pure response policies, so they can live at the
// Next layer without coordination.
const securityHeaders = [
  // Enforce HTTPS once the browser has seen the site over TLS.
  // includeSubDomains is safe: status pages share the apex domain.
  {
    key: "Strict-Transport-Security",
    value: "max-age=31536000; includeSubDomains",
  },
  // Prevent MIME sniffing on served assets (e.g. user-supplied JSON
  // being interpreted as script).
  { key: "X-Content-Type-Options", value: "nosniff" },
  // Don't leak full URLs (which carry slugs / query params) to
  // third-party sites the user navigates to.
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  // App has no use for these capabilities; denying them cuts off a
  // class of malicious-iframe attack vectors.
  {
    key: "Permissions-Policy",
    value: "camera=(), microphone=(), geolocation=(), interest-cohort=()",
  },
  // Status pages are embeddable by design if users choose to, but the
  // authenticated app should never be framed. Simplest way to handle
  // both: only the app routes are guarded via X-Frame-Options, below.
  { key: "X-Frame-Options", value: "SAMEORIGIN" },
];

const nextConfig: NextConfig = {
  output: "standalone",
  async headers() {
    return [{ source: "/:path*", headers: securityHeaders }];
  },
  async rewrites() {
    // Rewrites execute server-side inside the web container, so they
    // must target the internal Docker DNS name — NEXT_PUBLIC_API_URL
    // points at the browser-reachable host URL (localhost:8080), which
    // is unroutable from inside the container.
    const dest =
      process.env.INTERNAL_API_URL ??
      process.env.NEXT_PUBLIC_API_URL ??
      "http://localhost:8080/api";
    return [
      {
        source: "/api/:path*",
        destination: `${dest}/:path*`,
      },
    ];
  },
};

export default nextConfig;

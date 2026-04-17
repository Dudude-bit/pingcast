import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  async rewrites() {
    // In dev (pnpm dev), proxy /api/* to Go so the browser sees a
    // single origin. In prod (Docker), the ingress (Dokploy/Traefik)
    // routes at domain level; this rewrite is harmless because
    // INTERNAL_API_URL is the server-side source of truth.
    return [
      {
        source: "/api/:path*",
        destination: `${process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/api"}/:path*`,
      },
    ];
  },
};

export default nextConfig;

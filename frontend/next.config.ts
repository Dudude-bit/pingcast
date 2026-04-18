import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
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

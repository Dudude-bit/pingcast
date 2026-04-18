import type { MetadataRoute } from "next";

/**
 * Sitemap for the public, crawlable surface. Authenticated routes live
 * behind a redirect to /login and are disallowed via robots.ts, so they
 * are intentionally omitted here. Public /status/[slug] pages are owned
 * by end-users and are not enumerable from the frontend, so those get
 * indexed by direct link, not by sitemap traversal.
 */
export default function sitemap(): MetadataRoute.Sitemap {
  const base = process.env.NEXT_PUBLIC_SITE_URL ?? "http://localhost:3000";
  const now = new Date();
  return [
    {
      url: `${base}/`,
      lastModified: now,
      changeFrequency: "weekly",
      priority: 1.0,
    },
    {
      url: `${base}/login`,
      lastModified: now,
      changeFrequency: "monthly",
      priority: 0.3,
    },
    {
      url: `${base}/register`,
      lastModified: now,
      changeFrequency: "monthly",
      priority: 0.7,
    },
  ];
}

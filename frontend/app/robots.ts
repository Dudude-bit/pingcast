import type { MetadataRoute } from "next";

/**
 * Exposes /robots.txt. Disallows the authenticated app surface so search
 * engines don't waste crawl budget on redirect-to-/login responses, and
 * points crawlers at the sitemap for the public pages they should index.
 */
export default function robots(): MetadataRoute.Robots {
  const base = process.env.NEXT_PUBLIC_SITE_URL ?? "http://localhost:3000";
  return {
    rules: [
      {
        userAgent: "*",
        allow: "/",
        disallow: [
          "/dashboard",
          "/monitors",
          "/channels",
          "/api-keys",
          "/api/",
        ],
      },
    ],
    sitemap: `${base}/sitemap.xml`,
  };
}

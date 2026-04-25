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
          // Authenticated app surface — locale-prefixed and not. Both
          // need to be listed because Google considers /en/dashboard and
          // /dashboard as separate URLs even though our proxy redirects
          // one to the other.
          "/dashboard",
          "/monitors",
          "/channels",
          "/api-keys",
          "/en/dashboard",
          "/en/monitors",
          "/en/channels",
          "/en/api-keys",
          "/ru/dashboard",
          "/ru/monitors",
          "/ru/channels",
          "/ru/api-keys",
          "/api/",
        ],
      },
    ],
    sitemap: `${base}/sitemap.xml`,
  };
}

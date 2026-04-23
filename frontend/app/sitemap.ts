import type { MetadataRoute } from "next";
import { listAlternativeSlugs } from "@/content/alternatives";
import { POSTS } from "@/content/blog";

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

  // Core product + auth pages.
  const core: MetadataRoute.Sitemap = [
    { url: `${base}/`,          lastModified: now, changeFrequency: "weekly",  priority: 1.0 },
    { url: `${base}/pricing`,   lastModified: now, changeFrequency: "monthly", priority: 0.9 },
    { url: `${base}/register`,  lastModified: now, changeFrequency: "monthly", priority: 0.7 },
    { url: `${base}/login`,     lastModified: now, changeFrequency: "monthly", priority: 0.3 },
    { url: `${base}/docs/api`,  lastModified: now, changeFrequency: "weekly",  priority: 0.6 },
    { url: `${base}/blog`,      lastModified: now, changeFrequency: "weekly",  priority: 0.7 },
  ];

  // Category / SEO landing pages (sprint 4).
  const seo: MetadataRoute.Sitemap = [
    "status-page-software",
    "best-status-page-software-2026",
    "how-to-create-status-page",
    "atlassian-statuspage-pricing",
    "open-source-status-page",
    "saas-status-page",
    "status-page-template",
  ].map((slug) => ({
    url: `${base}/${slug}`,
    lastModified: now,
    changeFrequency: "monthly",
    priority: 0.8,
  }));

  // Alternatives (one page per competitor).
  const alternatives: MetadataRoute.Sitemap = listAlternativeSlugs().map((slug) => ({
    url: `${base}/alternatives/${slug}`,
    lastModified: now,
    changeFrequency: "monthly",
    priority: 0.8,
  }));

  // Blog posts.
  const blog: MetadataRoute.Sitemap = POSTS.map((p) => ({
    url: `${base}/blog/${p.slug}`,
    lastModified: new Date(p.publishedAt),
    changeFrequency: "monthly",
    priority: 0.6,
  }));

  return [...core, ...seo, ...alternatives, ...blog];
}

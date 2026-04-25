import type { MetadataRoute } from "next";
import { listAlternativeSlugs } from "@/content/alternatives";
import { POSTS } from "@/content/blog";
import { SUPPORTED_LOCALES } from "@/lib/i18n-shared";

/**
 * Sitemap for the public, crawlable surface. Every page is emitted
 * once per supported locale. Authenticated routes live behind a
 * redirect to /login and are disallowed via robots.ts, so they are
 * intentionally omitted here. Public /status/[slug] pages are owned
 * by end-users and are not enumerable from the frontend, so those get
 * indexed by direct link, not by sitemap traversal.
 *
 * `alternates.languages` on each entry tells Google which other
 * locale URLs to consider equivalent — drives hreflang in SERP.
 */
export default function sitemap(): MetadataRoute.Sitemap {
  const base = process.env.NEXT_PUBLIC_SITE_URL ?? "http://localhost:3000";
  const now = new Date();

  type LocalePath = { path: string; freq: MetadataRoute.Sitemap[number]["changeFrequency"]; priority: number; lastMod?: Date };

  const corePaths: LocalePath[] = [
    { path: "", freq: "weekly", priority: 1.0 },
    { path: "/pricing", freq: "monthly", priority: 0.9 },
    { path: "/register", freq: "monthly", priority: 0.7 },
    { path: "/login", freq: "monthly", priority: 0.3 },
    { path: "/docs/api", freq: "weekly", priority: 0.6 },
    { path: "/blog", freq: "weekly", priority: 0.7 },
    { path: "/status-page-software", freq: "monthly", priority: 0.8 },
    { path: "/best-status-page-software-2026", freq: "monthly", priority: 0.8 },
    { path: "/how-to-create-status-page", freq: "monthly", priority: 0.8 },
    { path: "/atlassian-statuspage-pricing", freq: "monthly", priority: 0.8 },
    { path: "/open-source-status-page", freq: "monthly", priority: 0.8 },
    { path: "/saas-status-page", freq: "monthly", priority: 0.8 },
    { path: "/status-page-template", freq: "monthly", priority: 0.8 },
  ];

  const altPaths: LocalePath[] = listAlternativeSlugs().map((slug) => ({
    path: `/alternatives/${slug}`,
    freq: "monthly",
    priority: 0.8,
  }));

  const blogPaths: LocalePath[] = POSTS.map((p) => ({
    path: `/blog/${p.slug}`,
    freq: "monthly",
    priority: 0.6,
    lastMod: new Date(p.publishedAt),
  }));

  const all = [...corePaths, ...altPaths, ...blogPaths];

  return all.flatMap((entry) =>
    SUPPORTED_LOCALES.map((lang) => ({
      url: `${base}/${lang}${entry.path}`,
      lastModified: entry.lastMod ?? now,
      changeFrequency: entry.freq,
      priority: entry.priority,
      alternates: {
        languages: Object.fromEntries(
          SUPPORTED_LOCALES.map((l) => [l, `${base}/${l}${entry.path}`]),
        ),
      },
    })),
  );
}

// Blog content registry. Adding a post = one .mdx file under
// content/blog/, one entry here (metadata), and one entry in
// app/(main)/blog/[slug]/page.tsx (import + POST_BODIES map).

export type BlogPost = {
  slug: string;
  title: string;
  description: string;
  publishedAt: string; // YYYY-MM-DD
  author: string;
  readingMinutes: number;
};

// Ordered newest-first so the /blog index lists freshest at the top.
export const POSTS: BlogPost[] = [
  {
    slug: "status-pages-reduce-support-tickets",
    title: "How a public status page cuts support tickets (and when it doesn't)",
    description:
      "The math on \"is your service down?\" tickets, what patterns actually reduce them, and when a status page is premature. 6-min read for SaaS founders debating whether to ship one.",
    publishedAt: "2026-04-24",
    author: "Kirill",
    readingMinutes: 6,
  },
  {
    slug: "migrating-from-atlassian-statuspage",
    title: "Migrating from Atlassian Statuspage in under 60 seconds",
    description:
      "What the Statuspage JSON export actually contains, what our 1-click importer does with it, and what doesn't transfer (subscribers, audiences, SLA reports) and why.",
    publishedAt: "2026-04-23",
    author: "Kirill",
    readingMinutes: 5,
  },
  {
    slug: "pivoting-from-uptime-monitoring-to-status-pages",
    title: "Why we pivoted from \"uptime monitoring\" to \"branded status pages for SaaS\"",
    description:
      "A month ago PingCast sold as uptime monitoring. Today it sells as the budget-friendly alternative to Atlassian Statuspage. Here's what changed and why.",
    publishedAt: "2026-04-22",
    author: "Kirill",
    readingMinutes: 6,
  },
];

export function getPostBySlug(slug: string): BlogPost | undefined {
  return POSTS.find((p) => p.slug === slug);
}

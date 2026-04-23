// Blog content registry. Adding a post = one entry here + one file
// with the body. Kept as TSX not MDX for now — MDX is a worthwhile
// upgrade once we pass 10 posts.

export type BlogPost = {
  slug: string;
  title: string;
  description: string;
  publishedAt: string; // YYYY-MM-DD
  author: string;
  readingMinutes: number;
};

export const POSTS: BlogPost[] = [
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

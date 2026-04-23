import type { Metadata } from "next";
import Link from "next/link";
import { POSTS } from "@/content/blog";
import { BreadcrumbListJsonLd } from "@/components/seo/jsonld";

export const metadata: Metadata = {
  title: "Blog",
  description:
    "Notes on building PingCast — product pivots, technical deep-dives on uptime monitoring + status pages, and the occasional indie-SaaS launch retro.",
  alternates: { canonical: "/blog" },
};

export default function BlogIndexPage() {
  return (
    <div className="container mx-auto px-4 py-12 max-w-3xl">
      <BreadcrumbListJsonLd
        items={[
          { name: "Home", url: "/" },
          { name: "Blog", url: "/blog" },
        ]}
      />
      <h1 className="text-4xl md:text-5xl font-bold tracking-tight">Blog</h1>
      <p className="mt-4 text-lg text-muted-foreground leading-relaxed">
        Notes on building PingCast. Shipped in public, so the pivots, the
        pricing experiments, and the occasional 3-AM incident are all here.
      </p>

      <ul className="mt-12 space-y-8">
        {POSTS.map((p) => (
          <li key={p.slug} className="border-b border-border/40 pb-8 last:border-b-0">
            <Link href={`/blog/${p.slug}`} className="block group">
              <time className="text-xs uppercase tracking-wider text-muted-foreground">
                {p.publishedAt} · {p.readingMinutes} min read · {p.author}
              </time>
              <h2 className="mt-2 text-2xl font-semibold tracking-tight group-hover:text-primary transition-colors">
                {p.title}
              </h2>
              <p className="mt-2 text-muted-foreground leading-relaxed">
                {p.description}
              </p>
              <span className="mt-3 inline-block text-sm text-primary underline underline-offset-4">
                Read →
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}

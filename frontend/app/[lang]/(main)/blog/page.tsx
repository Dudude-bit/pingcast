import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { postsForLocale } from "@/content/blog";
import { BreadcrumbListJsonLd } from "@/components/seo/jsonld";
import { NewsletterForm } from "@/components/features/common/newsletter-form";
import { getDictionary, hasLocale } from "@/lib/i18n";

type Params = Promise<{ lang: string }>;

export async function generateMetadata({
  params,
}: {
  params: Params;
}): Promise<Metadata> {
  const { lang } = await params;
  if (!hasLocale(lang)) return {};
  const dict = await getDictionary(lang);
  return {
    title: dict.blog.title,
    description: dict.blog.subtitle,
    alternates: {
      canonical: `/${lang}/blog`,
      languages: { en: "/en/blog", ru: "/ru/blog", "x-default": "/en/blog" },
    },
  };
}

export default async function BlogIndexPage({ params }: { params: Params }) {
  const { lang } = await params;
  if (!hasLocale(lang)) notFound();
  const dict = await getDictionary(lang);
  const posts = postsForLocale(lang);
  const b = dict.blog;

  return (
    <div className="container mx-auto px-4 py-12 max-w-3xl">
      <BreadcrumbListJsonLd
        items={[
          { name: dict.alternatives_template.home, url: `/${lang}` },
          { name: b.title, url: `/${lang}/blog` },
        ]}
      />
      <h1 className="text-4xl md:text-5xl font-bold tracking-tight">{b.title}</h1>
      <p className="mt-4 text-lg text-muted-foreground leading-relaxed">
        {b.subtitle}
      </p>

      <div className="mt-8 rounded-lg border border-border/60 bg-card p-5">
        <h2 className="text-sm font-semibold mb-1">{b.subscribe_heading}</h2>
        <p className="text-sm text-muted-foreground mb-3">{b.subscribe_sub}</p>
        <div className="max-w-md">
          <NewsletterForm source="blog_index" />
        </div>
      </div>

      {posts.length === 0 ? (
        <p className="mt-12 text-muted-foreground">{b.empty}</p>
      ) : (
        <ul className="mt-12 space-y-8">
          {posts.map((p) => (
            <li
              key={p.slug}
              className="border-b border-border/40 pb-8 last:border-b-0"
            >
              <Link href={`/${lang}/blog/${p.slug}`} className="block group">
                <time className="text-xs uppercase tracking-wider text-muted-foreground">
                  {p.publishedAt} · {p.readingMinutes} {b.min_read} · {p.author}
                </time>
                <h2 className="mt-2 text-2xl font-semibold tracking-tight group-hover:text-primary transition-colors">
                  {p.title[lang] ?? p.title.en}
                </h2>
                <p className="mt-2 text-muted-foreground leading-relaxed">
                  {p.description[lang] ?? p.description.en}
                </p>
                <span className="mt-3 inline-block text-sm text-primary underline underline-offset-4">
                  {b.read_more}
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

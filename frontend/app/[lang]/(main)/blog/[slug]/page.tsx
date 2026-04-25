import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ArrowLeft } from "lucide-react";
import { POSTS, getPostBySlug } from "@/content/blog";
import { BreadcrumbListJsonLd } from "@/components/seo/jsonld";
import { getDictionary, hasLocale, SUPPORTED_LOCALES, type Locale } from "@/lib/i18n";
import PivotEN from "@/content/blog/pivoting-from-uptime-monitoring-to-status-pages.mdx";
import PivotRU from "@/content/blog/pivoting-from-uptime-monitoring-to-status-pages.ru.mdx";
import MigratingEN from "@/content/blog/migrating-from-atlassian-statuspage.mdx";
import MigratingRU from "@/content/blog/migrating-from-atlassian-statuspage.ru.mdx";
import SupportEN from "@/content/blog/status-pages-reduce-support-tickets.mdx";
import SupportRU from "@/content/blog/status-pages-reduce-support-tickets.ru.mdx";

// Per-locale MDX bodies. Adding a new post = one .mdx file per locale
// under content/blog/, one entry in content/blog/index.ts (metadata),
// one entry in this map per locale. Static imports keep the bundler
// honest; dynamic template-literal imports don't work well with
// @next/mdx.
const POST_BODIES: Record<string, Record<Locale, React.ComponentType>> = {
  "pivoting-from-uptime-monitoring-to-status-pages": { en: PivotEN, ru: PivotRU },
  "migrating-from-atlassian-statuspage": { en: MigratingEN, ru: MigratingRU },
  "status-pages-reduce-support-tickets": { en: SupportEN, ru: SupportRU },
};

type Params = Promise<{ lang: string; slug: string }>;

export function generateStaticParams() {
  return SUPPORTED_LOCALES.flatMap((lang) =>
    POSTS.map((p) => ({ lang, slug: p.slug })),
  );
}

export async function generateMetadata({
  params,
}: {
  params: Params;
}): Promise<Metadata> {
  const { lang, slug } = await params;
  if (!hasLocale(lang)) return {};
  const post = getPostBySlug(slug);
  if (!post) return {};
  const title = post.title[lang] ?? post.title.en;
  const description = post.description[lang] ?? post.description.en;
  return {
    title,
    description,
    alternates: {
      canonical: `/${lang}/blog/${post.slug}`,
      languages: Object.fromEntries(
        SUPPORTED_LOCALES.map((l) => [l, `/${l}/blog/${post.slug}`]),
      ),
    },
    openGraph: {
      title,
      description,
      type: "article",
      publishedTime: post.publishedAt,
      authors: [post.author],
      locale: lang === "ru" ? "ru_RU" : "en_US",
    },
  };
}

export default async function BlogPostPage({ params }: { params: Params }) {
  const { lang, slug } = await params;
  if (!hasLocale(lang)) notFound();
  const dict = await getDictionary(lang);
  const post = getPostBySlug(slug);
  const bodies = POST_BODIES[slug];
  if (!post || !bodies) notFound();
  // Pick the locale-specific body when available; fall back to EN with
  // a translation-in-progress banner if not.
  const Body = bodies[lang] ?? bodies.en;
  const title = post.title[lang] ?? post.title.en;
  const description = post.description[lang] ?? post.description.en;
  const englishOnlyNote = !post.locales.includes(lang) ? (
    <p className="mb-8 rounded-md border border-amber-500/30 bg-amber-500/10 px-4 py-3 text-sm text-amber-700 dark:text-amber-400">
      {dict.blog.english_only_note}
    </p>
  ) : null;

  return (
    <article className="container mx-auto px-4 py-12 max-w-2xl">
      <BreadcrumbListJsonLd
        items={[
          { name: dict.alternatives_template.home, url: `/${lang}` },
          { name: dict.blog.title, url: `/${lang}/blog` },
          { name: title, url: `/${lang}/blog/${post.slug}` },
        ]}
      />
      <Link
        href={`/${lang}/blog`}
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mb-8"
      >
        <ArrowLeft className="h-3.5 w-3.5" /> {dict.blog.all_posts}
      </Link>
      {englishOnlyNote}
      <header className="mb-10">
        <time className="text-xs uppercase tracking-wider text-muted-foreground">
          {post.publishedAt} · {post.readingMinutes} {dict.blog.min_read} ·{" "}
          {post.author}
        </time>
        <h1 className="mt-2 text-4xl md:text-5xl font-bold tracking-tight leading-tight">
          {title}
        </h1>
        <p className="mt-4 text-lg text-muted-foreground leading-relaxed">
          {description}
        </p>
      </header>
      <div className="prose-content space-y-5 text-foreground leading-relaxed [&_h2]:text-2xl [&_h2]:font-bold [&_h2]:tracking-tight [&_h2]:mt-10 [&_h2]:mb-3 [&_h3]:text-xl [&_h3]:font-semibold [&_h3]:mt-6 [&_h3]:mb-2 [&_p]:text-muted-foreground [&_ul]:list-disc [&_ul]:pl-6 [&_ul]:space-y-2 [&_ul]:text-muted-foreground [&_code]:bg-muted [&_code]:px-1.5 [&_code]:py-0.5 [&_code]:rounded [&_code]:text-xs [&_strong]:text-foreground [&_a]:underline [&_a]:underline-offset-4 [&_a]:text-foreground hover:[&_a]:text-primary">
        <Body />
      </div>
    </article>
  );
}

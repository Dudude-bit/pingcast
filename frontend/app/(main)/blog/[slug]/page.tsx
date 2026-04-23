import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ArrowLeft } from "lucide-react";
import { POSTS, getPostBySlug } from "@/content/blog";
import { BreadcrumbListJsonLd } from "@/components/seo/jsonld";
import PivotPost from "@/content/blog/pivoting-from-uptime-monitoring-to-status-pages.mdx";

// Map slug → MDX component. Adding a new post = one .mdx file under
// content/blog/, one entry in content/blog/index.ts (metadata), one
// entry here. Static imports keep the bundler honest; dynamic template-
// literal imports don't work well with @next/mdx.
const POST_BODIES: Record<string, React.ComponentType> = {
  "pivoting-from-uptime-monitoring-to-status-pages": PivotPost,
};

export function generateStaticParams() {
  return POSTS.map((p) => ({ slug: p.slug }));
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ slug: string }>;
}): Promise<Metadata> {
  const { slug } = await params;
  const post = getPostBySlug(slug);
  if (!post) return {};
  return {
    title: post.title,
    description: post.description,
    alternates: { canonical: `/blog/${post.slug}` },
    openGraph: {
      title: post.title,
      description: post.description,
      type: "article",
      publishedTime: post.publishedAt,
      authors: [post.author],
    },
  };
}

export default async function BlogPostPage({
  params,
}: {
  params: Promise<{ slug: string }>;
}) {
  const { slug } = await params;
  const post = getPostBySlug(slug);
  const Body = POST_BODIES[slug];
  if (!post || !Body) notFound();

  return (
    <article className="container mx-auto px-4 py-12 max-w-2xl">
      <BreadcrumbListJsonLd
        items={[
          { name: "Home", url: "/" },
          { name: "Blog", url: "/blog" },
          { name: post.title, url: `/blog/${post.slug}` },
        ]}
      />
      <Link
        href="/blog"
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mb-8"
      >
        <ArrowLeft className="h-3.5 w-3.5" /> All posts
      </Link>
      <header className="mb-10">
        <time className="text-xs uppercase tracking-wider text-muted-foreground">
          {post.publishedAt} · {post.readingMinutes} min read · {post.author}
        </time>
        <h1 className="mt-2 text-4xl md:text-5xl font-bold tracking-tight leading-tight">
          {post.title}
        </h1>
        <p className="mt-4 text-lg text-muted-foreground leading-relaxed">
          {post.description}
        </p>
      </header>
      <div className="prose-content space-y-5 text-foreground leading-relaxed [&_h2]:text-2xl [&_h2]:font-bold [&_h2]:tracking-tight [&_h2]:mt-10 [&_h2]:mb-3 [&_h3]:text-xl [&_h3]:font-semibold [&_h3]:mt-6 [&_h3]:mb-2 [&_p]:text-muted-foreground [&_ul]:list-disc [&_ul]:pl-6 [&_ul]:space-y-2 [&_ul]:text-muted-foreground [&_code]:bg-muted [&_code]:px-1.5 [&_code]:py-0.5 [&_code]:rounded [&_code]:text-xs [&_strong]:text-foreground [&_a]:underline [&_a]:underline-offset-4 [&_a]:text-foreground hover:[&_a]:text-primary">
        <Body />
      </div>
    </article>
  );
}

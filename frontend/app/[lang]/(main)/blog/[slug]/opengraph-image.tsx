import { ImageResponse } from "next/og";
import { getPostBySlug, POSTS } from "@/content/blog";
import { OGShell, OG_SIZE, OG_CONTENT_TYPE } from "@/components/og/shell";
import { hasLocale, type Locale } from "@/lib/i18n-shared";

export const size = OG_SIZE;
export const contentType = OG_CONTENT_TYPE;

export function generateImageMetadata() {
  return POSTS.map((p) => ({
    id: p.slug,
    alt: p.title.en,
    contentType,
    size,
  }));
}

type Params = Promise<{ lang: string; slug: string }>;

// Per-post share card. Each published post gets its own 1200×630 PNG
// per locale rendered at build/edge, so Twitter and Slack unfurls show
// the post title in the visitor's language.
export default async function Image({ params }: { params: Params }) {
  const { lang, slug } = await params;
  const locale: Locale = hasLocale(lang) ? lang : "en";
  const post = getPostBySlug(slug);
  if (!post) {
    return new ImageResponse(
      (
        <OGShell
          headline={locale === "ru" ? "Пост не найден" : "Post not found"}
        />
      ),
      { ...size },
    );
  }
  const kicker =
    locale === "ru"
      ? `Блог · ${post.readingMinutes} мин чтения`
      : `Blog · ${post.readingMinutes} min read`;
  return new ImageResponse(
    (
      <OGShell
        kicker={kicker}
        headline={post.title[locale] ?? post.title.en}
        sub={post.description[locale] ?? post.description.en}
      />
    ),
    { ...size },
  );
}

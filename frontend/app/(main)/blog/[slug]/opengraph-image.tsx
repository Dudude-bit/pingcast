import { ImageResponse } from "next/og";
import { getPostBySlug, POSTS } from "@/content/blog";
import { OGShell, OG_SIZE, OG_CONTENT_TYPE } from "@/components/og/shell";

export const size = OG_SIZE;
export const contentType = OG_CONTENT_TYPE;

export function generateImageMetadata() {
  return POSTS.map((p) => ({
    id: p.slug,
    alt: p.title,
    contentType,
    size,
  }));
}

type Params = Promise<{ slug: string }>;

// Per-post share card. Each published post gets its own 1200×630 PNG
// rendered at build/edge, so Twitter and Slack unfurls show the post
// title instead of the root card.
export default async function Image({ params }: { params: Params }) {
  const { slug } = await params;
  const post = getPostBySlug(slug);
  if (!post) {
    return new ImageResponse(
      (<OGShell headline="Post not found" />),
      { ...size },
    );
  }
  return new ImageResponse(
    (
      <OGShell
        kicker={`Blog · ${post.readingMinutes} min read`}
        headline={post.title}
        sub={post.description}
      />
    ),
    { ...size },
  );
}

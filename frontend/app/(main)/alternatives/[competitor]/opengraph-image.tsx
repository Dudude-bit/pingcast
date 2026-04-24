import { ImageResponse } from "next/og";
import { ALTERNATIVES } from "@/content/alternatives";
import { OGShell, OG_SIZE, OG_CONTENT_TYPE } from "@/components/og/shell";

export const size = OG_SIZE;
export const contentType = OG_CONTENT_TYPE;

export function generateImageMetadata() {
  return Object.values(ALTERNATIVES).map((alt) => ({
    id: alt.slug,
    alt: alt.metaTitle,
    contentType,
    size,
  }));
}

type Params = Promise<{ competitor: string }>;

// Per-competitor share card for /alternatives/<slug>. Ensures Twitter
// and Slack unfurls mention the specific competitor instead of the
// generic root card.
export default async function Image({ params }: { params: Params }) {
  const { competitor } = await params;
  const entry = ALTERNATIVES[competitor];
  if (!entry) {
    return new ImageResponse(
      (<OGShell headline="Alternative not found" />),
      { ...size },
    );
  }
  return new ImageResponse(
    (
      <OGShell
        kicker="Compare · Status page software"
        headline={`${entry.name} alternative`}
        sub={entry.tagline}
      />
    ),
    { ...size },
  );
}

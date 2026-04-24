import { ImageResponse } from "next/og";
import { OGShell, OG_SIZE, OG_CONTENT_TYPE } from "@/components/og/shell";

export const size = OG_SIZE;
export const contentType = OG_CONTENT_TYPE;
export const alt = "Best status page software in 2026 — honest comparison";

export default async function Image() {
  return new ImageResponse(
    (
      <OGShell
        kicker="2026 buyer's guide"
        headline="Best status page software"
        sub="An honest comparison across price, hosting, migration ease, and what you actually give up on each tier."
      />
    ),
    { ...size },
  );
}

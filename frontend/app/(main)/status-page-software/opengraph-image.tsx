import { ImageResponse } from "next/og";
import { OGShell, OG_SIZE, OG_CONTENT_TYPE } from "@/components/og/shell";

export const size = OG_SIZE;
export const contentType = OG_CONTENT_TYPE;
export const alt = "Status page software — a founder's guide";

export default async function Image() {
  return new ImageResponse(
    (
      <OGShell
        kicker="Guide"
        headline="Status page software — what actually matters"
        sub="Patterns that reduce support tickets, vendor tradeoffs, and the 10-minute setup path."
      />
    ),
    { ...size },
  );
}

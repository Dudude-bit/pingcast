import { ImageResponse } from "next/og";
import { OGShell, OG_SIZE, OG_CONTENT_TYPE } from "@/components/og/shell";

export const size = OG_SIZE;
export const contentType = OG_CONTENT_TYPE;
export const alt = "How to create a branded status page in 10 minutes";

export default async function Image() {
  return new ImageResponse(
    (
      <OGShell
        kicker="Tutorial"
        headline="Create a branded status page in 10 minutes"
        sub="Domain, monitors, brand colours, incident templates — the order that actually gets you live on the same afternoon."
      />
    ),
    { ...size },
  );
}

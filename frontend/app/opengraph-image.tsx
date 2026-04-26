import { ImageResponse } from "next/og";

export const alt =
  "PingCast — branded status pages for SaaS, at a third of Atlassian's price";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

// Generated at build time for the root route — shows whenever
// pingcast.io (or any non-status-page subpath) is shared.
export default async function Image() {
  return new ImageResponse(
    (
      <div
        style={{
          height: "100%",
          width: "100%",
          display: "flex",
          flexDirection: "column",
          background:
            "linear-gradient(135deg, #0b1020 0%, #111936 55%, #0b1020 100%)",
          color: "#f8fafc",
          padding: "80px",
          fontFamily: "system-ui, sans-serif",
          position: "relative",
        }}
      >
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: "16px",
            fontSize: "28px",
            fontWeight: 600,
            letterSpacing: "-0.01em",
            color: "#cbd5e1",
          }}
        >
          <div
            style={{
              width: "14px",
              height: "14px",
              borderRadius: "999px",
              background: "#10b981",
              boxShadow: "0 0 24px #10b981",
            }}
          />
          PingCast
        </div>

        <div
          style={{
            display: "flex",
            flexDirection: "column",
            marginTop: "auto",
            gap: "24px",
          }}
        >
          <div
            style={{
              fontSize: "84px",
              fontWeight: 700,
              lineHeight: 1.05,
              letterSpacing: "-0.025em",
              background:
                "linear-gradient(180deg, #ffffff 0%, #94a3b8 110%)",
              backgroundClip: "text",
              color: "transparent",
              display: "flex",
              flexDirection: "column",
            }}
          >
            <span>Branded status pages for SaaS,</span>
            <span style={{ color: "#60a5fa" }}>
              at a third of Atlassian&apos;s price.
            </span>
          </div>

          <div
            style={{
              fontSize: "28px",
              color: "#94a3b8",
              maxWidth: "900px",
              lineHeight: 1.4,
            }}
          >
            Custom domain · Incident timeline · Email subscribers · From $9/mo
          </div>
        </div>
      </div>
    ),
    { ...size },
  );
}

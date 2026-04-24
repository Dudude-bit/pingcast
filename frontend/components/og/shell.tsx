import type { CSSProperties, ReactNode } from "react";

// OGShell is the shared visual template for every opengraph-image.tsx
// route in the app. Keeping it in one place means the brand mark, gradient,
// and typography stay consistent when future routes add their own OG.
//
// Usage from a route's opengraph-image.tsx:
//   return new ImageResponse(
//     (<OGShell kicker="Blog post" headline={title} sub={description} />),
//     { ...size },
//   );

export const OG_SIZE = { width: 1200, height: 630 } as const;
export const OG_CONTENT_TYPE = "image/png";

type Props = {
  kicker?: string;
  headline: string;
  sub?: string;
  accent?: string;
  children?: ReactNode;
};

export function OGShell({
  kicker,
  headline,
  sub,
  accent = "#60a5fa",
  children,
}: Props) {
  return (
    <div style={shellStyle}>
      <div style={brandRow}>
        <div style={brandDot} />
        PingCast
      </div>

      <div style={stackStyle}>
        {kicker ? (
          <div
            style={{
              fontSize: "22px",
              fontWeight: 500,
              textTransform: "uppercase",
              letterSpacing: "0.16em",
              color: "#94a3b8",
              display: "flex",
            }}
          >
            {kicker}
          </div>
        ) : null}
        <div style={{ ...headlineStyle, color: accent }}>
          <span
            style={{
              background: "linear-gradient(180deg, #ffffff 0%, #94a3b8 110%)",
              backgroundClip: "text",
              color: "transparent",
              display: "flex",
            }}
          >
            {headline}
          </span>
        </div>
        {sub ? (
          <div
            style={{
              fontSize: "26px",
              color: "#94a3b8",
              maxWidth: "960px",
              lineHeight: 1.4,
              display: "flex",
            }}
          >
            {sub}
          </div>
        ) : null}
        {children}
      </div>
    </div>
  );
}

const shellStyle: CSSProperties = {
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
};

const brandRow: CSSProperties = {
  display: "flex",
  alignItems: "center",
  gap: "16px",
  fontSize: "28px",
  fontWeight: 600,
  letterSpacing: "-0.01em",
  color: "#cbd5e1",
};

const brandDot: CSSProperties = {
  width: "14px",
  height: "14px",
  borderRadius: "999px",
  background: "#10b981",
  boxShadow: "0 0 24px #10b981",
};

const stackStyle: CSSProperties = {
  display: "flex",
  flexDirection: "column",
  marginTop: "auto",
  gap: "24px",
};

const headlineStyle: CSSProperties = {
  fontSize: "76px",
  fontWeight: 700,
  lineHeight: 1.05,
  letterSpacing: "-0.025em",
  display: "flex",
  flexDirection: "column",
};

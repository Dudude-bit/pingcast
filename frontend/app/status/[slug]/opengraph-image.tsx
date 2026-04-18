import { ImageResponse } from "next/og";
import { apiFetch, ApiError } from "@/lib/api";
import type { components } from "@/lib/openapi-types";

type StatusPage = components["schemas"]["StatusPageResponse"];

export const alt = "Public uptime status page";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

// Re-fetches on each share — status is meaningful in real-time, so a
// cached build-time image would mislead. Falls back to a neutral card
// if the API is unreachable, because a broken OG is worse than a plain
// one when the whole purpose is making the link shareable.
async function safeFetch(slug: string): Promise<StatusPage | null> {
  try {
    return await apiFetch<StatusPage>(`/status/${slug}`);
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) return null;
    return null;
  }
}

export default async function Image({
  params,
}: {
  params: { slug: string };
}) {
  const data = await safeFetch(params.slug);
  const allUp = data?.all_up ?? true;
  const slug = data?.slug ?? params.slug;
  const headline = allUp ? "All Systems Operational" : "Service Disruption";
  const accent = allUp ? "#10b981" : "#ef4444";
  const subtitle = allUp
    ? "Everything is running smoothly."
    : "One or more services are reporting issues.";

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
        }}
      >
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: "12px",
            fontSize: "26px",
            color: "#cbd5e1",
            fontWeight: 600,
          }}
        >
          <div
            style={{
              width: "12px",
              height: "12px",
              borderRadius: "999px",
              background: "#10b981",
              boxShadow: "0 0 20px #10b981",
            }}
          />
          PingCast
        </div>

        <div
          style={{
            display: "flex",
            flexDirection: "column",
            marginTop: "auto",
            gap: "20px",
          }}
        >
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: "20px",
              fontSize: "36px",
              color: "#94a3b8",
              fontFamily: "ui-monospace, SFMono-Regular, monospace",
            }}
          >
            /status/{slug}
          </div>

          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: "32px",
              fontSize: "72px",
              fontWeight: 700,
              letterSpacing: "-0.02em",
              color: accent,
              lineHeight: 1.05,
            }}
          >
            <div
              style={{
                width: "28px",
                height: "28px",
                borderRadius: "999px",
                background: accent,
                boxShadow: `0 0 60px ${accent}`,
              }}
            />
            {headline}
          </div>

          <div style={{ fontSize: "28px", color: "#94a3b8" }}>{subtitle}</div>
        </div>
      </div>
    ),
    { ...size },
  );
}

import Script from "next/script";

// PlausibleScript mounts the analytics pixel. No-ops in dev and whenever
// NEXT_PUBLIC_PLAUSIBLE_DOMAIN is unset — keeps the landing page free
// of a broken tag when the env isn't configured yet (pre-launch).
export function PlausibleScript() {
  if (process.env.NODE_ENV !== "production") return null;
  const domain = process.env.NEXT_PUBLIC_PLAUSIBLE_DOMAIN;
  if (!domain) return null;
  const src =
    process.env.NEXT_PUBLIC_PLAUSIBLE_SRC ??
    "https://plausible.io/js/script.js";
  return (
    <Script defer data-domain={domain} src={src} strategy="afterInteractive" />
  );
}

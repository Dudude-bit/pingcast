// Cookie-based pricing-page A/B bucketer. Sprint 5 runbook calls for
// three variants:
//   A — $9 founder / $19 retail (current default)
//   B — $19 retail only (no founder)
//   C — $9 + 14-day trial → $19
// We need stable assignment so a returning visitor sees the same price,
// no infra beyond a 60-day cookie. Variants are env-driven so the
// operator can soft-launch (e.g. NEXT_PUBLIC_AB_VARIANTS=A,B) without
// shipping code.

export type Bucket = "A" | "B" | "C";

const COOKIE_NAME = "pc_ab_pricing";
const MAX_AGE_DAYS = 60;

function enabledVariants(): Bucket[] {
  const raw = (process.env.NEXT_PUBLIC_AB_VARIANTS ?? "A").trim();
  const parts = raw
    .split(",")
    .map((s) => s.trim().toUpperCase())
    .filter((s): s is Bucket => s === "A" || s === "B" || s === "C");
  // Always fall back to A so a misconfigured env doesn't blank the
  // pricing page out.
  return parts.length > 0 ? parts : ["A"];
}

function readCookie(name: string): string | null {
  if (typeof document === "undefined") return null;
  const cookies = document.cookie.split("; ");
  for (const c of cookies) {
    const idx = c.indexOf("=");
    if (idx < 0) continue;
    if (c.slice(0, idx) === name) return decodeURIComponent(c.slice(idx + 1));
  }
  return null;
}

function writeCookie(name: string, value: string) {
  if (typeof document === "undefined") return;
  const maxAge = MAX_AGE_DAYS * 24 * 60 * 60;
  document.cookie = `${name}=${encodeURIComponent(value)}; Max-Age=${maxAge}; Path=/; SameSite=Lax`;
}

// getBucket returns the visitor's stable bucket. Side-effect: assigns
// and persists a bucket on first call. SSR-safe — returns the first
// enabled variant when document is unavailable, so server-rendered HTML
// matches the most common client outcome and doesn't flash on hydrate.
export function getBucket(): Bucket {
  const variants = enabledVariants();
  if (typeof document === "undefined") return variants[0]!;
  const existing = readCookie(COOKIE_NAME);
  if (existing && variants.includes(existing as Bucket)) {
    return existing as Bucket;
  }
  const pick = variants[Math.floor(Math.random() * variants.length)]!;
  writeCookie(COOKIE_NAME, pick);
  return pick;
}

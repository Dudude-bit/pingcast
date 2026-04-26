// Tiny helper over Plausible's window.plausible function so callers
// don't have to null-check or type-cast at each site. A no-op when the
// pixel isn't loaded (dev, envs unset, ad-blocker) — safe to sprinkle
// freely across conversion-critical click paths.
//
// Every event automatically picks up the visitor's pricing-A/B bucket
// so funnel splits in Plausible work without per-call wiring.

import { getBucket } from "./abtest";

type PlausibleProps = Record<string, string | number | boolean>;

type Plausible = (
  event: string,
  options?: { props?: PlausibleProps },
) => void;

declare global {
  interface Window {
    plausible?: Plausible;
  }
}

export function track(event: string, props?: PlausibleProps) {
  if (typeof window === "undefined") return;
  const fn = window.plausible;
  if (!fn) return;
  const merged: PlausibleProps = { bucket: getBucket(), ...(props ?? {}) };
  fn(event, { props: merged });
}

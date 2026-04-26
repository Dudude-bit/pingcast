"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { buttonVariants } from "@/components/ui/button";
import { track } from "@/lib/analytics";
import type { components } from "@/lib/openapi-types";
import { useLocale } from "@/components/i18n/locale-provider";
import { getBucket, type Bucket } from "@/lib/abtest";

type FounderStatus = components["schemas"]["FounderStatus"];

const FOUNDER_URL = process.env.NEXT_PUBLIC_LEMONSQUEEZY_FOUNDER_URL;
const RETAIL_URL = process.env.NEXT_PUBLIC_LEMONSQUEEZY_RETAIL_URL;

// UpgradeButton renders the Pro-tier CTA. The pricing-A/B bucket
// decides which price/link the visitor sees:
//   A — $9 founder if cap not reached, else $19 retail
//   B — $19 retail only (no founder option)
//   C — same as A but with a "14-day free trial" note (the trial
//       itself is configured in LemonSqueezy on the product/variant)
// Bucket assignment runs once on mount (cookie-stable) so price never
// flickers between renders. Pre-mount we render nothing — same gate
// as the existing founder-status fetch — so server HTML doesn't lie
// about the price.
export function UpgradeButton({
  className,
  size = "lg",
}: {
  className?: string;
  size?: "sm" | "default" | "lg";
}) {
  const { dict, locale } = useLocale();
  const [status, setStatus] = useState<FounderStatus | null>(null);
  const [bucket, setBucket] = useState<Bucket | null>(null);

  useEffect(() => {
    setBucket(getBucket());
    let cancelled = false;
    fetch("/api/billing/founder-status", { credentials: "include" })
      .then((r) => (r.ok ? r.json() : null))
      .then((body: FounderStatus | null) => {
        if (!cancelled && body) setStatus(body);
      })
      .catch(() => {
        // Silent: button just stays hidden if the call fails.
      });
    return () => {
      cancelled = true;
    };
  }, []);

  if (!status || !bucket) return null;

  // Variant B forces retail pricing regardless of founder cap.
  const useFounder = bucket !== "B" && status.available;
  const url = useFounder ? FOUNDER_URL : RETAIL_URL;
  const label = dict.pricing.pro_cta;
  const price = useFounder ? dict.pricing.pro_price_founder : dict.pricing.pro_price_retail;
  const per = dict.pricing.pro_per;
  const placeholder =
    locale === "ru" ? "Pro скоро" : "Pro coming soon";

  if (!url) {
    return (
      <span
        className={`${buttonVariants({ size, variant: "outline" })} ${className ?? ""} opacity-60 cursor-not-allowed`}
        aria-disabled="true"
      >
        {placeholder}
      </span>
    );
  }

  const variantLabel = bucket === "B" ? "retail" : useFounder ? "founder" : "retail";
  const showTrialNote = bucket === "C";

  return (
    <span className="inline-flex flex-col items-center gap-1">
      <Link
        href={url}
        target="_blank"
        rel="noopener noreferrer"
        onClick={() =>
          track("pro_checkout_clicked", {
            variant: variantLabel,
            lang: locale,
          })
        }
        className={`${buttonVariants({ size })} ${className ?? ""}`}
      >
        {label} · {price}{per}
      </Link>
      {showTrialNote ? (
        <span className="text-xs text-muted-foreground">{dict.pricing.pro_trial_note}</span>
      ) : null}
    </span>
  );
}

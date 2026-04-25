"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { buttonVariants } from "@/components/ui/button";
import { track } from "@/lib/analytics";
import type { components } from "@/lib/openapi-types";
import { useLocale } from "@/components/i18n/locale-provider";

type FounderStatus = components["schemas"]["FounderStatus"];

const FOUNDER_URL = process.env.NEXT_PUBLIC_LEMONSQUEEZY_FOUNDER_URL;
const RETAIL_URL = process.env.NEXT_PUBLIC_LEMONSQUEEZY_RETAIL_URL;

// UpgradeButton renders the Pro-tier CTA, falling through three
// states:
//   1. Unknown founder status → nothing (avoid flashing wrong price)
//   2. Founder available → $9/mo link with "founder's price"
//   3. Founder sold out → $19/mo link
// Pro users see nothing — the caller checks plan upstream.
//
// All checkout-click events carry the current locale so we can split
// conversion by language in Plausible.
export function UpgradeButton({
  className,
  size = "lg",
}: {
  className?: string;
  size?: "sm" | "default" | "lg";
}) {
  const { dict, locale } = useLocale();
  const [status, setStatus] = useState<FounderStatus | null>(null);

  useEffect(() => {
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

  if (!status) return null;

  const founder = status.available;
  const url = founder ? FOUNDER_URL : RETAIL_URL;
  const label = dict.pricing.pro_cta;
  const price = founder ? dict.pricing.pro_price_founder : dict.pricing.pro_price_retail;
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

  return (
    <Link
      href={url}
      target="_blank"
      rel="noopener noreferrer"
      onClick={() =>
        track("pro_checkout_clicked", {
          variant: founder ? "founder" : "retail",
          lang: locale,
        })
      }
      className={`${buttonVariants({ size })} ${className ?? ""}`}
    >
      {label} · {price}{per}
    </Link>
  );
}

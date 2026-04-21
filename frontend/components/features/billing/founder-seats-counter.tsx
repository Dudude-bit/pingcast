"use client";

import { useEffect, useState } from "react";
import type { components } from "@/lib/openapi-types";

type FounderStatus = components["schemas"]["FounderStatus"];

// FounderSeatsCounter renders a small pill above the Pro card showing
// how many of the $9 founder seats are left. Hidden once the cap is
// reached (the pricing copy implicitly shifts to $19 retail at that
// point, handled by UpgradeButton).
export function FounderSeatsCounter() {
  const [status, setStatus] = useState<FounderStatus | null>(null);

  useEffect(() => {
    let cancelled = false;
    fetch("/api/billing/founder-status")
      .then((r) => (r.ok ? r.json() : null))
      .then((body: FounderStatus | null) => {
        if (!cancelled && body) setStatus(body);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, []);

  if (!status || !status.available) return null;

  const left = status.cap - Number(status.used);
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full border border-primary/40 bg-primary/10 px-3 py-1 text-[11px] font-medium text-primary shadow-sm">
      <span className="inline-block h-1.5 w-1.5 rounded-full bg-primary animate-pulse" />
      {left} of {status.cap} founder seats left
    </span>
  );
}

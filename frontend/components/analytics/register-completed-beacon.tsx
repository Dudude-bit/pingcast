"use client";

import { useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { track } from "@/lib/analytics";

// RegisterCompletedBeacon fires the `register_completed` Plausible
// event exactly once when the dashboard loads with ?registered=1,
// then strips the query param so a hard refresh doesn't double-count.
export function RegisterCompletedBeacon() {
  const searchParams = useSearchParams();
  const router = useRouter();

  useEffect(() => {
    if (searchParams.get("registered") === "1") {
      track("register_completed");
      // Replace URL to drop the query param. Stays on the same route;
      // no re-fetch because we don't use router.refresh().
      router.replace("/dashboard");
    }
  }, [searchParams, router]);

  return null;
}

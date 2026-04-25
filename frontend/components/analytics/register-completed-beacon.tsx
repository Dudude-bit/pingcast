"use client";

import { useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { track } from "@/lib/analytics";
import { useLocale } from "@/components/i18n/locale-provider";

// RegisterCompletedBeacon fires the `register_completed` Plausible
// event exactly once when the dashboard loads with ?registered=1,
// then strips the query param so a hard refresh doesn't double-count.
// The current locale is attached as a prop so we can split conversion
// by language in Plausible.
export function RegisterCompletedBeacon() {
  const { locale } = useLocale();
  const searchParams = useSearchParams();
  const router = useRouter();

  useEffect(() => {
    if (searchParams.get("registered") === "1") {
      track("register_completed", { lang: locale });
      // Replace URL to drop the query param. Stays on the same route;
      // no re-fetch because we don't use router.refresh().
      router.replace(`/${locale}/dashboard`);
    }
  }, [searchParams, router, locale]);

  return null;
}

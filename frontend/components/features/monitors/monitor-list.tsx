"use client";

import Link from "next/link";
import { useMonitors } from "@/lib/queries";
import { Skeleton } from "@/components/ui/skeleton";
import { buttonVariants } from "@/components/ui/button";
import { MonitorRow } from "./monitor-row";
import { Radio } from "lucide-react";
import { useLocale } from "@/components/i18n/locale-provider";

export function MonitorList() {
  const { dict, locale } = useLocale();
  const d = dict.dashboard;
  const { data, isLoading, error } = useMonitors();

  if (isLoading) {
    return (
      <div className="space-y-2">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-16 w-full rounded-lg" />
        ))}
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-lg border border-red-500/30 bg-red-500/5 p-6 text-sm text-red-700 dark:text-red-400">
        {dict.common.load_failed}: {error.message}
      </div>
    );
  }

  if (!data || data.length === 0) {
    return (
      <div className="rounded-lg border border-dashed border-border/60 bg-card py-16 px-6 text-center">
        <Radio className="mx-auto h-10 w-10 text-muted-foreground/60" />
        <h3 className="mt-4 text-base font-semibold">{d.monitors_empty}</h3>
        <Link
          href={`/${locale}/monitors/new`}
          className={`${buttonVariants()} mt-6`}
        >
          {dict.monitors.submit_create}
        </Link>
      </div>
    );
  }

  return (
    <div className="space-y-2">
      {data.map((m) => (
        <MonitorRow key={m.id} m={m} />
      ))}
    </div>
  );
}

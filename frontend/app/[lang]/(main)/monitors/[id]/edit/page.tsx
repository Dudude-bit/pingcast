"use client";

import { use } from "react";
import { useMonitor } from "@/lib/queries";
import { MonitorForm } from "@/components/features/monitors/monitor-form";
import { Skeleton } from "@/components/ui/skeleton";
import { useLocale } from "@/components/i18n/locale-provider";

export default function EditMonitorPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const { dict } = useLocale();
  const { data, isLoading, error } = useMonitor(id);

  if (isLoading) {
    return (
      <div className="container mx-auto px-4 py-8 max-w-xl space-y-4">
        <Skeleton className="h-6 w-32" />
        <Skeleton className="h-96 w-full" />
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="container mx-auto px-4 py-8 max-w-xl">
        <div className="rounded-lg border border-red-500/30 bg-red-500/5 p-6 text-sm text-red-700 dark:text-red-400">
          {error?.message ?? dict.monitors.not_found}
        </div>
      </div>
    );
  }

  return <MonitorForm mode="edit" initial={data} />;
}

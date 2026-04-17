"use client";

import { use, useState } from "react";
import Link from "next/link";
import { ArrowLeft, Pencil, Trash2 } from "lucide-react";
import { useMonitor } from "@/lib/queries";
import { Button, buttonVariants } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { StatusBadge } from "@/components/features/monitors/status-badge";
import { UptimeStats } from "@/components/features/monitors/uptime-stats";
import { IncidentList } from "@/components/features/monitors/incident-list";
import { DeleteMonitorDialog } from "@/components/features/monitors/delete-monitor-dialog";

export default function MonitorDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const { data, isLoading, error } = useMonitor(id);
  const [deleteOpen, setDeleteOpen] = useState(false);

  if (isLoading) {
    return (
      <div className="container mx-auto px-4 py-8 max-w-4xl space-y-6">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-48 w-full" />
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="container mx-auto px-4 py-8 max-w-4xl">
        <div className="rounded-lg border border-red-500/30 bg-red-500/5 p-6 text-sm text-red-700 dark:text-red-400">
          {error?.message ?? "Monitor not found"}
        </div>
      </div>
    );
  }

  const incidents = data.incidents ?? [];

  return (
    <div className="container mx-auto px-4 py-8 max-w-4xl space-y-6">
      <Link
        href="/dashboard"
        className="inline-flex items-center text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft className="mr-1 h-4 w-4" /> Back to dashboard
      </Link>

      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div className="min-w-0">
          <h1 className="text-2xl font-bold tracking-tight flex items-center gap-3">
            {data.name}
            <StatusBadge status={data.current_status} />
          </h1>
          <p className="mt-1 text-sm text-muted-foreground break-all">
            {data.target}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Link
            href={`/monitors/${id}/edit`}
            className={buttonVariants({ variant: "outline", size: "sm" })}
          >
            <Pencil className="mr-1.5 h-4 w-4" /> Edit
          </Link>
          <Button
            variant="destructive"
            size="sm"
            onClick={() => setDeleteOpen(true)}
          >
            <Trash2 className="mr-1.5 h-4 w-4" /> Delete
          </Button>
        </div>
      </div>

      <UptimeStats
        u24={data.uptime_24h ?? 0}
        u7={data.uptime_7d ?? 0}
        u30={data.uptime_30d ?? 0}
      />

      <div className="rounded-lg border border-border/60 bg-card p-6">
        <h3 className="text-sm font-semibold mb-3">Response time</h3>
        <div className="h-40 flex items-center justify-center rounded-md bg-muted/30 text-sm text-muted-foreground">
          Response-time chart — coming soon
        </div>
      </div>

      <div>
        <h3 className="text-sm font-semibold mb-3">Incidents</h3>
        <IncidentList incidents={incidents} />
      </div>

      <DeleteMonitorDialog
        monitorId={id}
        monitorName={data.name ?? "this monitor"}
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        redirectOnSuccess="/dashboard"
      />
    </div>
  );
}

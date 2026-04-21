"use client";

import { Suspense } from "react";
import Link from "next/link";
import { Plus } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { MonitorList } from "@/components/features/monitors/monitor-list";
import { GettingStarted } from "@/components/features/common/getting-started";
import { RegisterCompletedBeacon } from "@/components/analytics/register-completed-beacon";

export default function DashboardPage() {
  return (
    <div className="container mx-auto px-4 py-8 max-w-5xl">
      <Suspense fallback={null}>
        <RegisterCompletedBeacon />
      </Suspense>
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-8">
        <div className="min-w-0">
          <h1 className="text-2xl font-bold tracking-tight">Monitors</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Your endpoints, checked every minute. Live status updates every 15 seconds.
          </p>
        </div>
        <Link
          href="/monitors/new"
          className={`${buttonVariants()} shrink-0 self-start sm:self-auto`}
        >
          <Plus className="mr-2 h-4 w-4" /> New monitor
        </Link>
      </div>
      <GettingStarted />
      <MonitorList />
    </div>
  );
}

"use client";

import Link from "next/link";
import { Plus } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { MonitorList } from "@/components/features/monitors/monitor-list";

export default function DashboardPage() {
  return (
    <div className="container mx-auto px-4 py-8 max-w-5xl">
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Monitors</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Your endpoints, checked every minute. Live status updates every 15 seconds.
          </p>
        </div>
        <Link href="/monitors/new" className={buttonVariants()}>
          <Plus className="mr-2 h-4 w-4" /> New monitor
        </Link>
      </div>
      <MonitorList />
    </div>
  );
}

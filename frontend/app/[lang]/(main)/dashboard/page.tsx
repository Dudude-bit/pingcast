"use client";

import { Suspense } from "react";
import Link from "next/link";
import { Plus } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { MonitorList } from "@/components/features/monitors/monitor-list";
import { GettingStarted } from "@/components/features/common/getting-started";
import { RegisterCompletedBeacon } from "@/components/analytics/register-completed-beacon";
import { UpgradeButton } from "@/components/features/billing/upgrade-button";
import { ProNav } from "@/components/features/common/pro-nav";
import { useLocale } from "@/components/i18n/locale-provider";

export default function DashboardPage() {
  const { dict, locale } = useLocale();
  const d = dict.dashboard;
  return (
    <div className="container mx-auto px-4 py-8 max-w-5xl">
      <Suspense fallback={null}>
        <RegisterCompletedBeacon />
      </Suspense>
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-8">
        <div className="min-w-0">
          <h1 className="text-2xl font-bold tracking-tight">{d.monitors_heading}</h1>
        </div>
        <div className="flex items-center gap-2 shrink-0 self-start sm:self-auto">
          <UpgradeButton size="default" />
          <Link href={`/${locale}/monitors/new`} className={buttonVariants()}>
            <Plus className="mr-2 h-4 w-4" /> {d.monitors_add}
          </Link>
        </div>
      </div>
      <GettingStarted />
      <MonitorList />
      <ProNav />
    </div>
  );
}

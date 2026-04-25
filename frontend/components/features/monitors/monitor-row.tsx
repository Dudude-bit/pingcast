"use client";

import { useState } from "react";
import Link from "next/link";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { buttonVariants } from "@/components/ui/button";
import { MoreHorizontal, Pause, Play, Pencil, Trash2 } from "lucide-react";
import { useRouter } from "next/navigation";
import { useTogglePause } from "@/lib/mutations";
import type { MonitorWithUptime } from "@/lib/queries";
import { StatusDot } from "./status-badge";
import { DeleteMonitorDialog } from "./delete-monitor-dialog";
import { cn } from "@/lib/utils";
import { useLocale } from "@/components/i18n/locale-provider";

function uptimeColor(u: number) {
  if (u >= 99.5) return "text-emerald-600 dark:text-emerald-400";
  if (u >= 95.0) return "text-amber-600 dark:text-amber-400";
  return "text-red-600 dark:text-red-400";
}

export function MonitorRow({ m }: { m: MonitorWithUptime }) {
  const { dict, locale } = useLocale();
  const t = dict.monitors;
  const router = useRouter();
  const [deleteOpen, setDeleteOpen] = useState(false);
  const toggle = useTogglePause();
  const uptime = m.uptime_24h ?? 0;
  const paused = m.is_paused ?? false;

  return (
    <div className="group flex items-center gap-3 sm:gap-4 rounded-lg border border-border/60 bg-card px-3 sm:px-4 py-3 hover:border-border hover:bg-accent/30 transition-colors">
      <StatusDot status={m.current_status} pulse={!paused} />

      <Link
        href={`/${locale}/monitors/${m.id}`}
        className="flex-1 min-w-0 focus:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-md -mx-1 px-1"
      >
        <div className="flex items-center gap-2">
          <span className="font-medium truncate">{m.name}</span>
          {paused ? (
            <span className="text-xs px-1.5 py-0.5 rounded bg-zinc-500/15 text-zinc-600 dark:text-zinc-400 shrink-0">
              {t.row_paused_chip}
            </span>
          ) : null}
        </div>
        <div className="text-xs text-muted-foreground truncate mt-0.5">
          {m.target ?? m.type}
        </div>
      </Link>

      <span className="hidden md:inline text-xs text-muted-foreground tabular-nums">
        {m.interval_seconds ?? 300}s
      </span>

      <span
        className={cn(
          "text-sm font-semibold tabular-nums w-12 sm:w-16 text-right shrink-0",
          uptimeColor(uptime),
        )}
      >
        {uptime.toFixed(1)}%
      </span>

      <DropdownMenu>
        <DropdownMenuTrigger
          className={buttonVariants({ variant: "ghost", size: "icon-sm" })}
          aria-label={t.row_actions_label}
        >
          <MoreHorizontal className="h-4 w-4" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem
            onClick={() => router.push(`/${locale}/monitors/${m.id}/edit`)}
          >
            <Pencil className="mr-2 h-4 w-4" /> {dict.common.edit}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => m.id && toggle.mutate(m.id)}
            disabled={toggle.isPending}
          >
            {paused ? (
              <>
                <Play className="mr-2 h-4 w-4" /> {t.row_resume}
              </>
            ) : (
              <>
                <Pause className="mr-2 h-4 w-4" /> {t.row_pause}
              </>
            )}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            onClick={() => setDeleteOpen(true)}
            className="text-red-600 focus:text-red-600 focus:bg-red-500/10"
          >
            <Trash2 className="mr-2 h-4 w-4" /> {dict.common.delete}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <DeleteMonitorDialog
        monitorId={m.id ?? ""}
        monitorName={m.name ?? ""}
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
      />
    </div>
  );
}

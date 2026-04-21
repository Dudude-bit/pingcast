import { cn } from "@/lib/utils";

const STATE_COLORS: Record<string, string> = {
  investigating:
    "bg-amber-500/15 text-amber-700 dark:text-amber-300 border border-amber-500/30",
  identified:
    "bg-orange-500/15 text-orange-700 dark:text-orange-300 border border-orange-500/30",
  monitoring:
    "bg-blue-500/15 text-blue-700 dark:text-blue-300 border border-blue-500/30",
  resolved:
    "bg-emerald-500/15 text-emerald-700 dark:text-emerald-300 border border-emerald-500/30",
};

const STATE_LABELS: Record<string, string> = {
  investigating: "Investigating",
  identified: "Identified",
  monitoring: "Monitoring",
  resolved: "Resolved",
};

export function IncidentStateBadge({
  state,
  className,
}: {
  state: string;
  className?: string;
}) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium tracking-tight",
        STATE_COLORS[state] ?? "bg-muted text-muted-foreground border",
        className,
      )}
    >
      {STATE_LABELS[state] ?? state}
    </span>
  );
}

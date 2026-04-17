import { cn } from "@/lib/utils";

function uptimeColor(u: number) {
  if (u >= 99.5) return "text-emerald-600 dark:text-emerald-400";
  if (u >= 95.0) return "text-amber-600 dark:text-amber-400";
  return "text-red-600 dark:text-red-400";
}

function StatCard({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-lg border border-border/60 bg-card p-5">
      <div className="text-xs uppercase tracking-wider text-muted-foreground">
        {label}
      </div>
      <div className={cn("mt-1 text-3xl font-bold tabular-nums", uptimeColor(value))}>
        {value.toFixed(2)}
        <span className="text-lg text-muted-foreground font-normal">%</span>
      </div>
    </div>
  );
}

export function UptimeStats({
  u24,
  u7,
  u30,
}: {
  u24: number;
  u7: number;
  u30: number;
}) {
  return (
    <div className="grid gap-3 sm:grid-cols-3">
      <StatCard label="Uptime 24h" value={u24} />
      <StatCard label="Uptime 7d" value={u7} />
      <StatCard label="Uptime 30d" value={u30} />
    </div>
  );
}

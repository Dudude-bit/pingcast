import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

type Status = "up" | "down" | "unknown" | (string & {});

/**
 * StatusDot — colored pill showing up/down/unknown. When `pulse` is true
 * and the status is definite (up or down), the dot radiates a subtle
 * ping ring matching the colour. Used on live-polled views to signal
 * that the data is breathing.
 */
export function StatusDot({
  status,
  pulse = false,
}: {
  status?: Status;
  pulse?: boolean;
}) {
  const color =
    status === "up"
      ? "bg-emerald-500"
      : status === "down"
        ? "bg-red-500"
        : "bg-zinc-400";
  return (
    <span
      className={cn(
        "relative inline-block h-2.5 w-2.5 rounded-full ring-2 ring-background shrink-0",
        color,
      )}
      aria-label={`status ${status ?? "unknown"}`}
    >
      {pulse && status !== "unknown" ? (
        <span
          className={cn(
            "absolute inset-0 rounded-full opacity-60 animate-ping",
            color,
          )}
          aria-hidden="true"
        />
      ) : null}
    </span>
  );
}

export function StatusBadge({ status }: { status?: Status }) {
  const variant =
    status === "up"
      ? "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 border-emerald-500/20"
      : status === "down"
        ? "bg-red-500/10 text-red-700 dark:text-red-400 border-red-500/20"
        : "bg-zinc-500/10 text-zinc-600 dark:text-zinc-400 border-zinc-500/20";
  return (
    <Badge className={cn("gap-1.5 capitalize font-medium", variant)}>
      <StatusDot status={status} />
      {status ?? "unknown"}
    </Badge>
  );
}

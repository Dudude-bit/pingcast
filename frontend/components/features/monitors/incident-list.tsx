import type { Incident } from "@/lib/queries";
import { CheckCircle2, AlertCircle } from "lucide-react";

function formatDate(iso?: string | null) {
  if (!iso) return "";
  const d = new Date(iso);
  return d.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function IncidentList({ incidents }: { incidents: Incident[] }) {
  if (incidents.length === 0) {
    return (
      <div className="flex items-center gap-3 rounded-lg border border-border/60 bg-card p-6 text-sm text-muted-foreground">
        <CheckCircle2 className="h-5 w-5 text-emerald-500" />
        <span>No incidents recorded.</span>
      </div>
    );
  }

  return (
    <ul className="divide-y divide-border/60 rounded-lg border border-border/60 bg-card">
      {incidents.map((inc) => (
        <li key={inc.id} className="p-4">
          <div className="flex items-start gap-3">
            <AlertCircle
              className={
                inc.resolved_at
                  ? "mt-0.5 h-4 w-4 text-muted-foreground"
                  : "mt-0.5 h-4 w-4 text-red-500"
              }
            />
            <div className="flex-1 min-w-0">
              <div className="text-sm font-medium truncate">
                {inc.cause || "Check failed"}
              </div>
              <div className="mt-1 text-xs text-muted-foreground">
                Started {formatDate(inc.started_at)}
                {inc.resolved_at ? (
                  <> · Resolved {formatDate(inc.resolved_at)}</>
                ) : (
                  <span className="text-red-600 dark:text-red-400"> · Ongoing</span>
                )}
              </div>
            </div>
          </div>
        </li>
      ))}
    </ul>
  );
}

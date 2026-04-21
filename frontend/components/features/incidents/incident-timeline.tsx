import type { components } from "@/lib/openapi-types";
import { IncidentStateBadge } from "./incident-state-badge";

type IncidentUpdate = components["schemas"]["IncidentUpdate"];

function formatWhen(iso: string) {
  return new Date(iso).toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function IncidentTimeline({ updates }: { updates: IncidentUpdate[] }) {
  if (!updates.length) return null;
  return (
    <ol className="relative mt-3 space-y-3 border-l border-border/60 pl-5">
      {updates.map((u) => (
        <li key={u.id} className="relative">
          <span className="absolute -left-[27px] top-1.5 h-2.5 w-2.5 rounded-full bg-primary ring-4 ring-background" />
          <div className="flex items-center gap-2 flex-wrap">
            <IncidentStateBadge state={u.state} />
            <time className="text-xs text-muted-foreground tabular-nums">
              {formatWhen(u.posted_at)}
            </time>
          </div>
          <p className="mt-1.5 text-sm text-foreground leading-relaxed whitespace-pre-line">
            {u.body}
          </p>
        </li>
      ))}
    </ol>
  );
}

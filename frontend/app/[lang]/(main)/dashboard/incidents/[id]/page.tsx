import { notFound } from "next/navigation";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { apiFetch } from "@/lib/api";
import type { components } from "@/lib/openapi-types";
import { IncidentStateBadge } from "@/components/features/incidents/incident-state-badge";
import { IncidentTimeline } from "@/components/features/incidents/incident-timeline";
import { IncidentUpdateForm } from "@/components/features/incidents/incident-update-form";
import { buttonVariants } from "@/components/ui/button";

export const dynamic = "force-dynamic";

type IncidentUpdate = components["schemas"]["IncidentUpdate"];

async function fetchUpdates(id: number): Promise<IncidentUpdate[]> {
  try {
    return await apiFetch<IncidentUpdate[]>(`/incidents/${id}/updates`);
  } catch {
    return [];
  }
}

function formatWhen(iso: string) {
  return new Date(iso).toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export default async function IncidentDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const incidentID = Number(id);
  if (!Number.isFinite(incidentID) || incidentID <= 0) notFound();

  const updates = await fetchUpdates(incidentID);
  if (updates.length === 0) notFound();

  // Derive current state and first-post details from the timeline (the
  // public endpoint already exposes what we need — no separate
  // get-incident-by-id call required).
  const currentState = updates[0].state; // list is DESC on posted_at
  const opened = updates[updates.length - 1];

  return (
    <div className="container mx-auto px-4 py-8 max-w-2xl">
      <Link
        href="/dashboard"
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mb-6"
      >
        <ArrowLeft className="h-3.5 w-3.5" /> Back to dashboard
      </Link>

      <div className="flex items-center gap-3 flex-wrap">
        <h1 className="text-2xl font-bold tracking-tight">
          Incident #{incidentID}
        </h1>
        <IncidentStateBadge state={currentState} />
      </div>
      <p className="mt-2 text-sm text-muted-foreground">
        Opened {formatWhen(opened.posted_at)}
      </p>

      <section className="mt-8 rounded-lg border border-border/60 bg-card p-5">
        <h2 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground mb-4">
          Post an update
        </h2>
        <IncidentUpdateForm
          incidentId={incidentID}
          currentState={currentState}
        />
      </section>

      <section className="mt-8">
        <h2 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground mb-4">
          Timeline
        </h2>
        <IncidentTimeline updates={updates} />
      </section>

      <p className="mt-10 text-xs text-muted-foreground">
        Public timeline link:{" "}
        <Link
          href={`/api/incidents/${incidentID}/updates`}
          className={`${buttonVariants({ variant: "link" })} px-0 h-auto`}
        >
          /api/incidents/{incidentID}/updates
        </Link>
      </p>
    </div>
  );
}

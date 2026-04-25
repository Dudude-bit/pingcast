import { notFound } from "next/navigation";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { apiFetch } from "@/lib/api";
import type { components } from "@/lib/openapi-types";
import { IncidentStateBadge } from "@/components/features/incidents/incident-state-badge";
import { IncidentTimeline } from "@/components/features/incidents/incident-timeline";
import { IncidentUpdateForm } from "@/components/features/incidents/incident-update-form";
import { buttonVariants } from "@/components/ui/button";
import { getDictionary, hasLocale } from "@/lib/i18n";

export const dynamic = "force-dynamic";

type IncidentUpdate = components["schemas"]["IncidentUpdate"];
type Params = Promise<{ lang: string; id: string }>;

async function fetchUpdates(id: number): Promise<IncidentUpdate[]> {
  try {
    return await apiFetch<IncidentUpdate[]>(`/incidents/${id}/updates`);
  } catch {
    return [];
  }
}

function formatWhen(iso: string, locale: string) {
  return new Date(iso).toLocaleString(locale === "ru" ? "ru-RU" : "en-US", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export default async function IncidentDetailPage({ params }: { params: Params }) {
  const { lang, id } = await params;
  if (!hasLocale(lang)) notFound();
  const dict = await getDictionary(lang);
  const t = dict.dashboard_incidents;
  const incidentID = Number(id);
  if (!Number.isFinite(incidentID) || incidentID <= 0) notFound();

  const updates = await fetchUpdates(incidentID);
  if (updates.length === 0) notFound();

  const currentState = updates[0].state;
  const opened = updates[updates.length - 1];

  return (
    <div className="container mx-auto px-4 py-8 max-w-2xl">
      <Link
        href={`/${lang}/dashboard`}
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mb-6"
      >
        <ArrowLeft className="h-3.5 w-3.5" /> {dict.common.back_to_dashboard}
      </Link>

      <div className="flex items-center gap-3 flex-wrap">
        <h1 className="text-2xl font-bold tracking-tight">
          {t.title} #{incidentID}
        </h1>
        <IncidentStateBadge state={currentState} />
      </div>
      <p className="mt-2 text-sm text-muted-foreground">
        {t.started_at}: {formatWhen(opened.posted_at, lang)}
      </p>

      <section className="mt-8 rounded-lg border border-border/60 bg-card p-5">
        <h2 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground mb-4">
          {t.post_update_heading}
        </h2>
        <IncidentUpdateForm
          incidentId={incidentID}
          currentState={currentState}
        />
      </section>

      <section className="mt-8">
        <h2 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground mb-4">
          {t.timeline_heading}
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

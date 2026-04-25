import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { apiFetch, ApiError } from "@/lib/api";
import type { components } from "@/lib/openapi-types";
import { Activity, AlertCircle, CheckCircle2 } from "lucide-react";
import { IncidentStateBadge } from "@/components/features/incidents/incident-state-badge";
import { IncidentTimeline } from "@/components/features/incidents/incident-timeline";
import { StatusSubscribeForm } from "@/components/features/incidents/status-subscribe-form";
import { getDictionary } from "@/lib/i18n";
import { pickLocaleFromHeaders } from "@/lib/locale-from-headers";

type IncidentUpdate = components["schemas"]["IncidentUpdate"];
type Incident = components["schemas"]["Incident"];

async function fetchIncidentUpdates(id: number): Promise<IncidentUpdate[]> {
  try {
    return await apiFetch<IncidentUpdate[]>(`/incidents/${id}/updates`);
  } catch {
    return [];
  }
}

export const revalidate = 30;

type StatusPage = components["schemas"]["StatusPageResponse"];

async function fetchStatus(slug: string): Promise<StatusPage | null> {
  try {
    return await apiFetch<StatusPage>(`/status/${slug}`);
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) return null;
    throw e;
  }
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ slug: string }>;
}): Promise<Metadata> {
  const { slug } = await params;
  const locale = await pickLocaleFromHeaders();
  const dict = await getDictionary(locale);
  const data = await fetchStatus(slug).catch(() => null);
  if (!data) return { title: dict.status_page.meta_title_unknown };
  const title = data.all_up
    ? dict.status_page.meta_title_all_up
    : dict.status_page.meta_title_degraded;
  return {
    title: `${title} · ${data.slug}`,
    description: dict.status_page.meta_description.replace(
      "{slug}",
      data.slug ?? slug,
    ),
    robots: { index: true, follow: true },
  };
}

function uptimeColor(u: number) {
  if (u >= 99.5) return "text-emerald-600 dark:text-emerald-400";
  if (u >= 95.0) return "text-amber-600 dark:text-amber-400";
  return "text-red-600 dark:text-red-400";
}

function formatDate(iso: string | null | undefined, locale: string) {
  if (!iso) return "";
  return new Date(iso).toLocaleString(locale === "ru" ? "ru-RU" : "en-US", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export default async function StatusPageRoute({
  params,
}: {
  params: Promise<{ slug: string }>;
}) {
  const { slug } = await params;
  const locale = await pickLocaleFromHeaders();
  const dict = await getDictionary(locale);
  const t = dict.status_page;
  const data = await fetchStatus(slug);
  if (!data) notFound();

  const allUp = data.all_up ?? true;
  const monitors = data.monitors ?? [];
  const incidents: Incident[] = data.incidents ?? [];
  const groups = data.groups ?? [];

  const UNGROUPED = "__ungrouped__";
  const byGroup = new Map<string, typeof monitors>();
  for (const m of monitors) {
    const key =
      typeof m.group_id === "number" ? String(m.group_id) : UNGROUPED;
    const bucket = byGroup.get(key) ?? [];
    bucket.push(m);
    byGroup.set(key, bucket);
  }

  const sections: { key: string; title: string | null; items: typeof monitors }[] = [];
  for (const g of [...groups].sort((a, b) => a.ordering - b.ordering)) {
    const items = byGroup.get(String(g.id));
    if (items && items.length > 0) {
      sections.push({ key: String(g.id), title: g.name, items });
    }
  }
  const ungrouped = byGroup.get(UNGROUPED);
  if (ungrouped && ungrouped.length > 0) {
    sections.push({
      key: UNGROUPED,
      title: sections.length > 0 ? t.other_services : null,
      items: ungrouped,
    });
  }

  const timelines = await Promise.all(
    incidents.map((inc) => fetchIncidentUpdates(inc.id)),
  );

  const branding = data.branding;
  const accent = branding?.accent_color ?? undefined;

  return (
    <div
      className="min-h-screen bg-background"
      style={accent ? ({ ["--brand-accent" as string]: accent } as React.CSSProperties) : undefined}
    >
      <div className="container mx-auto px-4 py-12 max-w-2xl">
        <header className="mb-10 text-center">
          {branding?.logo_url ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={branding.logo_url}
              alt={`${data.slug} logo`}
              className="mx-auto mb-4 h-12 w-auto object-contain"
            />
          ) : null}
          <div className="inline-flex items-center justify-center gap-2 text-xs uppercase tracking-wider text-muted-foreground">
            <Activity className="h-3 w-3" />
            <span>{data.slug}</span>
          </div>
          <div
            className={`mt-6 rounded-2xl border px-8 py-10 ${
              allUp
                ? "border-emerald-500/30 bg-emerald-500/5"
                : "border-red-500/30 bg-red-500/5"
            }`}
          >
            <div className="inline-flex items-center justify-center">
              {allUp ? (
                <CheckCircle2 className="h-12 w-12 text-emerald-500" />
              ) : (
                <AlertCircle className="h-12 w-12 text-red-500" />
              )}
            </div>
            <h1 className="mt-4 text-3xl font-bold tracking-tight">
              {allUp ? t.all_up : t.some_down}
            </h1>
            <p className="mt-2 text-sm text-muted-foreground">{t.auto_refresh}</p>
          </div>
        </header>

        <section className="mb-10">
          <h2 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground mb-3">
            {t.services_heading}
          </h2>
          {monitors.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t.no_services}</p>
          ) : sections.length === 0 ? null : (
            <div className="space-y-6">
              {sections.map((section) => (
                <div key={section.key}>
                  {section.title ? (
                    <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-2 px-1">
                      {section.title}
                    </h3>
                  ) : null}
                  <ul className="divide-y divide-border/60 rounded-lg border border-border/60 bg-card">
                    {section.items.map((m, i) => {
                      const status = m.current_status ?? "unknown";
                      const uptime = m.uptime_90d ?? 0;
                      const inMaintenance = Boolean(m.in_maintenance);
                      return (
                        <li
                          key={`${section.key}-${i}`}
                          className="flex items-center justify-between p-4"
                        >
                          <div className="flex items-center gap-3 min-w-0">
                            <span
                              className={`inline-block h-2.5 w-2.5 rounded-full shrink-0 ${
                                inMaintenance
                                  ? "bg-blue-500"
                                  : status === "up"
                                    ? "bg-emerald-500"
                                    : status === "down"
                                      ? "bg-red-500"
                                      : "bg-zinc-400"
                              }`}
                            />
                            <span className="font-medium truncate">{m.name}</span>
                          </div>
                          <div className="flex items-center gap-4 shrink-0">
                            {inMaintenance ? (
                              <span className="text-xs uppercase tracking-wider text-blue-700 dark:text-blue-300 text-right">
                                {t.scheduled_maintenance}
                              </span>
                            ) : (
                              <>
                                <span
                                  className={`text-sm font-semibold tabular-nums ${uptimeColor(uptime)}`}
                                >
                                  {uptime.toFixed(2)}%
                                </span>
                                <span className="text-xs uppercase tracking-wider text-muted-foreground capitalize w-20 text-right">
                                  {status === "up"
                                    ? t.status_up
                                    : status === "down"
                                      ? t.status_down
                                      : t.status_unknown}
                                </span>
                              </>
                            )}
                          </div>
                        </li>
                      );
                    })}
                  </ul>
                </div>
              ))}
            </div>
          )}
        </section>

        {incidents.length > 0 ? (
          <section className="mb-10">
            <h2 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground mb-3">
              {t.incidents_heading}
            </h2>
            <ul className="divide-y divide-border/60 rounded-lg border border-border/60 bg-card">
              {incidents.map((inc, idx) => {
                const timeline = timelines[idx] ?? [];
                const headline = inc.title ?? inc.cause ?? "Check failed";
                return (
                  <li key={inc.id} className="p-4">
                    <div className="flex items-start gap-3">
                      <AlertCircle
                        className={`mt-0.5 h-4 w-4 shrink-0 ${
                          inc.resolved_at
                            ? "text-muted-foreground"
                            : "text-red-500"
                        }`}
                      />
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 flex-wrap">
                          <span className="text-sm font-medium">{headline}</span>
                          <IncidentStateBadge state={inc.state} />
                        </div>
                        <div className="mt-1 text-xs text-muted-foreground">
                          {t.incident_started} {formatDate(inc.started_at, locale)}
                          {inc.resolved_at ? (
                            <> · {t.incident_resolved} {formatDate(inc.resolved_at, locale)}</>
                          ) : (
                            <span className="text-red-600 dark:text-red-400">
                              {" · "}{t.incident_ongoing}
                            </span>
                          )}
                        </div>
                        <IncidentTimeline updates={timeline} />
                      </div>
                    </div>
                  </li>
                );
              })}
            </ul>
          </section>
        ) : null}

        <section className="mt-8 mb-10">
          <StatusSubscribeForm
            slug={data.slug ?? slug}
            locale={locale}
            labels={{
              heading: t.subscribe_heading,
              placeholder: t.subscribe_email_placeholder,
              button: t.subscribe_button,
              busy: t.subscribe_busy,
              helper: t.subscribe_helper,
              sentHeading: t.subscribe_sent_heading,
              sentBody: t.subscribe_sent_body,
              failed: t.subscribe_failed,
            }}
            accentStyle={
              accent
                ? { backgroundColor: accent, borderColor: accent }
                : undefined
            }
          />
        </section>

        {branding?.custom_footer_text ? (
          <footer className="mt-8 text-center text-xs text-muted-foreground whitespace-pre-line">
            {branding.custom_footer_text}
          </footer>
        ) : null}

        {data.show_branding ? (
          <footer className="mt-8 text-center text-xs text-muted-foreground">
            {t.powered_by}{" "}
            <Link href={`/${locale}`} className="underline hover:text-foreground">
              PingCast
            </Link>
          </footer>
        ) : null}
      </div>
    </div>
  );
}

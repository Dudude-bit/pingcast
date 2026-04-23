import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { BreadcrumbListJsonLd } from "@/components/seo/jsonld";

export const metadata: Metadata = {
  title: "Status page template — copy-paste incident-update phrasing",
  description:
    "Canned incident-update copy for each state (investigating / identified / monitoring / resolved). Plus structural templates for the public page itself.",
  alternates: { canonical: "/status-page-template" },
};

type Template = { state: string; label: string; body: string };

const TEMPLATES: Template[] = [
  {
    state: "investigating",
    label: "Initial post — just saw it, don't know why yet",
    body: "We're seeing elevated error rates on <service/surface>. Our team is investigating. We'll post an update within 15 minutes.",
  },
  {
    state: "investigating",
    label: "Initial post — we paged a human",
    body: "We've received alerts that <service> is not responding for some users. An engineer is investigating. We'll update as we learn more.",
  },
  {
    state: "identified",
    label: "Cause found, fix in progress",
    body: "Cause identified: <plain-English summary — e.g. 'a bad config push to our API gateway caused requests to time out'>. Mitigation is in progress. ETA to recovery: <X minutes>.",
  },
  {
    state: "identified",
    label: "Third-party dependency",
    body: "The issue is a partial outage at <upstream provider>. We're seeing <X>% error rate on <surface>. We've opened a ticket with them and are exploring a workaround. See also: <upstream status URL>.",
  },
  {
    state: "monitoring",
    label: "Fix deployed, watching",
    body: "Fix deployed. Error rates are returning to baseline. We'll continue monitoring for the next 15 minutes and mark this resolved if everything holds.",
  },
  {
    state: "resolved",
    label: "Clean recovery",
    body: "All services have recovered. Total incident duration: <MM> minutes. We'll publish a post-mortem within the next 48 hours at <blog URL or TBD>. Sorry for the disruption.",
  },
  {
    state: "resolved",
    label: "Partial / lingering effects",
    body: "Primary services recovered at <HH:MM UTC>. <Customers with X configuration> may still see stale data until <Y>; this will resolve on its own. Post-mortem to follow.",
  },
];

export default function StatusPageTemplatePage() {
  return (
    <div className="container mx-auto px-4 py-12 max-w-3xl">
      <BreadcrumbListJsonLd
        items={[
          { name: "Home", url: "/" },
          { name: "Status page template", url: "/status-page-template" },
        ]}
      />
      <h1 className="text-4xl md:text-5xl font-bold tracking-tight leading-tight">
        Status page template + incident-update phrasing
      </h1>
      <p className="mt-4 text-lg text-muted-foreground leading-relaxed">
        Writing incident updates at 3 AM while prod is on fire is a bad
        time to find your voice. Draft these phrases once, keep them
        handy, fill the blanks when things break.
      </p>

      <div className="mt-8 flex flex-wrap gap-3">
        <Link href="/register?intent=pro" className={buttonVariants({ size: "lg" })}>
          Use templates in PingCast <ArrowRight className="ml-2 h-4 w-4" />
        </Link>
        <Link
          href="/how-to-create-status-page"
          className={buttonVariants({ variant: "outline", size: "lg" })}
        >
          Setup guide
        </Link>
      </div>

      <section className="mt-12">
        <h2 className="text-2xl font-bold tracking-tight">Incident-update templates</h2>
        <div className="mt-6 space-y-5">
          {TEMPLATES.map((t, i) => (
            <div key={i} className="rounded-lg border border-border/60 bg-card p-5">
              <div className="flex items-center gap-3 flex-wrap">
                <span className="text-xs uppercase tracking-wider font-semibold text-primary">
                  {t.state}
                </span>
                <span className="text-xs text-muted-foreground">{t.label}</span>
              </div>
              <pre className="mt-3 text-sm text-foreground whitespace-pre-wrap font-sans leading-relaxed">
                {t.body}
              </pre>
            </div>
          ))}
        </div>
      </section>

      <section className="mt-12">
        <h2 className="text-2xl font-bold tracking-tight">Page-structure template</h2>
        <p className="mt-3 text-muted-foreground leading-relaxed">
          A good status page has four sections, in this order:
        </p>
        <ol className="mt-4 list-decimal pl-6 space-y-2 text-sm text-muted-foreground">
          <li>
            <strong>Current status banner</strong> — all-green or an active
            incident summary. First thing anyone sees.
          </li>
          <li>
            <strong>Services list</strong> — 3–8 monitored surfaces, each
            with a status chip + 90-day uptime %.
          </li>
          <li>
            <strong>Active incidents</strong> — timeline for any unresolved
            issue, with latest state on top.
          </li>
          <li>
            <strong>Subscribe box</strong> — email-update opt-in. Footer
            links to an RSS feed and your terms.
          </li>
        </ol>
        <p className="mt-4 text-sm text-muted-foreground leading-relaxed">
          This is exactly what PingCast renders at{" "}
          <code>/status/&lt;your-slug&gt;</code>. No layout work required on
          your side.
        </p>
      </section>

      <div className="mt-16 rounded-2xl border border-border/60 bg-card p-8 text-center">
        <h2 className="text-2xl font-bold tracking-tight">
          Skip the design, keep the templates
        </h2>
        <p className="mt-3 text-sm text-muted-foreground max-w-xl mx-auto">
          PingCast&apos;s status-page UI is already set. Copy the incident
          templates above into your runbook and you&apos;re ready for the
          next outage.
        </p>
        <Link href="/register?intent=pro" className={`${buttonVariants({ size: "lg" })} mt-5`}>
          Start now
        </Link>
      </div>
    </div>
  );
}

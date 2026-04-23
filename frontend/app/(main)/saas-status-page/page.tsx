import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight, Check } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { BreadcrumbListJsonLd } from "@/components/seo/jsonld";

export const metadata: Metadata = {
  title: "Status page for SaaS — PingCast ($9/mo, Atlassian importer)",
  description:
    "The status page every growing SaaS needs: custom domain, branded UI, incident timeline, email subscribers, uptime monitoring built in. $9/mo founder's price.",
  alternates: { canonical: "/saas-status-page" },
};

export default function SaaSStatusPagePage() {
  return (
    <div className="container mx-auto px-4 py-12 max-w-3xl">
      <BreadcrumbListJsonLd
        items={[
          { name: "Home", url: "/" },
          { name: "Status page for SaaS", url: "/saas-status-page" },
        ]}
      />
      <h1 className="text-4xl md:text-5xl font-bold tracking-tight leading-tight">
        The status page every growing SaaS eventually needs
      </h1>
      <p className="mt-4 text-lg text-muted-foreground leading-relaxed">
        Somewhere between your 10th customer and your 100th, support tickets
        start asking &ldquo;is this down?&rdquo; A public status page
        answers that question before they ask it — and signals to
        prospects that you know how to run production.
      </p>

      <div className="mt-8 flex flex-wrap gap-3">
        <Link href="/register?intent=pro" className={buttonVariants({ size: "lg" })}>
          Start PingCast Pro <ArrowRight className="ml-2 h-4 w-4" />
        </Link>
        <Link
          href="/alternatives/atlassian-statuspage"
          className={buttonVariants({ variant: "outline", size: "lg" })}
        >
          vs Atlassian
        </Link>
      </div>

      <section className="mt-12">
        <h2 className="text-2xl font-bold tracking-tight">
          What SaaS customers actually want to see
        </h2>
        <ul className="mt-4 space-y-3 text-sm">
          {[
            {
              title: "A URL on your domain",
              body: "status.yourcompany.com reads as a company asset. status.pingcast.io/your-slug reads as a third-party tool that might go away.",
            },
            {
              title: "The timeline, not just the light",
              body: "Green/red monitors tell them something broke. An incident timeline tells them you saw it, named it, deployed a fix, and verified recovery — that's trust.",
            },
            {
              title: "Opt-in email updates",
              body: "Customers who want to know ping-by-ping will subscribe. Customers who'd rather not get spammed won't. You don't get to force the choice.",
            },
            {
              title: "The badge in your README",
              body: "Developer customers pin an SVG status badge next to your GitHub link. Free public proof of uptime.",
            },
          ].map((f) => (
            <li key={f.title} className="flex gap-3">
              <Check className="h-5 w-5 text-primary shrink-0 mt-0.5" />
              <div>
                <strong>{f.title}</strong>
                <p className="text-muted-foreground mt-0.5">{f.body}</p>
              </div>
            </li>
          ))}
        </ul>
      </section>

      <section className="mt-12">
        <h2 className="text-2xl font-bold tracking-tight">Three stages, one tool</h2>
        <div className="mt-6 space-y-5">
          <Stage
            stage="0-50 customers"
            body="Free tier. 5 monitors, basic status page at pingcast.io/status/your-slug. Telegram alerts to your phone when things break. Costs nothing."
          />
          <Stage
            stage="50-500 customers"
            body="Pro at $9/mo. Custom domain (status.yourcompany.com), branded UI (your logo and colour), incident updates, email subscribers. The full SaaS-grade look."
          />
          <Stage
            stage="500+ customers"
            body="Stay on Pro or self-host under MIT for full data sovereignty. At 500+ customers the $108/year cost is invisible; the migration path exists regardless."
          />
        </div>
      </section>

      <div className="mt-16 rounded-2xl border border-border/60 bg-card p-8 text-center">
        <h2 className="text-2xl font-bold tracking-tight">
          Get it right before you need it
        </h2>
        <p className="mt-3 text-sm text-muted-foreground max-w-xl mx-auto">
          The wrong time to set up a status page is during an incident.
          Spin one up now, leave it quietly operational, and it&apos;s there
          when you need it.
        </p>
        <Link
          href="/register?intent=pro"
          className={`${buttonVariants({ size: "lg" })} mt-5`}
        >
          Get started
        </Link>
      </div>
    </div>
  );
}

function Stage({ stage, body }: { stage: string; body: string }) {
  return (
    <div className="rounded-lg border border-border/60 bg-card p-5">
      <h3 className="text-sm font-semibold uppercase tracking-wider text-primary">
        {stage}
      </h3>
      <p className="mt-2 text-sm text-muted-foreground leading-relaxed">{body}</p>
    </div>
  );
}

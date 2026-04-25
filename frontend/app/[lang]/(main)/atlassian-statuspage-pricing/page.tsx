import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { BreadcrumbListJsonLd } from "@/components/seo/jsonld";

export const metadata: Metadata = {
  title: "Atlassian Statuspage pricing explained (2026)",
  description:
    "Atlassian Statuspage pricing tiers — Hobby, Starter, Growth, Enterprise — with what you actually get at each level. Plus the $9/mo PingCast alternative that covers 90% of the Growth-tier features.",
  alternates: { canonical: "/atlassian-statuspage-pricing" },
};

type Tier = {
  name: string;
  price: string;
  annual: string;
  subscribers: string;
  audiences: number;
  sla: boolean;
  customerComms: boolean;
  bestFor: string;
};

const TIERS: Tier[] = [
  {
    name: "Hobby",
    price: "Free",
    annual: "$0",
    subscribers: "100",
    audiences: 0,
    sla: false,
    customerComms: false,
    bestFor: "Side projects that don't need a custom domain.",
  },
  {
    name: "Starter",
    price: "$29/mo",
    annual: "$348",
    subscribers: "2,000",
    audiences: 0,
    sla: false,
    customerComms: false,
    bestFor: "Solo-founder SaaS wanting a branded page.",
  },
  {
    name: "Growth",
    price: "$99/mo",
    annual: "$1,188",
    subscribers: "10,000",
    audiences: 2,
    sla: true,
    customerComms: true,
    bestFor: "Mid-market SaaS with tiered customer communications.",
  },
  {
    name: "Enterprise",
    price: "$1,499+/mo",
    annual: "$17,988+",
    subscribers: "Unlimited",
    audiences: 999,
    sla: true,
    customerComms: true,
    bestFor: "Fortune 500 with compliance + per-region audiences.",
  },
];

export default function AtlassianStatuspagePricingPage() {
  return (
    <div className="container mx-auto px-4 py-12 max-w-4xl">
      <BreadcrumbListJsonLd
        items={[
          { name: "Home", url: "/" },
          { name: "Atlassian Statuspage pricing", url: "/atlassian-statuspage-pricing" },
        ]}
      />
      <h1 className="text-4xl md:text-5xl font-bold tracking-tight leading-tight">
        Atlassian Statuspage pricing, explained
      </h1>
      <p className="mt-4 text-lg text-muted-foreground leading-relaxed">
        Public pricing pages aren&apos;t always clear about what jumps between
        tiers. Here&apos;s the breakdown, then the short version of when it
        makes sense vs when a $9/mo alternative does the same job.
      </p>

      <section className="mt-10 overflow-x-auto rounded-xl border border-border/60 bg-card">
        <table className="w-full text-sm">
          <thead className="bg-muted/40 text-xs uppercase tracking-wide text-muted-foreground">
            <tr>
              <th className="text-left font-medium px-4 py-3">Tier</th>
              <th className="text-left font-medium px-4 py-3">Monthly</th>
              <th className="text-left font-medium px-4 py-3">Annual</th>
              <th className="text-left font-medium px-4 py-3">Subscribers</th>
              <th className="text-left font-medium px-4 py-3">SLA reports</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border/50">
            {TIERS.map((t) => (
              <tr key={t.name}>
                <td className="px-4 py-3 font-medium">{t.name}</td>
                <td className="px-4 py-3">{t.price}</td>
                <td className="px-4 py-3 text-muted-foreground">{t.annual}</td>
                <td className="px-4 py-3 text-muted-foreground">{t.subscribers}</td>
                <td className="px-4 py-3 text-muted-foreground">{t.sla ? "Yes" : "No"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>

      <section className="mt-12">
        <h2 className="text-2xl font-bold tracking-tight">What you actually get at each tier</h2>
        <div className="mt-6 space-y-6">
          {TIERS.map((t) => (
            <div key={t.name} className="rounded-lg border border-border/60 bg-card p-5">
              <div className="flex items-baseline gap-3 flex-wrap">
                <h3 className="text-lg font-semibold">{t.name}</h3>
                <span className="text-sm text-muted-foreground">{t.price}</span>
              </div>
              <p className="mt-2 text-sm text-muted-foreground leading-relaxed">{t.bestFor}</p>
            </div>
          ))}
        </div>
      </section>

      <section className="mt-12 rounded-2xl border-2 border-primary/40 bg-card p-8">
        <h2 className="text-2xl font-bold tracking-tight">
          When PingCast replaces Atlassian — and when it doesn&apos;t
        </h2>
        <div className="mt-5 grid md:grid-cols-2 gap-6 text-sm">
          <div>
            <h3 className="font-semibold mb-3">PingCast $9/mo covers</h3>
            <ul className="space-y-2 text-muted-foreground">
              <li>Custom domain + branded UI (Starter-tier feature)</li>
              <li>Email subscribers with double opt-in</li>
              <li>Incident updates with state timeline</li>
              <li>Uptime monitoring built in (Atlassian needs you to BYO)</li>
              <li>MIT self-host escape hatch if you outgrow hosted</li>
            </ul>
          </div>
          <div>
            <h3 className="font-semibold mb-3">Stay on Atlassian for</h3>
            <ul className="space-y-2 text-muted-foreground">
              <li>Per-audience subscriber carve-outs (Growth+ feature)</li>
              <li>Built-in SLA reports with downtime-bucket math</li>
              <li>Jira / Confluence tight integration</li>
              <li>Enterprise single-tenancy with regional isolation</li>
            </ul>
          </div>
        </div>
      </section>

      <div className="mt-12 rounded-2xl border border-border/60 bg-card p-8 text-center">
        <h2 className="text-2xl font-bold tracking-tight">
          Save $240-1,080/year by starting on PingCast
        </h2>
        <p className="mt-3 text-sm text-muted-foreground max-w-xl mx-auto">
          Import your Atlassian Statuspage in one click. If you end up needing
          the Growth tier later, you can always move back — your data exports
          cleanly.
        </p>
        <div className="mt-5 flex gap-3 justify-center flex-wrap">
          <Link
            href="/register?intent=pro"
            className={buttonVariants({ size: "lg" })}
          >
            Start PingCast Pro <ArrowRight className="ml-2 h-4 w-4" />
          </Link>
          <Link
            href="/alternatives/atlassian-statuspage"
            className={buttonVariants({ variant: "outline", size: "lg" })}
          >
            Full comparison
          </Link>
        </div>
      </div>
    </div>
  );
}

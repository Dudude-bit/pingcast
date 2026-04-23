import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { ALTERNATIVES } from "@/content/alternatives";
import { buttonVariants } from "@/components/ui/button";
import { BreadcrumbListJsonLd } from "@/components/seo/jsonld";

export const metadata: Metadata = {
  title: "Best status page software in 2026 — ranked + compared",
  description:
    "Six status page tools ranked by price, branding quality, uptime integration, and self-host availability. PingCast leads at $9/mo; Atlassian still strong at the enterprise end.",
  alternates: { canonical: "/best-status-page-software-2026" },
};

type Ranked = {
  rank: number;
  name: string;
  slug: string;
  tagline: string;
  price: string;
  verdict: string;
  href: string;
};

const RANKED: Ranked[] = [
  {
    rank: 1,
    name: "PingCast",
    slug: "pingcast",
    tagline: "Best overall: branded status page + uptime monitoring + MIT self-host, $9/mo.",
    price: "$9/mo (founder), $19/mo retail",
    verdict:
      "Cheapest real-featured option. Incident updates with state timeline, custom domain, Atlassian importer, SVG badge, JS widget — all in one. Open source under MIT.",
    href: "/register?intent=pro",
  },
  {
    rank: 2,
    name: "Atlassian Statuspage",
    slug: "atlassian-statuspage",
    tagline: "Enterprise default — comprehensive, expensive, not sold in Russia.",
    price: "$29–$1499/mo",
    verdict:
      "The incumbent. Deep SLA reports, subscriber audiences, built-in Jira workflows. Overkill for most indie SaaS and 3x the price of a PingCast that matches on every customer-facing feature.",
    href: "/alternatives/atlassian-statuspage",
  },
  {
    rank: 3,
    name: "Instatus",
    slug: "instatus",
    tagline: "Flashy UI, no monitoring, Pro tier 2.2x the price.",
    price: "$20/mo Pro, $80/mo Business",
    verdict:
      "Lovely animations and design polish. Loses on the feature count: no uptime monitoring, no Atlassian import, no embeddable JS widget.",
    href: "/alternatives/instatus",
  },
  {
    rank: 4,
    name: "Openstatus",
    slug: "openstatus",
    tagline: "Open-source peer — AGPL license, $30/mo hosted.",
    price: "Self-host free; $30/mo hosted",
    verdict:
      "Great OSS work. AGPL complicates commercial self-host; hosted is the most expensive of the real contenders.",
    href: "/alternatives/openstatus",
  },
  {
    rank: 5,
    name: "Uptime Kuma",
    slug: "uptime-kuma",
    tagline: "Self-host-only, clunky public page, requires Docker skills.",
    price: "Self-host only",
    verdict:
      "Dominates the free-self-host niche. The public status page is functional but nothing you'd put in front of enterprise customers. No hosted tier — if you want someone else to run it, PingCast is the closest spiritual sibling.",
    href: "/alternatives/uptime-kuma",
  },
  {
    rank: 6,
    name: "UptimeRobot",
    slug: "uptimerobot",
    tagline: "Monitoring-first, not really a status page tool.",
    price: "$7/mo Pro",
    verdict:
      "Generous free tier (50 monitors), but the status page is a dashboard, not a branded customer touchpoint. Keep for internal alerts; pair with PingCast for the public page.",
    href: "/alternatives/uptimerobot",
  },
];

export default function BestStatusPageSoftwarePage() {
  return (
    <div className="container mx-auto px-4 py-12 max-w-4xl">
      <BreadcrumbListJsonLd
        items={[
          { name: "Home", url: "/" },
          { name: "Best status page software 2026", url: "/best-status-page-software-2026" },
        ]}
      />
      <h1 className="text-4xl md:text-5xl font-bold tracking-tight leading-tight">
        Best status page software in 2026
      </h1>
      <p className="mt-4 text-lg text-muted-foreground leading-relaxed">
        Ranked by what indie SaaS and mid-market actually need: branded UI,
        real incident updates, fair pricing, and an escape hatch. Six tools,
        one recommendation per budget bracket.
      </p>

      <div className="mt-12 space-y-6">
        {RANKED.map((r) => {
          const alt = ALTERNATIVES[r.slug];
          return (
            <article
              key={r.slug}
              className={`rounded-xl border p-6 ${
                r.rank === 1
                  ? "border-primary/40 bg-card ring-1 ring-primary/20"
                  : "border-border/60 bg-card"
              }`}
            >
              <div className="flex items-start gap-4">
                <div className="flex-shrink-0 h-10 w-10 rounded-full bg-primary/10 text-primary font-bold flex items-center justify-center">
                  {r.rank}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-baseline gap-3 flex-wrap">
                    <h2 className="text-xl font-semibold">{r.name}</h2>
                    <span className="text-xs uppercase tracking-wider text-muted-foreground">{r.price}</span>
                  </div>
                  <p className="mt-1 text-sm font-medium text-muted-foreground">{r.tagline}</p>
                  <p className="mt-3 text-sm text-muted-foreground leading-relaxed">{r.verdict}</p>
                  <div className="mt-4 flex gap-3 flex-wrap text-sm">
                    <Link href={r.href} className="text-primary underline underline-offset-4 hover:text-foreground">
                      {r.rank === 1 ? "Start PingCast →" : `Full comparison →`}
                    </Link>
                    {alt ? (
                      <a
                        href={alt.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-muted-foreground underline underline-offset-4 hover:text-foreground"
                      >
                        Visit {r.name} ↗
                      </a>
                    ) : null}
                  </div>
                </div>
              </div>
            </article>
          );
        })}
      </div>

      <div className="mt-12 rounded-2xl border border-border/60 bg-card p-8 text-center">
        <h2 className="text-2xl font-bold tracking-tight">Cut the research</h2>
        <p className="mt-3 text-sm text-muted-foreground max-w-xl mx-auto">
          You can spend a week comparing, or spin PingCast up in 60 seconds.
          If it&apos;s not right, self-host on your own infra for free or
          cancel — there&apos;s no annual lock-in.
        </p>
        <Link href="/register?intent=pro" className={`${buttonVariants({ size: "lg" })} mt-5`}>
          Start PingCast Pro <ArrowRight className="ml-2 h-4 w-4" />
        </Link>
      </div>
    </div>
  );
}

import type { Metadata } from "next";
import Link from "next/link";
import { Check, Server, Zap, Sparkles } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { FounderSeatsCounter } from "@/components/features/billing/founder-seats-counter";
import { UpgradeButton } from "@/components/features/billing/upgrade-button";

export const metadata: Metadata = {
  title: "Pricing",
  description:
    "PingCast Pro: branded status pages for SaaS at $9/mo founder's price (first 100 customers). Free tier with 5 monitors. Self-host under MIT.",
};

export default function PricingPage() {
  return (
    <div className="container mx-auto px-4 py-16 max-w-6xl">
      <div className="text-center mb-12">
        <h1 className="text-4xl md:text-5xl font-bold tracking-tight">
          Pricing
        </h1>
        <p className="mt-4 text-muted-foreground max-w-xl mx-auto">
          One open-source project, three ways to use it. Pro is how we pay for
          the hosted infra; free and self-host stay free forever.
        </p>
      </div>

      <div className="grid gap-6 md:grid-cols-3 items-stretch">
        <Plan
          icon={<Zap className="h-5 w-5" />}
          name="Free"
          price="$0"
          priceHint="/ forever"
          cta="Start monitoring"
          href="/register"
          features={[
            "5 HTTP / TCP / DNS monitors",
            "1-minute check interval",
            "Telegram, email, webhook alerts",
            "Public status page (with PingCast watermark)",
            "30 days of incident history",
            "Scoped REST API keys",
          ]}
          footnote="No credit card. No trial countdown. If you outgrow the free tier, self-host for free or upgrade to Pro."
        />

        <PlanPro />

        <Plan
          icon={<Server className="h-5 w-5" />}
          name="Self-hosted"
          price="Free"
          priceHint="on your infra"
          cta="See the repo"
          href="https://github.com/kirillinakin/pingcast"
          external
          features={[
            "Unlimited monitors, channels, keys",
            "No check-interval ceiling",
            "One docker-compose file",
            "Your data never leaves your network",
            "MIT license, upgrade on your schedule",
            "All Pro features unlocked",
          ]}
          footnote="Typical deploy: 4 containers (~150 MB), Postgres + Redis + NATS. One Traefik label for TLS. Instructions in the README."
        />
      </div>

      <div className="mt-16 rounded-xl border border-border/60 bg-card p-8 text-center">
        <h2 className="text-xl font-semibold">Running 100+ monitors?</h2>
        <p className="mt-2 text-sm text-muted-foreground max-w-xl mx-auto">
          Pro covers side-projects and small SaaS. If you need sub-30-second
          checks, a dedicated region, or white-label status pages, open an
          issue and we&apos;ll figure it out.
        </p>
        <Link
          href="https://github.com/kirillinakin/pingcast/issues/new"
          className={`${buttonVariants({ variant: "outline" })} mt-5`}
        >
          Open an issue
        </Link>
      </div>
    </div>
  );
}

function PlanPro() {
  // Pro column lives in its own component so the founder-seats live
  // counter (client-only) is scoped inside a server-rendered card.
  return (
    <div className="flex flex-col rounded-2xl border-2 border-primary/40 bg-card ring-1 ring-primary/20 p-8 relative">
      <div className="absolute -top-3 left-1/2 -translate-x-1/2">
        <FounderSeatsCounter />
      </div>

      <div className="flex items-center gap-3">
        <div className="inline-flex h-9 w-9 items-center justify-center rounded-md bg-primary/10 text-primary">
          <Sparkles className="h-5 w-5" />
        </div>
        <h3 className="font-semibold">Pro</h3>
      </div>

      <div className="mt-6 flex items-baseline gap-2">
        <span className="text-4xl font-bold tracking-tight">$9</span>
        <span className="text-sm text-muted-foreground">
          / mo · founder&apos;s price
        </span>
      </div>
      <p className="text-xs text-muted-foreground mt-1">
        $9 locked for the first 100 customers, then $19/mo retail.
      </p>

      <ul className="mt-6 space-y-2.5 text-sm">
        {[
          "50 monitors · 30s interval",
          "Custom domain: status.yourcompany.com",
          "Branded status page (logo, accent colour, no watermark)",
          "Incident updates with state timeline",
          "Email subscriptions for your customers",
          "Atlassian Statuspage importer (1-click)",
          "SVG status badge for READMEs",
          "Embeddable JS incident widget",
          "SSL expiry warnings (T-14d / T-7d / T-1d)",
          "1 year of incident history + CSV export",
          "Maintenance windows",
          "Priority email support",
        ].map((f) => (
          <li key={f} className="flex gap-2 items-start">
            <Check className="h-4 w-4 text-primary shrink-0 mt-0.5" />
            <span>{f}</span>
          </li>
        ))}
      </ul>

      <p className="mt-6 text-xs text-muted-foreground leading-relaxed">
        Cancel anytime. Price-locked for the lifetime of your subscription —
        even after we raise retail.
      </p>

      <UpgradeButton className="mt-6 w-full" size="default" />
    </div>
  );
}

function Plan({
  icon,
  name,
  price,
  priceHint,
  features,
  cta,
  href,
  footnote,
  external,
}: {
  icon: React.ReactNode;
  name: string;
  price: string;
  priceHint: string;
  features: string[];
  cta: string;
  href: string;
  footnote: string;
  external?: boolean;
}) {
  return (
    <div className="flex flex-col rounded-2xl border border-border/60 bg-card p-8">
      <div className="flex items-center gap-3">
        <div className="inline-flex h-9 w-9 items-center justify-center rounded-md bg-primary/10 text-primary">
          {icon}
        </div>
        <h3 className="font-semibold">{name}</h3>
      </div>
      <div className="mt-6 flex items-baseline gap-2">
        <span className="text-4xl font-bold tracking-tight">{price}</span>
        <span className="text-sm text-muted-foreground">{priceHint}</span>
      </div>
      <ul className="mt-6 space-y-2.5 text-sm">
        {features.map((f) => (
          <li key={f} className="flex gap-2 items-start">
            <Check className="h-4 w-4 text-primary shrink-0 mt-0.5" />
            <span>{f}</span>
          </li>
        ))}
      </ul>
      <p className="mt-6 text-xs text-muted-foreground leading-relaxed">
        {footnote}
      </p>
      <Link
        href={href}
        className={`${buttonVariants({ variant: "outline" })} mt-6`}
        {...(external ? { target: "_blank", rel: "noopener noreferrer" } : {})}
      >
        {cta}
      </Link>
    </div>
  );
}

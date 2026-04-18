import type { Metadata } from "next";
import Link from "next/link";
import { Check, Server, Zap } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";

export const metadata: Metadata = {
  title: "Pricing",
  description:
    "PingCast is free to use and MIT-licensed to self-host. Hosted plan limits and self-host setup on one page.",
};

// Intentionally honest: PingCast is early and free. A Pro tier will
// show up here when it actually exists — I'd rather ship a real
// /pricing page with the current state than stage a fake upgrade CTA.
export default function PricingPage() {
  return (
    <div className="container mx-auto px-4 py-16 max-w-5xl">
      <div className="text-center mb-12">
        <h1 className="text-4xl md:text-5xl font-bold tracking-tight">
          Pricing
        </h1>
        <p className="mt-4 text-muted-foreground max-w-xl mx-auto">
          One free tier on the hosted version, one MIT license on your own
          hardware. No usage-based surprises, no upsell to SMS packs.
        </p>
      </div>

      <div className="grid gap-6 md:grid-cols-2">
        <Plan
          icon={<Zap className="h-5 w-5" />}
          name="Hosted · Free"
          price="$0"
          priceHint="/ forever"
          cta="Start monitoring"
          href="/register"
          primary
          features={[
            "5 HTTP/TCP/DNS monitors",
            "1-minute check interval",
            "Telegram, email, and webhook alerts",
            "Unlimited public status pages",
            "Scoped REST API keys",
            "30-day incident history",
          ]}
          footnote="No credit card, no 14-day trial countdown. If you outgrow the free tier, self-host."
        />

        <Plan
          icon={<Server className="h-5 w-5" />}
          name="Self-hosted · MIT"
          price="Free"
          priceHint="on your infra"
          cta="See the repo"
          href="https://github.com/kirillinakin/pingcast"
          external
          features={[
            "Unlimited monitors, channels, and keys",
            "No check-interval ceiling",
            "Runs on one docker-compose file",
            "Postgres + Redis + NATS JetStream",
            "Your data never leaves your network",
            "Upgrade on your own schedule",
          ]}
          footnote="Typical deploy: 4 containers, ~150 MB total, one Traefik label for TLS. Instructions in the README."
        />
      </div>

      <div className="mt-16 rounded-xl border border-border/60 bg-card p-8 text-center">
        <h2 className="text-xl font-semibold">Running high volume?</h2>
        <p className="mt-2 text-sm text-muted-foreground max-w-xl mx-auto">
          The hosted tier is tuned for side-projects and small SaaS. If you
          need 100+ monitors, sub-30-second checks, or a dedicated instance,
          open an issue and we&apos;ll figure it out.
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

function Plan({
  icon,
  name,
  price,
  priceHint,
  features,
  cta,
  href,
  footnote,
  primary,
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
  primary?: boolean;
  external?: boolean;
}) {
  return (
    <div
      className={`flex flex-col rounded-2xl border p-8 ${
        primary
          ? "border-primary/40 bg-card ring-1 ring-primary/20"
          : "border-border/60 bg-card"
      }`}
    >
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
        className={`${buttonVariants({
          variant: primary ? "default" : "outline",
        })} mt-6`}
        {...(external ? { target: "_blank", rel: "noopener noreferrer" } : {})}
      >
        {cta}
      </Link>
    </div>
  );
}

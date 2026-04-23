import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { BreadcrumbListJsonLd } from "@/components/seo/jsonld";

export const metadata: Metadata = {
  title: "Open-source status page — PingCast (MIT-licensed, self-host)",
  description:
    "PingCast is an open-source status page + uptime monitor under the MIT license. Self-host via docker-compose in one command, or use the hosted tier at $9/mo.",
  alternates: { canonical: "/open-source-status-page" },
};

export default function OpenSourceStatusPagePage() {
  return (
    <div className="container mx-auto px-4 py-12 max-w-3xl">
      <BreadcrumbListJsonLd
        items={[
          { name: "Home", url: "/" },
          { name: "Open-source status page", url: "/open-source-status-page" },
        ]}
      />
      <h1 className="text-4xl md:text-5xl font-bold tracking-tight leading-tight">
        An open-source status page you can actually trust
      </h1>
      <p className="mt-4 text-lg text-muted-foreground leading-relaxed">
        PingCast is MIT-licensed on GitHub and ships with a hosted SaaS
        tier. Same codebase. Same feature set. Either run it yourself or
        let us run it — the commercial escape hatch most indie SaaS
        founders actually want.
      </p>

      <div className="mt-8 flex flex-wrap gap-3">
        <a
          href="https://github.com/kirillinakin/pingcast"
          target="_blank"
          rel="noopener noreferrer"
          className={buttonVariants({ size: "lg" })}
        >
          View on GitHub <ArrowRight className="ml-2 h-4 w-4" />
        </a>
        <Link
          href="/register?intent=pro"
          className={buttonVariants({ variant: "outline", size: "lg" })}
        >
          Start hosted from $9/mo
        </Link>
      </div>

      <section className="mt-12">
        <h2 className="text-2xl font-bold tracking-tight">Why MIT, not AGPL</h2>
        <p className="mt-3 text-muted-foreground leading-relaxed">
          Openstatus is AGPL. Uptime Kuma is MIT. Cachet is BSD-3. Licensing
          matters because it dictates whether you can self-host inside a
          commercial product without publishing every modification.
          PingCast picks MIT — fork, patch, embed, redistribute, whatever.
          Your legal team&apos;s easiest call.
        </p>
      </section>

      <section className="mt-12">
        <h2 className="text-2xl font-bold tracking-tight">Self-host in one command</h2>
        <pre className="mt-4 overflow-x-auto rounded-lg border border-border/60 bg-card p-4 text-sm font-mono">
          <code>{`git clone https://github.com/kirillinakin/pingcast
cd pingcast && docker compose up -d
# → http://localhost:3001`}</code>
        </pre>
        <p className="mt-3 text-sm text-muted-foreground leading-relaxed">
          Four containers (~150 MB total): Go API, Next.js web, Postgres,
          Redis + NATS. Add one Traefik label for TLS and you&apos;re in
          production.
        </p>
      </section>

      <section className="mt-12">
        <h2 className="text-2xl font-bold tracking-tight">What you get either way</h2>
        <ul className="mt-4 grid md:grid-cols-2 gap-3 text-sm">
          {[
            "HTTP / TCP / DNS uptime monitoring",
            "Branded status pages (custom domain in hosted)",
            "Incident updates with state timeline",
            "Email subscribers with double opt-in",
            "Atlassian Statuspage 1-click importer",
            "SVG status badge for READMEs",
            "Embeddable JS incident widget",
            "SSL expiry warnings (14/7/1 days)",
            "Telegram / email / webhook alerts",
            "Typed REST API (OpenAPI spec)",
          ].map((f) => (
            <li key={f} className="flex gap-2 items-start">
              <span className="text-primary mt-1">✓</span>
              <span className="text-muted-foreground">{f}</span>
            </li>
          ))}
        </ul>
      </section>

      <div className="mt-16 rounded-2xl border border-border/60 bg-card p-8 text-center">
        <h2 className="text-2xl font-bold tracking-tight">Open source, commercial hosted</h2>
        <p className="mt-3 text-sm text-muted-foreground max-w-xl mx-auto">
          Both paths work forever. Start hosted; switch to self-host when you
          outgrow. Or start self-hosted; move to hosted when managing
          containers stops being fun.
        </p>
        <div className="mt-5 flex gap-3 justify-center flex-wrap">
          <a
            href="https://github.com/kirillinakin/pingcast"
            target="_blank"
            rel="noopener noreferrer"
            className={buttonVariants({ size: "lg" })}
          >
            View repo
          </a>
          <Link href="/pricing" className={buttonVariants({ variant: "outline", size: "lg" })}>
            Hosted pricing
          </Link>
        </div>
      </div>
    </div>
  );
}

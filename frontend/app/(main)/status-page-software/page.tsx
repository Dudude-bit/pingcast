import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight, Check } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { BreadcrumbListJsonLd } from "@/components/seo/jsonld";

export const metadata: Metadata = {
  title: "Status page software for SaaS — PingCast ($9/mo)",
  description:
    "Compact guide to picking status page software: what to look for, what to avoid, and why PingCast ships a branded status page + uptime monitoring for $9/mo.",
  alternates: { canonical: "/status-page-software" },
};

export default function StatusPageSoftwarePage() {
  return (
    <div className="container mx-auto px-4 py-12 max-w-3xl">
      <BreadcrumbListJsonLd
        items={[
          { name: "Home", url: "/" },
          { name: "Status page software", url: "/status-page-software" },
        ]}
      />
      <h1 className="text-4xl md:text-5xl font-bold tracking-tight leading-tight">
        Status page software, minus the $29 entry fee.
      </h1>
      <p className="mt-4 text-lg text-muted-foreground leading-relaxed">
        A public status page is the cheapest trust-building tool a SaaS can
        ship. It tells your customers you know when something&apos;s broken,
        you&apos;re on it, and you&apos;ll tell them when it&apos;s fixed. This
        page is the short version of how to pick one.
      </p>

      <div className="mt-8 flex flex-wrap gap-3">
        <Link href="/register?intent=pro" className={buttonVariants({ size: "lg" })}>
          Start PingCast Pro <ArrowRight className="ml-2 h-4 w-4" />
        </Link>
        <Link href="/pricing" className={buttonVariants({ variant: "outline", size: "lg" })}>
          See pricing
        </Link>
      </div>

      <section className="mt-12">
        <h2 className="text-2xl font-bold tracking-tight">
          What good status page software actually ships
        </h2>
        <p className="mt-3 text-muted-foreground leading-relaxed">
          Ignore pixel animations. The five things that actually matter:
        </p>
        <ul className="mt-4 space-y-3">
          {[
            [
              "Custom domain",
              "status.yourcompany.com via CNAME, with a TLS cert we issue automatically. Without this the status page feels like a third-party tool.",
            ],
            [
              "Branded UI",
              "Your logo, your accent colour, your footer. No vendor watermark. Customers should feel they&apos;re still on your site.",
            ],
            [
              "Incident updates with a state timeline",
              "Investigating → identified → monitoring → resolved, with prose body on each step. Flat lists of `monitor down` / `monitor up` are not status pages.",
            ],
            [
              "Email subscribers (double opt-in)",
              "Customers opt in once, get notified on every state change, unsubscribe in one click. CAN-SPAM / 152-ФЗ want a real double opt-in, not a checkbox.",
            ],
            [
              "Embeddable signals",
              "SVG badge for your README + a JS widget that pops an in-site banner when an incident is open. Free viral distribution.",
            ],
          ].map(([title, body]) => (
            <li key={title} className="flex gap-3">
              <Check className="h-5 w-5 text-primary shrink-0 mt-0.5" />
              <div>
                <strong>{title}</strong>
                <p className="text-sm text-muted-foreground mt-0.5" dangerouslySetInnerHTML={{ __html: body }} />
              </div>
            </li>
          ))}
        </ul>
      </section>

      <section className="mt-12">
        <h2 className="text-2xl font-bold tracking-tight">The price floor matters</h2>
        <p className="mt-3 text-muted-foreground leading-relaxed">
          Status page software is a fixed-cost tool you pay for forever. Every
          $20 a month is $240/year, $1,200 over five years. Atlassian
          Statuspage starts at $29/mo; Instatus and Statuspal land at $20-46.
          PingCast opens at $9/mo (founder&apos;s price for the first 100
          customers, $19/mo retail after), and self-hosting is free under MIT
          if you ever want out of the hosted flow.
        </p>
        <div className="mt-6">
          <Link
            href="/alternatives/atlassian-statuspage"
            className="text-sm underline underline-offset-4 hover:text-foreground"
          >
            Full comparison vs Atlassian Statuspage →
          </Link>
        </div>
      </section>

      <section className="mt-12">
        <h2 className="text-2xl font-bold tracking-tight">Skip the ones that aren&apos;t status pages</h2>
        <p className="mt-3 text-muted-foreground leading-relaxed">
          A generic dashboard showing &ldquo;service up&rdquo; / &ldquo;service
          down&rdquo; pixels isn&apos;t a status page. UptimeRobot, for
          instance, ships a public dashboard — fine for internal visibility,
          but there&apos;s no incident narrative, no subscribers, no custom
          domain. Check for the five features above before committing.
        </p>
      </section>

      <div className="mt-12 rounded-2xl border border-border/60 bg-card p-8 text-center">
        <h2 className="text-2xl font-bold tracking-tight">
          Spin up a status page in 60 seconds
        </h2>
        <p className="mt-3 text-sm text-muted-foreground max-w-xl mx-auto">
          Free tier works on <code>pingcast.io/status/your-slug</code>. Upgrade
          to Pro and point <code>status.yourcompany.com</code> at us.
        </p>
        <Link href="/register?intent=pro" className={`${buttonVariants({ size: "lg" })} mt-5`}>
          Get started
        </Link>
      </div>
    </div>
  );
}

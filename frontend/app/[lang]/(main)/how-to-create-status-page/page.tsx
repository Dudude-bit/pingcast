import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { BreadcrumbListJsonLd } from "@/components/seo/jsonld";

export const metadata: Metadata = {
  title: "How to create a status page for your SaaS (complete guide)",
  description:
    "Step-by-step guide to shipping a branded status page for your SaaS — from domain setup to incident workflows. Uses PingCast as the example, transferable to any tool.",
  alternates: { canonical: "/how-to-create-status-page" },
};

export default function HowToCreateStatusPagePage() {
  return (
    <div className="container mx-auto px-4 py-12 max-w-3xl">
      <BreadcrumbListJsonLd
        items={[
          { name: "Home", url: "/" },
          { name: "How to create a status page", url: "/how-to-create-status-page" },
        ]}
      />
      <h1 className="text-4xl md:text-5xl font-bold tracking-tight leading-tight">
        How to create a status page for your SaaS
      </h1>
      <p className="mt-4 text-lg text-muted-foreground leading-relaxed">
        A status page done right takes under an hour of setup and pays back
        every time something breaks. Here&apos;s the full sequence, example
        tool agnostic (uses PingCast to make it concrete; translates 1:1 to
        Atlassian, Instatus, Openstatus).
      </p>

      <ol className="mt-12 space-y-10">
        <Step n={1} title="Pick the URL your customers will remember">
          <p>
            Use <code>status.yourcompany.com</code>. It signals ownership,
            avoids looking like a third-party tool, and Google indexes it as
            part of your domain. If you don&apos;t have a root domain budget,
            fall back to a path on your apex (<code>yourcompany.com/status</code>)
            but avoid subdomains on the vendor (<code>yourcompany.pingcast.io</code>{" "}
            works, but loses the trust-signal).
          </p>
        </Step>

        <Step n={2} title="Decide what goes on the page">
          <p>
            Aim for 3–8 monitored surfaces. More than that becomes a wall of
            green dots that people stop reading. Common groupings:
          </p>
          <ul className="list-disc pl-6 mt-3 space-y-1 text-sm text-muted-foreground">
            <li>Public-facing app (www)</li>
            <li>Customer API (api.)</li>
            <li>Admin / dashboard</li>
            <li>Checkout + billing</li>
            <li>Third-party dependencies you want to surface (Stripe, DB provider)</li>
          </ul>
        </Step>

        <Step n={3} title="Configure the monitors">
          <p>
            HTTP with a 30-60s interval is right for nearly everything. Set{" "}
            <code>alert_after_failures: 2</code> — a single flaky check from a
            transient DNS hiccup shouldn&apos;t page you. For customer-facing
            APIs, check a representative endpoint that exercises DB, cache,
            and auth in one call (not just <code>/health</code> — that tells
            you nothing).
          </p>
        </Step>

        <Step n={4} title="Write your incident-update templates now, not at 3 AM">
          <p>
            Draft canned phrasing for each state transition while you&apos;re
            calm. Four templates cover most incidents:
          </p>
          <ul className="list-disc pl-6 mt-3 space-y-2 text-sm text-muted-foreground">
            <li>
              <strong>investigating</strong>: &ldquo;We&apos;re aware of
              elevated error rates on &lt;surface&gt;. Investigating now.&rdquo;
            </li>
            <li>
              <strong>identified</strong>: &ldquo;Cause identified:
              &lt;plain-English cause&gt;. Mitigation in progress.&rdquo;
            </li>
            <li>
              <strong>monitoring</strong>: &ldquo;Fix deployed. Watching for
              recovery; will confirm in 10-15 minutes.&rdquo;
            </li>
            <li>
              <strong>resolved</strong>: &ldquo;All services recovered. Total
              incident duration: &lt;mm&gt; minutes. Post-mortem: &lt;link or
              TBD&gt;.&rdquo;
            </li>
          </ul>
        </Step>

        <Step n={5} title="Enable email subscribers">
          <p>
            Double opt-in is the only legal form of this (GDPR, CAN-SPAM,
            152-ФЗ). The subscribe box goes on your public status page;
            subscribers confirm via email and get notified on every state
            change. Unsubscribe link must be one click in every outbound
            email — no confirmation modals.
          </p>
        </Step>

        <Step n={6} title="Drop the badge and widget into your product">
          <p>
            An SVG status badge in your README signals &ldquo;this product has
            its shit together&rdquo; to anyone looking at your GitHub. A JS
            widget (<code>{`<script src="/widget.js" data-slug="...">`}</code>)
            pops an in-product banner during active incidents so customers
            don&apos;t have to navigate to the status page to find out.
          </p>
        </Step>

        <Step n={7} title="Test it before you need it">
          <p>
            Once, manually post a fake incident in working hours, watch every
            channel fire (email, Telegram, widget banner), then resolve it.
            You want to know the notification path works <em>before</em>{" "}
            production breaks at 3 AM.
          </p>
        </Step>
      </ol>

      <div className="mt-16 rounded-2xl border border-border/60 bg-card p-8 text-center">
        <h2 className="text-2xl font-bold tracking-tight">
          Want a status page in 60 seconds?
        </h2>
        <p className="mt-3 text-sm text-muted-foreground max-w-xl mx-auto">
          PingCast is $9/mo (founder&apos;s price for the first 100 customers)
          and ships every step above — domain, subscribers, templates, badge,
          widget — out of the box.
        </p>
        <Link
          href="/register?intent=pro"
          className={`${buttonVariants({ size: "lg" })} mt-5`}
        >
          Start PingCast Pro <ArrowRight className="ml-2 h-4 w-4" />
        </Link>
      </div>
    </div>
  );
}

function Step({ n, title, children }: { n: number; title: string; children: React.ReactNode }) {
  return (
    <li className="flex gap-5">
      <span className="flex-shrink-0 h-10 w-10 rounded-full bg-primary/10 text-primary font-bold text-lg flex items-center justify-center">
        {n}
      </span>
      <div className="flex-1 pt-1">
        <h3 className="text-xl font-semibold">{title}</h3>
        <div className="mt-2 text-muted-foreground leading-relaxed">{children}</div>
      </div>
    </li>
  );
}

"use client";

import Link from "next/link";
import { motion } from "framer-motion";
import { Zap, Bell, LineChart, ArrowRight, Code2, Terminal } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { LandingDemo } from "@/components/site/landing-demo";

const jsonLd = {
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  name: "PingCast",
  applicationCategory: "DeveloperApplication",
  operatingSystem: "Web",
  description:
    "Lightweight uptime monitoring with instant Telegram alerts and public status pages.",
  offers: {
    "@type": "Offer",
    price: "0",
    priceCurrency: "USD",
  },
  featureList: [
    "HTTP uptime checks",
    "Telegram alerts",
    "Public status pages",
    "REST API with scoped keys",
  ],
};

export default function LandingPage() {
  return (
    <div className="container mx-auto px-4">
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
      />
      <section className="py-20 md:py-28 max-w-4xl mx-auto text-center">
        <motion.div
          initial={{ opacity: 0, y: 12 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, ease: "easeOut" }}
          className="inline-flex items-center gap-2 rounded-full border border-border/60 bg-card px-3 py-1 text-xs text-muted-foreground"
        >
          <span className="inline-block h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse" />
          Live now · 5 monitors free
        </motion.div>

        <motion.h1
          initial={{ opacity: 0, y: 16 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.1, duration: 0.6, ease: "easeOut" }}
          className="mt-6 text-4xl md:text-6xl font-bold tracking-tight leading-[1.1]"
        >
          Know when it breaks.
          <br />
          <span className="bg-gradient-to-r from-blue-600 via-cyan-500 to-teal-500 bg-clip-text text-transparent">
            Before your users do.
          </span>
        </motion.h1>

        <motion.p
          initial={{ opacity: 0, y: 16 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.2, duration: 0.6, ease: "easeOut" }}
          className="mt-6 text-lg md:text-xl text-muted-foreground max-w-2xl mx-auto"
        >
          Lightweight uptime monitoring with instant Telegram alerts and public
          status pages. Built for developers who ship fast.
        </motion.p>

        <motion.div
          initial={{ opacity: 0, y: 16 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.3, duration: 0.6, ease: "easeOut" }}
          className="mt-10 flex flex-col sm:flex-row items-center justify-center gap-4"
        >
          <Link href="/register" className={buttonVariants({ size: "lg" })}>
            Start monitoring
            <ArrowRight className="ml-2 h-4 w-4" />
          </Link>
          <p className="text-sm text-muted-foreground">
            No credit card · 30-second checks · unlimited status pages
          </p>
        </motion.div>
      </section>

      <section className="pb-16">
        <LandingDemo />
      </section>

      <section className="py-16 grid gap-6 md:grid-cols-3 max-w-5xl mx-auto">
        <FeatureCard
          icon={<Zap className="h-6 w-6" />}
          title="30-second checks"
          body="HTTP, TCP, and DNS checks with keyword matching, status-code validation, and TLS 1.2+ verification."
        />
        <FeatureCard
          icon={<Bell className="h-6 w-6" />}
          title="Instant alerts"
          body="Telegram, email, and webhook destinations. Configurable failure thresholds to filter false positives."
        />
        <FeatureCard
          icon={<LineChart className="h-6 w-6" />}
          title="Public status pages"
          body="SSR + ISR status pages for your customers. Show uptime, incidents, build trust with transparency."
        />
      </section>

      <section className="py-16 max-w-4xl mx-auto">
        <motion.div
          initial={{ opacity: 0, y: 12 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true, margin: "-80px" }}
          transition={{ duration: 0.5, ease: "easeOut" }}
          className="rounded-2xl border border-border/60 bg-card overflow-hidden"
        >
          <div className="flex items-center gap-2 border-b border-border/60 bg-muted/40 px-4 py-2.5 text-xs font-mono text-muted-foreground">
            <Terminal className="h-3.5 w-3.5" />
            <span>bash — 80x24</span>
            <span className="ml-auto flex gap-1.5">
              <span className="h-2 w-2 rounded-full bg-red-400/80" />
              <span className="h-2 w-2 rounded-full bg-amber-400/80" />
              <span className="h-2 w-2 rounded-full bg-emerald-400/80" />
            </span>
          </div>
          <pre className="overflow-x-auto px-6 py-5 text-[13px] leading-relaxed font-mono">
            <code>
              <span className="text-muted-foreground"># Create a monitor from CI after every deploy</span>
              {"\n"}
              <span className="text-emerald-600 dark:text-emerald-400">curl</span>{" "}
              <span className="text-blue-600 dark:text-blue-400">-X</span> POST https://pingcast.io/api/monitors{" "}
              {"\\\n  "}
              <span className="text-blue-600 dark:text-blue-400">-H</span>{" "}
              <span className="text-amber-600 dark:text-amber-400">{`"Authorization: Bearer $PINGCAST_KEY"`}</span>{" "}
              {"\\\n  "}
              <span className="text-blue-600 dark:text-blue-400">-H</span>{" "}
              <span className="text-amber-600 dark:text-amber-400">{`"Content-Type: application/json"`}</span>{" "}
              {"\\\n  "}
              <span className="text-blue-600 dark:text-blue-400">-d</span>{" "}
              <span className="text-amber-600 dark:text-amber-400">{`'{"name": "api prod", "type": "http",`}</span>
              {"\n       "}
              <span className="text-amber-600 dark:text-amber-400">{`"config": {"url": "https://api.example.com/health"},`}</span>
              {"\n       "}
              <span className="text-amber-600 dark:text-amber-400">{`"interval_seconds": 60}'`}</span>
            </code>
          </pre>
        </motion.div>
        <p className="mt-4 text-center text-sm text-muted-foreground">
          Scoped API keys · Typed OpenAPI spec ·{" "}
          <Link href="/docs/api" className="underline underline-offset-4 hover:text-foreground">
            Full reference
          </Link>
        </p>
      </section>

      <section className="py-16 max-w-3xl mx-auto">
        <h2 className="text-center text-2xl md:text-3xl font-bold tracking-tight mb-10">
          Frequently asked
        </h2>
        <div className="space-y-3">
          <FAQItem
            q="Is there a free tier?"
            a="Yes. 5 monitors, 1-minute checks, unlimited status pages, and Telegram + email + webhook notifications — all free, no credit card."
          />
          <FAQItem
            q="How quickly do alerts fire?"
            a="Checks run at your configured interval (down to 1 minute). A monitor is only marked down after the configured consecutive-failure threshold, so a single flaky check won't page you."
          />
          <FAQItem
            q="Can I embed my status page?"
            a="Every monitor you mark public appears on /status/your-slug. The page is SSR + ISR with a 30-second revalidate — share the URL anywhere, embed it in an iframe, or point your own subdomain at it."
          />
          <FAQItem
            q="What happens if PingCast itself goes down?"
            a="The checker is a separate service from the API and dashboard. Alerts keep firing even if the dashboard is unreachable. For full independence, self-host — the stack is a single docker-compose file."
          />
          <FAQItem
            q="Is the data portable?"
            a="Yes. Every field exposed in the dashboard is available over the REST API, and the database is standard Postgres. You can self-host the whole stack or export whenever you want."
          />
        </div>
      </section>

      <section className="py-16 max-w-4xl mx-auto">
        <div className="rounded-2xl border border-border/60 bg-card p-8 md:p-12 text-center">
          <div className="inline-flex h-10 w-10 items-center justify-center rounded-md bg-primary/10 text-primary mb-4">
            <Code2 className="h-5 w-5" />
          </div>
          <h2 className="text-2xl md:text-3xl font-bold tracking-tight">
            Built on a real API, not a marketing website.
          </h2>
          <p className="mt-3 text-sm md:text-base text-muted-foreground max-w-xl mx-auto">
            Every feature you see in the dashboard is available via a stable
            JSON API with scoped keys. Integrate pingcast into your tools,
            CI/CD, and runbooks.
          </p>
          <Link
            href="/register"
            className={`${buttonVariants({ variant: "outline" })} mt-6`}
          >
            Get your API key
          </Link>
        </div>
      </section>
    </div>
  );
}

function FAQItem({ q, a }: { q: string; a: string }) {
  return (
    <motion.details
      initial={{ opacity: 0, y: 6 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, margin: "-40px" }}
      transition={{ duration: 0.35, ease: "easeOut" }}
      className="group rounded-lg border border-border/60 bg-card px-5 py-4 [&[open]_svg]:rotate-90"
    >
      <summary className="flex cursor-pointer list-none items-center justify-between gap-4 font-medium">
        {q}
        <ArrowRight className="h-4 w-4 shrink-0 text-muted-foreground transition-transform" />
      </summary>
      <p className="mt-3 text-sm text-muted-foreground leading-relaxed">{a}</p>
    </motion.details>
  );
}

function FeatureCard({
  icon,
  title,
  body,
}: {
  icon: React.ReactNode;
  title: string;
  body: string;
}) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 12 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, margin: "-50px" }}
      transition={{ duration: 0.5, ease: "easeOut" }}
      className="rounded-lg border border-border/60 bg-card p-6 hover:border-border hover:bg-accent/20 transition-colors"
    >
      <div className="inline-flex h-10 w-10 items-center justify-center rounded-md bg-primary/10 text-primary mb-4">
        {icon}
      </div>
      <h3 className="font-semibold text-lg">{title}</h3>
      <p className="mt-2 text-sm text-muted-foreground leading-relaxed">{body}</p>
    </motion.div>
  );
}

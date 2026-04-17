"use client";

import Link from "next/link";
import { motion } from "framer-motion";
import { Zap, Bell, LineChart, ArrowRight, Code2 } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { LandingDemo } from "@/components/site/landing-demo";

export default function LandingPage() {
  return (
    <div className="container mx-auto px-4">
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

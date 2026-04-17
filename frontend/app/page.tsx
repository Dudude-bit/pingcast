import Link from "next/link";
import { Zap, Bell, LineChart, ArrowRight } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";

export default function LandingPage() {
  return (
    <div className="container mx-auto px-4">
      <section className="py-24 md:py-32 max-w-4xl mx-auto text-center">
        <h1 className="text-4xl md:text-6xl font-bold tracking-tight leading-tight">
          Know when it breaks.
          <br />
          <span className="bg-gradient-to-r from-blue-600 via-cyan-500 to-teal-500 bg-clip-text text-transparent">
            Before your users do.
          </span>
        </h1>
        <p className="mt-6 text-lg md:text-xl text-muted-foreground max-w-2xl mx-auto">
          Lightweight uptime monitoring with instant Telegram alerts and public
          status pages. Built for developers who ship fast.
        </p>
        <div className="mt-10 flex flex-col sm:flex-row items-center justify-center gap-4">
          <Link href="/register" className={buttonVariants({ size: "lg" })}>
            Start monitoring
            <ArrowRight className="ml-2 h-4 w-4" />
          </Link>
          <p className="text-sm text-muted-foreground">
            5 monitors free · No credit card required
          </p>
        </div>
      </section>

      <section className="py-16 grid gap-6 md:grid-cols-3 max-w-5xl mx-auto">
        <FeatureCard
          icon={<Zap className="h-6 w-6" />}
          title="30-Second Checks"
          body="HTTP checks every 30 seconds with keyword matching, status code validation, and TLS verification."
        />
        <FeatureCard
          icon={<Bell className="h-6 w-6" />}
          title="Instant Alerts"
          body="Telegram and email notifications the moment your service goes down. Configurable thresholds to avoid false positives."
        />
        <FeatureCard
          icon={<LineChart className="h-6 w-6" />}
          title="Status Pages"
          body="Public status page for your customers. Show uptime, incidents, and build trust with transparency."
        />
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
    <div className="rounded-lg border border-border/60 bg-card p-6 hover:border-border transition-colors">
      <div className="inline-flex h-10 w-10 items-center justify-center rounded-md bg-primary/10 text-primary mb-4">
        {icon}
      </div>
      <h3 className="font-semibold text-lg">{title}</h3>
      <p className="mt-2 text-sm text-muted-foreground leading-relaxed">
        {body}
      </p>
    </div>
  );
}

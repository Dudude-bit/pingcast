import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { Check, Server, Zap, Sparkles } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { FounderSeatsCounter } from "@/components/features/billing/founder-seats-counter";
import { UpgradeButton } from "@/components/features/billing/upgrade-button";
import { getDictionary, hasLocale } from "@/lib/i18n";

type Params = Promise<{ lang: string }>;

export async function generateMetadata({
  params,
}: {
  params: Params;
}): Promise<Metadata> {
  const { lang } = await params;
  if (!hasLocale(lang)) return {};
  const dict = await getDictionary(lang);
  return {
    title: dict.pricing.page_title,
    description: dict.pricing.page_subtitle,
    alternates: {
      canonical: `/${lang}/pricing`,
      languages: { en: "/en/pricing", ru: "/ru/pricing", "x-default": "/en/pricing" },
    },
  };
}

export default async function PricingPage({ params }: { params: Params }) {
  const { lang } = await params;
  if (!hasLocale(lang)) notFound();
  const dict = await getDictionary(lang);
  const p = dict.pricing;

  return (
    <div className="container mx-auto px-4 py-16 max-w-6xl">
      <div className="text-center mb-12">
        <h1 className="text-4xl md:text-5xl font-bold tracking-tight">
          {p.page_title}
        </h1>
        <p className="mt-4 text-muted-foreground max-w-xl mx-auto">
          {p.page_subtitle}
        </p>
      </div>

      <div className="grid gap-6 md:grid-cols-3 items-stretch">
        <Plan
          icon={<Zap className="h-5 w-5" />}
          name={p.free_title}
          price={p.free_price}
          priceHint={p.free_per}
          subtitle={p.free_subtitle}
          cta={p.free_cta}
          href={`/${lang}/register`}
          features={p.free_features}
        />

        <PlanPro lang={lang} dict={dict} />

        <Plan
          icon={<Server className="h-5 w-5" />}
          name={p.self_host_title}
          price={p.self_host_price}
          priceHint=""
          subtitle={p.self_host_subtitle}
          cta={p.self_host_cta}
          href="https://github.com/kirillinakin/pingcast"
          external
          features={p.self_host_features}
        />
      </div>

      <div className="mt-16 max-w-3xl mx-auto">
        <h2 className="text-center text-2xl font-bold tracking-tight mb-6">
          {p.faq_heading}
        </h2>
        <div className="space-y-3">
          {p.faq.map((q) => (
            <details
              key={q.q}
              className="group rounded-lg border border-border/60 bg-card px-5 py-4"
            >
              <summary className="cursor-pointer font-medium list-none">
                {q.q}
              </summary>
              <p className="mt-3 text-sm text-muted-foreground leading-relaxed">
                {q.a}
              </p>
            </details>
          ))}
        </div>
      </div>
    </div>
  );
}

function PlanPro({
  lang,
  dict,
}: {
  lang: string;
  dict: Awaited<ReturnType<typeof getDictionary>>;
}) {
  const p = dict.pricing;
  return (
    <div className="flex flex-col rounded-2xl border-2 border-primary/40 bg-card ring-1 ring-primary/20 p-8 relative">
      <div className="absolute -top-3 left-1/2 -translate-x-1/2">
        <FounderSeatsCounter />
      </div>

      <div className="flex items-center gap-3">
        <div className="inline-flex h-9 w-9 items-center justify-center rounded-md bg-primary/10 text-primary">
          <Sparkles className="h-5 w-5" />
        </div>
        <h3 className="font-semibold">{p.pro_title}</h3>
      </div>
      <p className="text-sm text-muted-foreground mt-1">{p.pro_subtitle}</p>

      <div className="mt-6 flex items-baseline gap-2">
        <span className="text-4xl font-bold tracking-tight">
          {p.pro_price_founder}
        </span>
        <span className="text-sm text-muted-foreground">{p.pro_per}</span>
        <span className="ml-2 inline-flex items-center rounded-full bg-primary/10 text-primary text-xs px-2 py-0.5">
          {p.pro_badge_founder}
        </span>
      </div>

      <ul className="mt-6 space-y-2.5 text-sm">
        {p.pro_features.map((f) => (
          <li key={f} className="flex gap-2 items-start">
            <Check className="h-4 w-4 text-primary shrink-0 mt-0.5" />
            <span>{f}</span>
          </li>
        ))}
      </ul>

      <UpgradeButton className="mt-6 w-full" size="default" />
      {/* lang prop intentionally unused inside UpgradeButton — checkout
          flow stays English-only on the LemonSqueezy side. */}
      <input type="hidden" value={lang} readOnly />
    </div>
  );
}

function Plan({
  icon,
  name,
  price,
  priceHint,
  subtitle,
  features,
  cta,
  href,
  external,
}: {
  icon: React.ReactNode;
  name: string;
  price: string;
  priceHint: string;
  subtitle?: string;
  features: readonly string[];
  cta: string;
  href: string;
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
      {subtitle && (
        <p className="text-sm text-muted-foreground mt-1">{subtitle}</p>
      )}
      <div className="mt-6 flex items-baseline gap-2">
        <span className="text-4xl font-bold tracking-tight">{price}</span>
        {priceHint && (
          <span className="text-sm text-muted-foreground">{priceHint}</span>
        )}
      </div>
      <ul className="mt-6 space-y-2.5 text-sm">
        {features.map((f) => (
          <li key={f} className="flex gap-2 items-start">
            <Check className="h-4 w-4 text-primary shrink-0 mt-0.5" />
            <span>{f}</span>
          </li>
        ))}
      </ul>
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

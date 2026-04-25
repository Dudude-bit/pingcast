import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ArrowRight } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { BreadcrumbListJsonLd } from "@/components/seo/jsonld";
import { getDictionary, hasLocale, SUPPORTED_LOCALES } from "@/lib/i18n";

type Params = Promise<{ lang: string }>;

export async function generateMetadata({
  params,
}: {
  params: Params;
}): Promise<Metadata> {
  const { lang } = await params;
  if (!hasLocale(lang)) return {};
  const dict = await getDictionary(lang);
  const c = dict.seo_atlassian_pricing;
  return {
    title: c.metaTitle,
    description: c.metaDesc,
    alternates: {
      canonical: `/${lang}/atlassian-statuspage-pricing`,
      languages: Object.fromEntries(
        SUPPORTED_LOCALES.map((l) => [l, `/${l}/atlassian-statuspage-pricing`]),
      ),
    },
  };
}

export default async function AtlassianStatuspagePricingPage({
  params,
}: {
  params: Params;
}) {
  const { lang } = await params;
  if (!hasLocale(lang)) notFound();
  const dict = await getDictionary(lang);
  const c = dict.seo_atlassian_pricing;

  return (
    <div className="container mx-auto px-4 py-12 max-w-4xl">
      <BreadcrumbListJsonLd
        items={[
          { name: dict.alternatives_template.home, url: `/${lang}` },
          { name: c.crumb, url: `/${lang}/atlassian-statuspage-pricing` },
        ]}
      />
      <h1 className="text-4xl md:text-5xl font-bold tracking-tight leading-tight">
        {c.h1}
      </h1>
      <p className="mt-4 text-lg text-muted-foreground leading-relaxed">{c.intro}</p>

      <section className="mt-10 overflow-x-auto rounded-xl border border-border/60 bg-card">
        <table className="w-full text-sm">
          <thead className="bg-muted/40 text-xs uppercase tracking-wide text-muted-foreground">
            <tr>
              <th className="text-left font-medium px-4 py-3">{c.th_tier}</th>
              <th className="text-left font-medium px-4 py-3">{c.th_monthly}</th>
              <th className="text-left font-medium px-4 py-3">{c.th_annual}</th>
              <th className="text-left font-medium px-4 py-3">{c.th_subs}</th>
              <th className="text-left font-medium px-4 py-3">{c.th_sla}</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border/50">
            {c.tiers.map((t) => (
              <tr key={t.name}>
                <td className="px-4 py-3 font-medium">{t.name}</td>
                <td className="px-4 py-3">{t.price}</td>
                <td className="px-4 py-3 text-muted-foreground">{t.annual}</td>
                <td className="px-4 py-3 text-muted-foreground">{t.subscribers}</td>
                <td className="px-4 py-3 text-muted-foreground">{t.sla ? c.yes : c.no}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>

      <section className="mt-12">
        <h2 className="text-2xl font-bold tracking-tight">{c.h2_what_get}</h2>
        <div className="mt-6 space-y-6">
          {c.tiers.map((t) => (
            <div key={t.name} className="rounded-lg border border-border/60 bg-card p-5">
              <div className="flex items-baseline gap-3 flex-wrap">
                <h3 className="text-lg font-semibold">{t.name}</h3>
                <span className="text-sm text-muted-foreground">{t.price}</span>
              </div>
              <p className="mt-2 text-sm text-muted-foreground leading-relaxed">{t.bestFor}</p>
            </div>
          ))}
        </div>
      </section>

      <section className="mt-12 rounded-2xl border-2 border-primary/40 bg-card p-8">
        <h2 className="text-2xl font-bold tracking-tight">{c.h2_when}</h2>
        <div className="mt-5 grid md:grid-cols-2 gap-6 text-sm">
          <div>
            <h3 className="font-semibold mb-3">{c.we_cover}</h3>
            <ul className="space-y-2 text-muted-foreground">
              {c.we_cover_items.map((x) => <li key={x}>{x}</li>)}
            </ul>
          </div>
          <div>
            <h3 className="font-semibold mb-3">{c.stay_atlassian}</h3>
            <ul className="space-y-2 text-muted-foreground">
              {c.stay_items.map((x) => <li key={x}>{x}</li>)}
            </ul>
          </div>
        </div>
      </section>

      <div className="mt-12 rounded-2xl border border-border/60 bg-card p-8 text-center">
        <h2 className="text-2xl font-bold tracking-tight">{c.cta_box_h}</h2>
        <p className="mt-3 text-sm text-muted-foreground max-w-xl mx-auto">{c.cta_box_body}</p>
        <div className="mt-5 flex gap-3 justify-center flex-wrap">
          <Link
            href={`/${lang}/register?intent=pro`}
            className={buttonVariants({ size: "lg" })}
          >
            {c.cta_box_btn} <ArrowRight className="ml-2 h-4 w-4" />
          </Link>
          <Link
            href={`/${lang}/alternatives/atlassian-statuspage`}
            className={buttonVariants({ variant: "outline", size: "lg" })}
          >
            {c.full_compare}
          </Link>
        </div>
      </div>
    </div>
  );
}

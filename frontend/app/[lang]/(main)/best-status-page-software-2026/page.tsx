import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ArrowRight } from "lucide-react";
import { ALTERNATIVES } from "@/content/alternatives";
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
  const c = dict.seo_best;
  return {
    title: c.metaTitle,
    description: c.metaDesc,
    alternates: {
      canonical: `/${lang}/best-status-page-software-2026`,
      languages: Object.fromEntries(
        SUPPORTED_LOCALES.map((l) => [l, `/${l}/best-status-page-software-2026`]),
      ),
    },
  };
}

export default async function BestStatusPageSoftwarePage({
  params,
}: {
  params: Params;
}) {
  const { lang } = await params;
  if (!hasLocale(lang)) notFound();
  const dict = await getDictionary(lang);
  const c = dict.seo_best;

  return (
    <div className="container mx-auto px-4 py-12 max-w-4xl">
      <BreadcrumbListJsonLd
        items={[
          { name: dict.alternatives_template.home, url: `/${lang}` },
          { name: c.crumb, url: `/${lang}/best-status-page-software-2026` },
        ]}
      />
      <h1 className="text-4xl md:text-5xl font-bold tracking-tight leading-tight">
        {c.h1}
      </h1>
      <p className="mt-4 text-lg text-muted-foreground leading-relaxed">{c.intro}</p>

      <div className="mt-12 space-y-6">
        {c.ranked.map((r) => {
          const alt = ALTERNATIVES[r.slug];
          const href = r.hrefSelf
            ? `/${lang}/register?intent=pro`
            : `/${lang}/alternatives/${r.slug}`;
          return (
            <article
              key={r.slug}
              className={`rounded-xl border p-6 ${
                r.rank === 1
                  ? "border-primary/40 bg-card ring-1 ring-primary/20"
                  : "border-border/60 bg-card"
              }`}
            >
              <div className="flex items-start gap-4">
                <div className="flex-shrink-0 h-10 w-10 rounded-full bg-primary/10 text-primary font-bold flex items-center justify-center">
                  {r.rank}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-baseline gap-3 flex-wrap">
                    <h2 className="text-xl font-semibold">{r.name}</h2>
                    <span className="text-xs uppercase tracking-wider text-muted-foreground">
                      {r.price}
                    </span>
                  </div>
                  <p className="mt-1 text-sm font-medium text-muted-foreground">{r.tagline}</p>
                  <p className="mt-3 text-sm text-muted-foreground leading-relaxed">{r.verdict}</p>
                  <div className="mt-4 flex gap-3 flex-wrap text-sm">
                    <Link
                      href={href}
                      className="text-primary underline underline-offset-4 hover:text-foreground"
                    >
                      {r.rank === 1 ? c.start_pingcast : c.full_compare}
                    </Link>
                    {alt ? (
                      <a
                        href={alt.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-muted-foreground underline underline-offset-4 hover:text-foreground"
                      >
                        {c.visit} {r.name} ↗
                      </a>
                    ) : null}
                  </div>
                </div>
              </div>
            </article>
          );
        })}
      </div>

      <div className="mt-12 rounded-2xl border border-border/60 bg-card p-8 text-center">
        <h2 className="text-2xl font-bold tracking-tight">{c.cta_box_h}</h2>
        <p className="mt-3 text-sm text-muted-foreground max-w-xl mx-auto">{c.cta_box_body}</p>
        <Link
          href={`/${lang}/register?intent=pro`}
          className={`${buttonVariants({ size: "lg" })} mt-5`}
        >
          {c.cta_box_btn} <ArrowRight className="ml-2 h-4 w-4" />
        </Link>
      </div>
    </div>
  );
}

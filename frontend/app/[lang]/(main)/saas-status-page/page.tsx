import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ArrowRight, Check } from "lucide-react";
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
  const c = dict.seo_saas;
  return {
    title: c.metaTitle,
    description: c.metaDesc,
    alternates: {
      canonical: `/${lang}/saas-status-page`,
      languages: Object.fromEntries(
        SUPPORTED_LOCALES.map((l) => [l, `/${l}/saas-status-page`]),
      ),
    },
  };
}

export default async function SaaSStatusPagePage({
  params,
}: {
  params: Params;
}) {
  const { lang } = await params;
  if (!hasLocale(lang)) notFound();
  const dict = await getDictionary(lang);
  const c = dict.seo_saas;

  return (
    <div className="container mx-auto px-4 py-12 max-w-3xl">
      <BreadcrumbListJsonLd
        items={[
          { name: dict.alternatives_template.home, url: `/${lang}` },
          { name: c.crumb, url: `/${lang}/saas-status-page` },
        ]}
      />
      <h1 className="text-4xl md:text-5xl font-bold tracking-tight leading-tight">
        {c.h1}
      </h1>
      <p className="mt-4 text-lg text-muted-foreground leading-relaxed">{c.intro}</p>

      <div className="mt-8 flex flex-wrap gap-3">
        <Link
          href={`/${lang}/register?intent=pro`}
          className={buttonVariants({ size: "lg" })}
        >
          {c.cta_pro} <ArrowRight className="ml-2 h-4 w-4" />
        </Link>
        <Link
          href={`/${lang}/alternatives/atlassian-statuspage`}
          className={buttonVariants({ variant: "outline", size: "lg" })}
        >
          {c.vs_atlassian}
        </Link>
      </div>

      <section className="mt-12">
        <h2 className="text-2xl font-bold tracking-tight">{c.h2_what_want}</h2>
        <ul className="mt-4 space-y-3 text-sm">
          {c.points.map((f) => (
            <li key={f.title} className="flex gap-3">
              <Check className="h-5 w-5 text-primary shrink-0 mt-0.5" />
              <div>
                <strong>{f.title}</strong>
                <p className="text-muted-foreground mt-0.5">{f.body}</p>
              </div>
            </li>
          ))}
        </ul>
      </section>

      <section className="mt-12">
        <h2 className="text-2xl font-bold tracking-tight">{c.h2_stages}</h2>
        <div className="mt-6 space-y-5">
          {c.stages.map((s) => (
            <div key={s.stage} className="rounded-lg border border-border/60 bg-card p-5">
              <h3 className="text-sm font-semibold uppercase tracking-wider text-primary">
                {s.stage}
              </h3>
              <p className="mt-2 text-sm text-muted-foreground leading-relaxed">{s.body}</p>
            </div>
          ))}
        </div>
      </section>

      <div className="mt-16 rounded-2xl border border-border/60 bg-card p-8 text-center">
        <h2 className="text-2xl font-bold tracking-tight">{c.cta_box_h}</h2>
        <p className="mt-3 text-sm text-muted-foreground max-w-xl mx-auto">{c.cta_box_body}</p>
        <Link
          href={`/${lang}/register?intent=pro`}
          className={`${buttonVariants({ size: "lg" })} mt-5`}
        >
          {c.cta_box_btn}
        </Link>
      </div>
    </div>
  );
}

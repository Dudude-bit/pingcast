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
  const c = dict.seo_template;
  return {
    title: c.metaTitle,
    description: c.metaDesc,
    alternates: {
      canonical: `/${lang}/status-page-template`,
      languages: Object.fromEntries(
        SUPPORTED_LOCALES.map((l) => [l, `/${l}/status-page-template`]),
      ),
    },
  };
}

export default async function StatusPageTemplatePage({
  params,
}: {
  params: Params;
}) {
  const { lang } = await params;
  if (!hasLocale(lang)) notFound();
  const dict = await getDictionary(lang);
  const c = dict.seo_template;

  return (
    <div className="container mx-auto px-4 py-12 max-w-3xl">
      <BreadcrumbListJsonLd
        items={[
          { name: dict.alternatives_template.home, url: `/${lang}` },
          { name: c.crumb, url: `/${lang}/status-page-template` },
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
          {c.cta_use} <ArrowRight className="ml-2 h-4 w-4" />
        </Link>
        <Link
          href={`/${lang}/how-to-create-status-page`}
          className={buttonVariants({ variant: "outline", size: "lg" })}
        >
          {c.cta_setup}
        </Link>
      </div>

      <section className="mt-12">
        <h2 className="text-2xl font-bold tracking-tight">{c.h2_templates}</h2>
        <div className="mt-6 space-y-5">
          {c.templates.map((t, i) => (
            <div key={i} className="rounded-lg border border-border/60 bg-card p-5">
              <div className="flex items-center gap-3 flex-wrap">
                <span className="text-xs uppercase tracking-wider font-semibold text-primary">
                  {t.state}
                </span>
                <span className="text-xs text-muted-foreground">{t.label}</span>
              </div>
              <pre className="mt-3 text-sm text-foreground whitespace-pre-wrap font-sans leading-relaxed">
                {t.body}
              </pre>
            </div>
          ))}
        </div>
      </section>

      <section className="mt-12">
        <h2 className="text-2xl font-bold tracking-tight">{c.h2_structure}</h2>
        <p className="mt-3 text-muted-foreground leading-relaxed">{c.structure_intro}</p>
        <ol className="mt-4 list-decimal pl-6 space-y-2 text-sm text-muted-foreground">
          {c.structure.map(([title, body]) => (
            <li key={title}>
              <strong>{title}</strong> — {body}
            </li>
          ))}
        </ol>
        <p className="mt-4 text-sm text-muted-foreground leading-relaxed">
          {c.structure_outro}
        </p>
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

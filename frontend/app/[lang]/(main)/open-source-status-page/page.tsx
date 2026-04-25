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
  const c = dict.seo_open_source;
  return {
    title: c.metaTitle,
    description: c.metaDesc,
    alternates: {
      canonical: `/${lang}/open-source-status-page`,
      languages: Object.fromEntries(
        SUPPORTED_LOCALES.map((l) => [l, `/${l}/open-source-status-page`]),
      ),
    },
  };
}

export default async function OpenSourceStatusPagePage({
  params,
}: {
  params: Params;
}) {
  const { lang } = await params;
  if (!hasLocale(lang)) notFound();
  const dict = await getDictionary(lang);
  const c = dict.seo_open_source;

  return (
    <div className="container mx-auto px-4 py-12 max-w-3xl">
      <BreadcrumbListJsonLd
        items={[
          { name: dict.alternatives_template.home, url: `/${lang}` },
          { name: c.crumb, url: `/${lang}/open-source-status-page` },
        ]}
      />
      <h1 className="text-4xl md:text-5xl font-bold tracking-tight leading-tight">
        {c.h1}
      </h1>
      <p className="mt-4 text-lg text-muted-foreground leading-relaxed">{c.intro}</p>

      <div className="mt-8 flex flex-wrap gap-3">
        <a
          href="https://github.com/kirillinakin/pingcast"
          target="_blank"
          rel="noopener noreferrer"
          className={buttonVariants({ size: "lg" })}
        >
          {c.cta_repo} <ArrowRight className="ml-2 h-4 w-4" />
        </a>
        <Link
          href={`/${lang}/register?intent=pro`}
          className={buttonVariants({ variant: "outline", size: "lg" })}
        >
          {c.cta_hosted}
        </Link>
      </div>

      <section className="mt-12">
        <h2 className="text-2xl font-bold tracking-tight">{c.h2_mit}</h2>
        <p className="mt-3 text-muted-foreground leading-relaxed">{c.mit_body}</p>
      </section>

      <section className="mt-12">
        <h2 className="text-2xl font-bold tracking-tight">{c.h2_self}</h2>
        <pre className="mt-4 overflow-x-auto rounded-lg border border-border/60 bg-card p-4 text-sm font-mono">
          <code>{`git clone https://github.com/kirillinakin/pingcast
cd pingcast && docker compose up -d
# → http://localhost:3001`}</code>
        </pre>
        <p className="mt-3 text-sm text-muted-foreground leading-relaxed">{c.self_body}</p>
      </section>

      <section className="mt-12">
        <h2 className="text-2xl font-bold tracking-tight">{c.h2_features}</h2>
        <ul className="mt-4 grid md:grid-cols-2 gap-3 text-sm">
          {c.features.map((f) => (
            <li key={f} className="flex gap-2 items-start">
              <span className="text-primary mt-1">✓</span>
              <span className="text-muted-foreground">{f}</span>
            </li>
          ))}
        </ul>
      </section>

      <div className="mt-16 rounded-2xl border border-border/60 bg-card p-8 text-center">
        <h2 className="text-2xl font-bold tracking-tight">{c.cta_box_h}</h2>
        <p className="mt-3 text-sm text-muted-foreground max-w-xl mx-auto">{c.cta_box_body}</p>
        <div className="mt-5 flex gap-3 justify-center flex-wrap">
          <a
            href="https://github.com/kirillinakin/pingcast"
            target="_blank"
            rel="noopener noreferrer"
            className={buttonVariants({ size: "lg" })}
          >
            {c.cta_box_btn}
          </a>
          <Link
            href={`/${lang}/pricing`}
            className={buttonVariants({ variant: "outline", size: "lg" })}
          >
            {c.cta_box_btn2}
          </Link>
        </div>
      </div>
    </div>
  );
}

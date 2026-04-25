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
  const c = dict.seo_howto;
  return {
    title: c.metaTitle,
    description: c.metaDesc,
    alternates: {
      canonical: `/${lang}/how-to-create-status-page`,
      languages: Object.fromEntries(
        SUPPORTED_LOCALES.map((l) => [l, `/${l}/how-to-create-status-page`]),
      ),
    },
  };
}

export default async function HowToCreateStatusPagePage({
  params,
}: {
  params: Params;
}) {
  const { lang } = await params;
  if (!hasLocale(lang)) notFound();
  const dict = await getDictionary(lang);
  const c = dict.seo_howto;

  return (
    <div className="container mx-auto px-4 py-12 max-w-3xl">
      <BreadcrumbListJsonLd
        items={[
          { name: dict.alternatives_template.home, url: `/${lang}` },
          { name: c.crumb, url: `/${lang}/how-to-create-status-page` },
        ]}
      />
      <h1 className="text-4xl md:text-5xl font-bold tracking-tight leading-tight">
        {c.h1}
      </h1>
      <p className="mt-4 text-lg text-muted-foreground leading-relaxed">{c.intro}</p>

      <ol className="mt-12 space-y-10">
        {c.steps.map((s, i) => (
          <li key={s.title} className="flex gap-5">
            <span className="flex-shrink-0 h-10 w-10 rounded-full bg-primary/10 text-primary font-bold text-lg flex items-center justify-center">
              {i + 1}
            </span>
            <div className="flex-1 pt-1">
              <h3 className="text-xl font-semibold">{s.title}</h3>
              <p className="mt-2 text-muted-foreground leading-relaxed">{s.body}</p>
            </div>
          </li>
        ))}
      </ol>

      <div className="mt-16 rounded-2xl border border-border/60 bg-card p-8 text-center">
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

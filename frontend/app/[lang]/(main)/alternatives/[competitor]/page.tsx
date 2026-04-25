import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ArrowRight, Check, X } from "lucide-react";
import {
  getAlternative,
  listAlternativeSlugs,
} from "@/content/alternatives";
import { buttonVariants } from "@/components/ui/button";
import { BreadcrumbListJsonLd, FaqPageJsonLd } from "@/components/seo/jsonld";
import { getDictionary, hasLocale, SUPPORTED_LOCALES } from "@/lib/i18n";

type Params = Promise<{ lang: string; competitor: string }>;

export function generateStaticParams() {
  const slugs = listAlternativeSlugs();
  return SUPPORTED_LOCALES.flatMap((lang) =>
    slugs.map((c) => ({ lang, competitor: c })),
  );
}

export async function generateMetadata({
  params,
}: {
  params: Params;
}): Promise<Metadata> {
  const { lang, competitor } = await params;
  if (!hasLocale(lang)) return {};
  const alt = getAlternative(competitor, lang);
  if (!alt) return {};
  return {
    title: alt.metaTitle,
    description: alt.metaDescription,
    alternates: {
      canonical: `/${lang}/alternatives/${alt.slug}`,
      languages: Object.fromEntries(
        SUPPORTED_LOCALES.map((l) => [l, `/${l}/alternatives/${alt.slug}`]),
      ),
    },
    openGraph: {
      title: alt.metaTitle,
      description: alt.metaDescription,
      type: "article",
      locale: lang === "ru" ? "ru_RU" : "en_US",
    },
  };
}

export default async function AlternativePage({ params }: { params: Params }) {
  const { lang, competitor } = await params;
  if (!hasLocale(lang)) notFound();
  const alt = getAlternative(competitor, lang);
  if (!alt) notFound();
  const dict = await getDictionary(lang);
  const t = dict.alternatives_template;

  return (
    <div className="container mx-auto px-4 py-12 max-w-4xl">
      <BreadcrumbListJsonLd
        items={[
          { name: t.home, url: `/${lang}` },
          { name: t.breadcrumb, url: `/${lang}/alternatives` },
          { name: alt.name, url: `/${lang}/alternatives/${alt.slug}` },
        ]}
      />
      <FaqPageJsonLd items={alt.faq} />

      <nav className="mb-8 text-sm text-muted-foreground">
        <Link href={`/${lang}`} className="hover:text-foreground">
          {t.home}
        </Link>
        {" / "}
        <span className="text-foreground">vs {alt.name}</span>
      </nav>

      <header className="mb-12">
        <h1 className="text-4xl md:text-5xl font-bold tracking-tight leading-tight">
          {alt.hero.headline}
        </h1>
        <p className="mt-4 text-lg text-muted-foreground leading-relaxed max-w-3xl">
          {alt.hero.sub}
        </p>
        <div className="mt-6 flex flex-wrap gap-3">
          <Link
            href={`/${lang}/register?intent=pro`}
            className={buttonVariants({ size: "lg" })}
          >
            {t.cta_pro} <ArrowRight className="ml-2 h-4 w-4" />
          </Link>
          <a
            href={alt.url}
            target="_blank"
            rel="noopener noreferrer"
            className={buttonVariants({ variant: "outline", size: "lg" })}
          >
            {t.visit} {alt.name}
          </a>
        </div>
      </header>

      <section className="mb-12 overflow-x-auto rounded-xl border border-border/60 bg-card">
        <table className="w-full text-sm">
          <thead className="bg-muted/40 text-xs uppercase tracking-wide text-muted-foreground">
            <tr>
              <th className="text-left font-medium px-4 py-3 w-1/2">{t.col_feature}</th>
              <th className="text-left font-medium px-4 py-3">PingCast</th>
              <th className="text-left font-medium px-4 py-3">{alt.name}</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border/50">
            <Row label={t.row_price} us={t.us_price} them={alt.startingPrice} />
            <Row label="Open source" us="MIT" them={alt.openSource ? t.row_oss_alt_label : false} />
            <Row label={t.row_self_host} us={true} them={alt.selfHostable} />
            <Row label={t.row_uptime} us={true} them={alt.includesUptime} />
            <Row label={t.row_atlassian} us={true} them={alt.atlassianImport} />
            <Row label={t.row_custom_domain} us={true} them={true} />
            <Row label={t.row_subscribers} us={true} them={true} />
            <Row label={t.row_ru} us={true} them={alt.russiaAvailable} />
          </tbody>
        </table>
      </section>

      <div className="grid md:grid-cols-2 gap-6 mb-12">
        <section className="rounded-lg border border-border/60 bg-card p-6">
          <h2 className="font-semibold text-lg mb-4">
            {t.when_them.replace("{name}", alt.name)}
          </h2>
          <ul className="space-y-2 text-sm text-muted-foreground">
            {alt.whenThem.map((x) => (
              <li key={x} className="flex gap-2">
                <span className="text-muted-foreground/60 shrink-0">·</span>
                <span>{x}</span>
              </li>
            ))}
          </ul>
        </section>
        <section className="rounded-lg border-2 border-primary/40 bg-card p-6">
          <h2 className="font-semibold text-lg mb-4">{t.when_us}</h2>
          <ul className="space-y-2 text-sm">
            {alt.whenUs.map((x) => (
              <li key={x} className="flex gap-2">
                <Check className="h-4 w-4 text-primary shrink-0 mt-0.5" />
                <span>{x}</span>
              </li>
            ))}
          </ul>
        </section>
      </div>

      {alt.migration ? (
        <section className="mb-12 rounded-xl border border-emerald-500/40 bg-emerald-500/5 p-8">
          <h2 className="text-2xl font-bold tracking-tight">{alt.migration.title}</h2>
          <p className="mt-3 text-sm text-muted-foreground leading-relaxed max-w-3xl">
            {alt.migration.body}
          </p>
          <Link
            href={`/${lang}/import/atlassian`}
            className={`${buttonVariants()} mt-5`}
          >
            {t.start_import} →
          </Link>
        </section>
      ) : null}

      <section className="mb-12">
        <h2 className="text-2xl font-bold tracking-tight mb-6">FAQ</h2>
        <div className="space-y-3">
          {alt.faq.map((f) => (
            <details
              key={f.q}
              className="group rounded-lg border border-border/60 bg-card px-5 py-4 [&[open]_svg]:rotate-90"
            >
              <summary className="flex cursor-pointer list-none items-center justify-between gap-4 font-medium">
                {f.q}
                <ArrowRight className="h-4 w-4 shrink-0 text-muted-foreground transition-transform" />
              </summary>
              <p className="mt-3 text-sm text-muted-foreground leading-relaxed">
                {f.a}
              </p>
            </details>
          ))}
        </div>
      </section>

      <div className="rounded-2xl border border-border/60 bg-card p-8 text-center">
        <h2 className="text-2xl font-bold tracking-tight">{t.ready_heading}</h2>
        <p className="mt-3 text-sm text-muted-foreground max-w-xl mx-auto">{t.ready_sub}</p>
        <Link
          href={`/${lang}/register?intent=pro`}
          className={`${buttonVariants({ size: "lg" })} mt-5`}
        >
          {t.cta_pro}
        </Link>
      </div>
    </div>
  );
}

function Row({
  label,
  us,
  them,
}: {
  label: string;
  us: string | boolean;
  them: string | boolean;
}) {
  return (
    <tr>
      <td className="px-4 py-3 font-medium">{label}</td>
      <td className="px-4 py-3">{renderVal(us, true)}</td>
      <td className="px-4 py-3 text-muted-foreground">{renderVal(them, false)}</td>
    </tr>
  );
}

function renderVal(v: string | boolean, emphasis: boolean) {
  if (v === true)
    return <Check className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />;
  if (v === false) return <X className="h-4 w-4 text-muted-foreground/60" />;
  return <span className={emphasis ? "font-medium" : ""}>{v}</span>;
}

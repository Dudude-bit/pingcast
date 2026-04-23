import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ArrowRight, Check, X } from "lucide-react";
import { ALTERNATIVES, listAlternativeSlugs } from "@/content/alternatives";
import { buttonVariants } from "@/components/ui/button";
import { BreadcrumbListJsonLd, FaqPageJsonLd } from "@/components/seo/jsonld";

// Static params so every alternative page is SSG'd at build time.
// Blog gets ISR, these don't — competitor pages are stable enough that
// on-demand revalidation on commit is fine.
export function generateStaticParams() {
  return listAlternativeSlugs().map((c) => ({ competitor: c }));
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ competitor: string }>;
}): Promise<Metadata> {
  const { competitor } = await params;
  const alt = ALTERNATIVES[competitor];
  if (!alt) return {};
  return {
    title: alt.metaTitle,
    description: alt.metaDescription,
    alternates: { canonical: `/alternatives/${alt.slug}` },
    openGraph: {
      title: alt.metaTitle,
      description: alt.metaDescription,
      type: "article",
    },
  };
}

export default async function AlternativePage({
  params,
}: {
  params: Promise<{ competitor: string }>;
}) {
  const { competitor } = await params;
  const alt = ALTERNATIVES[competitor];
  if (!alt) notFound();

  return (
    <div className="container mx-auto px-4 py-12 max-w-4xl">
      <BreadcrumbListJsonLd
        items={[
          { name: "Home", url: "/" },
          { name: "Alternatives", url: "/alternatives" },
          { name: alt.name, url: `/alternatives/${alt.slug}` },
        ]}
      />
      <FaqPageJsonLd items={alt.faq} />

      <nav className="mb-8 text-sm text-muted-foreground">
        <Link href="/" className="hover:text-foreground">Home</Link>
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
            href="/register?intent=pro"
            className={buttonVariants({ size: "lg" })}
          >
            Start PingCast Pro <ArrowRight className="ml-2 h-4 w-4" />
          </Link>
          <a
            href={alt.url}
            target="_blank"
            rel="noopener noreferrer"
            className={buttonVariants({ variant: "outline", size: "lg" })}
          >
            Visit {alt.name}
          </a>
        </div>
      </header>

      <section className="mb-12 overflow-x-auto rounded-xl border border-border/60 bg-card">
        <table className="w-full text-sm">
          <thead className="bg-muted/40 text-xs uppercase tracking-wide text-muted-foreground">
            <tr>
              <th className="text-left font-medium px-4 py-3 w-1/2">Feature</th>
              <th className="text-left font-medium px-4 py-3">PingCast</th>
              <th className="text-left font-medium px-4 py-3">{alt.name}</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border/50">
            <Row label="Starting price" us="$9/mo founder" them={alt.startingPrice} />
            <Row label="Open source" us="MIT" them={alt.openSource ? "AGPL / other" : false} />
            <Row label="Self-hostable" us={true} them={alt.selfHostable} />
            <Row label="Uptime monitoring included" us={true} them={alt.includesUptime} />
            <Row label="Atlassian Statuspage importer" us={true} them={alt.atlassianImport} />
            <Row label="Custom domain" us={true} them={true} />
            <Row label="Email subscribers (double opt-in)" us={true} them={true} />
            <Row label="Available in Russia" us={true} them={alt.russiaAvailable} />
          </tbody>
        </table>
      </section>

      <div className="grid md:grid-cols-2 gap-6 mb-12">
        <section className="rounded-lg border border-border/60 bg-card p-6">
          <h2 className="font-semibold text-lg mb-4">When {alt.name} is the right call</h2>
          <ul className="space-y-2 text-sm text-muted-foreground">
            {alt.whenThem.map((t) => (
              <li key={t} className="flex gap-2">
                <span className="text-muted-foreground/60 shrink-0">·</span>
                <span>{t}</span>
              </li>
            ))}
          </ul>
        </section>
        <section className="rounded-lg border-2 border-primary/40 bg-card p-6">
          <h2 className="font-semibold text-lg mb-4">When PingCast is the right call</h2>
          <ul className="space-y-2 text-sm">
            {alt.whenUs.map((t) => (
              <li key={t} className="flex gap-2">
                <Check className="h-4 w-4 text-primary shrink-0 mt-0.5" />
                <span>{t}</span>
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
            href="/import/atlassian"
            className={`${buttonVariants()} mt-5`}
          >
            Start the import →
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
        <h2 className="text-2xl font-bold tracking-tight">Ready to try PingCast?</h2>
        <p className="mt-3 text-sm text-muted-foreground max-w-xl mx-auto">
          $9/mo founder&apos;s price for the first 100 customers. Cancel anytime.
          Self-host under MIT if you outgrow the hosted tier.
        </p>
        <Link
          href="/register?intent=pro"
          className={`${buttonVariants({ size: "lg" })} mt-5`}
        >
          Start PingCast Pro
        </Link>
      </div>
    </div>
  );
}

function Row({ label, us, them }: { label: string; us: string | boolean; them: string | boolean }) {
  return (
    <tr>
      <td className="px-4 py-3 font-medium">{label}</td>
      <td className="px-4 py-3">{renderVal(us, true)}</td>
      <td className="px-4 py-3 text-muted-foreground">{renderVal(them, false)}</td>
    </tr>
  );
}

function renderVal(v: string | boolean, emphasis: boolean) {
  if (v === true) return <Check className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />;
  if (v === false) return <X className="h-4 w-4 text-muted-foreground/60" />;
  return <span className={emphasis ? "font-medium" : ""}>{v}</span>;
}

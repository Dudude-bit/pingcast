import Link from "next/link";
import { NewsletterForm } from "@/components/features/common/newsletter-form";
import { getDictionary, type Locale } from "@/lib/i18n";

// Five-column footer plus the newsletter strip on top. Receives the
// locale from app/[lang]/(main)/layout.tsx and looks up its own
// dictionary — keeps the component self-contained vs threading dict
// through every prop in the layout.
export async function Footer({ lang }: { lang: Locale }) {
  const dict = await getDictionary(lang);
  const f = dict.footer;
  const l = f.links;

  const COLUMNS: {
    heading: string;
    links: { label: string; href: string; external?: boolean }[];
  }[] = [
    {
      heading: f.col_product,
      links: [
        { label: l.features, href: `/${lang}#features` },
        { label: l.pricing, href: `/${lang}/pricing` },
        { label: l.status, href: `/status/pingcast` },
        { label: l.widget, href: "/widget.js", external: true },
        { label: l.changelog, href: `/${lang}/blog` },
      ],
    },
    {
      heading: f.col_compare,
      links: [
        { label: l.vs_atlassian, href: `/${lang}/alternatives/atlassian-statuspage` },
        { label: l.vs_instatus, href: `/${lang}/alternatives/instatus` },
        { label: l.vs_openstatus, href: `/${lang}/alternatives/openstatus` },
        { label: l.vs_uptimerobot, href: `/${lang}/alternatives/uptimerobot` },
        { label: l.vs_kuma, href: `/${lang}/alternatives/uptime-kuma` },
      ],
    },
    {
      heading: f.col_solutions,
      links: [
        { label: l.saas_status, href: `/${lang}/saas-status-page` },
        { label: l.open_source_status, href: `/${lang}/open-source-status-page` },
        { label: l.status_software, href: `/${lang}/status-page-software` },
        { label: l.best_2026, href: `/${lang}/best-status-page-software-2026` },
        { label: l.how_to_create, href: `/${lang}/how-to-create-status-page` },
      ],
    },
    {
      heading: f.col_resources,
      links: [
        { label: l.blog, href: `/${lang}/blog` },
        { label: l.docs, href: `/${lang}/docs/api` },
        { label: l.api_ref, href: `/${lang}/docs/api` },
        { label: l.template, href: `/${lang}/status-page-template` },
        { label: l.atlassian_pricing, href: `/${lang}/atlassian-statuspage-pricing` },
      ],
    },
    {
      heading: f.col_open_source,
      links: [
        { label: l.github_repo, href: "https://github.com/kirillinakin/pingcast", external: true },
        { label: l.self_host, href: "https://github.com/kirillinakin/pingcast#readme", external: true },
        { label: l.license, href: "https://github.com/kirillinakin/pingcast/blob/main/LICENSE", external: true },
        { label: l.issues, href: "https://github.com/kirillinakin/pingcast/issues", external: true },
      ],
    },
  ];

  return (
    <footer className="border-t border-border/40 bg-muted/20 mt-20">
      <div className="container mx-auto px-4 py-12">
        <div className="mb-10 rounded-lg border border-border/60 bg-card p-5 md:p-6">
          <h3 className="text-sm font-semibold mb-1">{f.newsletter_heading}</h3>
          <p className="text-sm text-muted-foreground mb-3">{f.newsletter_sub}</p>
          <div className="max-w-md">
            <NewsletterForm source="footer" />
          </div>
        </div>
        <div className="grid grid-cols-2 gap-8 md:grid-cols-5">
          {COLUMNS.map((col) => (
            <div key={col.heading}>
              <h3 className="text-xs font-semibold uppercase tracking-wider text-foreground mb-3">
                {col.heading}
              </h3>
              <ul className="space-y-2">
                {col.links.map((link) => (
                  <li key={link.href}>
                    {link.external ? (
                      <a
                        href={link.href}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-sm text-muted-foreground hover:text-foreground transition-colors"
                      >
                        {link.label}
                      </a>
                    ) : (
                      <Link
                        href={link.href}
                        className="text-sm text-muted-foreground hover:text-foreground transition-colors"
                      >
                        {link.label}
                      </Link>
                    )}
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
        <div className="mt-12 pt-8 border-t border-border/40 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 text-sm text-muted-foreground">
          <p>{f.tagline}</p>
          <p>&copy; {new Date().getFullYear()} PingCast.</p>
        </div>
      </div>
    </footer>
  );
}

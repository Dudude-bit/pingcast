import Link from "next/link";

// Five-column footer replaces the one-line stub. Every SEO landing and
// alternatives page links from here — the footer is where half the
// on-site SEO (internal linking) actually happens.
const COLUMNS: { heading: string; links: { label: string; href: string; external?: boolean }[] }[] = [
  {
    heading: "Product",
    links: [
      { label: "Features", href: "/#features" },
      { label: "Pricing", href: "/pricing" },
      { label: "Status", href: "/status/pingcast" },
      { label: "Embeddable widget", href: "/widget.js", external: true },
      { label: "Changelog", href: "/blog" },
    ],
  },
  {
    heading: "Compare",
    links: [
      { label: "vs Atlassian Statuspage", href: "/alternatives/atlassian-statuspage" },
      { label: "vs Instatus", href: "/alternatives/instatus" },
      { label: "vs Openstatus", href: "/alternatives/openstatus" },
      { label: "vs UptimeRobot", href: "/alternatives/uptimerobot" },
      { label: "vs Uptime Kuma", href: "/alternatives/uptime-kuma" },
    ],
  },
  {
    heading: "Solutions",
    links: [
      { label: "Status pages for SaaS", href: "/saas-status-page" },
      { label: "Open-source status page", href: "/open-source-status-page" },
      { label: "Status page software", href: "/status-page-software" },
      { label: "Best status pages 2026", href: "/best-status-page-software-2026" },
      { label: "How to create a status page", href: "/how-to-create-status-page" },
    ],
  },
  {
    heading: "Resources",
    links: [
      { label: "Blog", href: "/blog" },
      { label: "Docs", href: "/docs/api" },
      { label: "API reference", href: "/docs/api" },
      { label: "Status-page template", href: "/status-page-template" },
      { label: "Atlassian pricing explained", href: "/atlassian-statuspage-pricing" },
    ],
  },
  {
    heading: "Open source",
    links: [
      { label: "GitHub repository", href: "https://github.com/kirillinakin/pingcast", external: true },
      { label: "Self-host guide", href: "https://github.com/kirillinakin/pingcast#readme", external: true },
      { label: "MIT License", href: "https://github.com/kirillinakin/pingcast/blob/main/LICENSE", external: true },
      { label: "Report an issue", href: "https://github.com/kirillinakin/pingcast/issues", external: true },
    ],
  },
];

export function Footer() {
  return (
    <footer className="border-t border-border/40 bg-muted/20 mt-20">
      <div className="container mx-auto px-4 py-12">
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
          <p>
            PingCast &mdash; branded status pages for SaaS, at a third of
            Atlassian&rsquo;s price. Open source under{" "}
            <a
              href="https://github.com/kirillinakin/pingcast/blob/main/LICENSE"
              target="_blank"
              rel="noopener noreferrer"
              className="underline hover:text-foreground"
            >
              MIT
            </a>
            .
          </p>
          <p>&copy; {new Date().getFullYear()} PingCast.</p>
        </div>
      </div>
    </footer>
  );
}

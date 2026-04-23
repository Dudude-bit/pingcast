import Link from "next/link";
import {
  Sparkles,
  Globe,
  FolderTree,
  Wrench,
  Upload,
  Users,
  BookOpen,
} from "lucide-react";

// ProNav is the dashboard entry-point to every Pro surface. Each card
// links to an admin page we've shipped. Free users still see the
// cards — they hit 402 inside the flow (e.g. PATCH /me/branding) which
// surfaces as a toast, and the Upgrade button at the top of dashboard
// takes them to checkout.
const CARDS = [
  {
    href: "/dashboard/branding",
    icon: Sparkles,
    title: "Branding",
    body: "Logo, accent colour, and custom footer for your status page.",
  },
  {
    href: "/dashboard/custom-domain",
    icon: Globe,
    title: "Custom domain",
    body: "Point status.yourcompany.com at our edge with CNAME + TLS.",
  },
  {
    href: "/dashboard/groups",
    icon: FolderTree,
    title: "Monitor groups",
    body: "Organise monitors into collapsible sections on the status page.",
  },
  {
    href: "/dashboard/maintenance",
    icon: Wrench,
    title: "Maintenance windows",
    body: "Schedule planned downtime so alerts don't fire and the page shows 'scheduled maintenance'.",
  },
  {
    href: "/dashboard/subscribers",
    icon: Users,
    title: "Subscribers",
    body: "Confirmed email subscribers to your public status page.",
  },
  {
    href: "/import/atlassian",
    icon: Upload,
    title: "Import from Atlassian",
    body: "Upload your Statuspage JSON export; monitors and incidents recreate in one click.",
  },
  {
    href: "/docs/api",
    icon: BookOpen,
    title: "API reference",
    body: "Every feature, scoped API keys, OpenAPI spec. curl the dashboard.",
  },
];

export function ProNav() {
  return (
    <section className="mt-10">
      <h2 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground mb-4">
        Pro features
      </h2>
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {CARDS.map((c) => (
          <Link
            key={c.href}
            href={c.href}
            className="group rounded-lg border border-border/60 bg-card p-4 hover:border-border hover:bg-accent/20 transition-colors"
          >
            <div className="flex items-start gap-3">
              <span className="inline-flex h-9 w-9 items-center justify-center rounded-md bg-primary/10 text-primary shrink-0">
                <c.icon className="h-4 w-4" />
              </span>
              <div className="min-w-0">
                <h3 className="font-medium text-sm group-hover:text-primary transition-colors">
                  {c.title}
                </h3>
                <p className="mt-1 text-xs text-muted-foreground leading-relaxed">
                  {c.body}
                </p>
              </div>
            </div>
          </Link>
        ))}
      </div>
    </section>
  );
}

"use client";

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
import { useLocale } from "@/components/i18n/locale-provider";

// ProNav is the dashboard entry-point to every Pro surface. Each card
// links to an admin page we've shipped. Free users still see the
// cards — they hit 402 inside the flow (e.g. PATCH /me/branding) which
// surfaces as a toast, and the Upgrade button at the top of dashboard
// takes them to checkout.
export function ProNav() {
  const { dict, locale } = useLocale();
  const t = dict.pro_nav;
  const cards = [
    { href: `/${locale}/dashboard/branding`, icon: Sparkles, title: t.branding_title, body: t.branding_body },
    { href: `/${locale}/dashboard/custom-domain`, icon: Globe, title: t.domain_title, body: t.domain_body },
    { href: `/${locale}/dashboard/groups`, icon: FolderTree, title: t.groups_title, body: t.groups_body },
    { href: `/${locale}/dashboard/maintenance`, icon: Wrench, title: t.maintenance_title, body: t.maintenance_body },
    { href: `/${locale}/dashboard/subscribers`, icon: Users, title: t.subscribers_title, body: t.subscribers_body },
    { href: `/${locale}/import/atlassian`, icon: Upload, title: t.import_title, body: t.import_body },
    { href: `/${locale}/docs/api`, icon: BookOpen, title: t.api_title, body: t.api_body },
  ];

  return (
    <section className="mt-10">
      <h2 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground mb-4">
        {t.heading}
      </h2>
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {cards.map((c) => (
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

"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { ArrowLeft, Mail, Users } from "lucide-react";
import type { components } from "@/lib/openapi-types";
import { useLocale } from "@/components/i18n/locale-provider";

type Subscriber = components["schemas"]["StatusSubscriber"];

export default function SubscribersPage() {
  const { dict, locale } = useLocale();
  const t = dict.dashboard_subscribers;
  const [subs, setSubs] = useState<Subscriber[] | null>(null);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    fetch("/api/me/subscribers", { credentials: "include" })
      .then((r) =>
        r.ok ? r.json() : Promise.reject(new Error(`HTTP ${r.status}`)),
      )
      .then((d: Subscriber[]) => setSubs(d))
      .catch((e) => setErr(e instanceof Error ? e.message : "load failed"));
  }, []);

  return (
    <div className="container mx-auto px-4 py-12 max-w-3xl">
      <Link
        href={`/${locale}/dashboard`}
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mb-6"
      >
        <ArrowLeft className="h-3.5 w-3.5" /> {dict.common.back_to_dashboard}
      </Link>

      <div className="flex items-center gap-3">
        <div className="inline-flex h-9 w-9 items-center justify-center rounded-md bg-primary/10 text-primary">
          <Users className="h-5 w-5" />
        </div>
        <h1 className="text-2xl font-bold tracking-tight">{t.title}</h1>
      </div>
      <p className="mt-3 text-sm text-muted-foreground max-w-xl">{t.subtitle}</p>

      {err ? (
        <p className="mt-8 text-sm text-destructive">{t.load_failed}: {err}</p>
      ) : subs === null ? (
        <p className="mt-8 text-sm text-muted-foreground">{dict.common.loading}</p>
      ) : subs.length === 0 ? (
        <div className="mt-8 rounded-lg border border-border/60 bg-card p-8 text-center">
          <Mail className="h-8 w-8 text-muted-foreground mx-auto mb-3" />
          <p className="font-medium">{t.empty_title}</p>
          <p className="mt-2 text-sm text-muted-foreground max-w-md mx-auto">
            {t.empty_sub}
          </p>
        </div>
      ) : (
        <>
          <p className="mt-6 text-sm text-muted-foreground">
            {(subs.length === 1 ? t.count_one : t.count_many).replace(
              "{n}",
              String(subs.length),
            )}
          </p>
          <ul className="mt-3 rounded-lg border border-border/60 bg-card divide-y divide-border/40">
            {subs.map((s) => (
              <li
                key={s.id}
                className="flex items-center justify-between gap-4 px-5 py-3 flex-wrap"
              >
                <span className="font-mono text-sm truncate">{s.email}</span>
                <span className="text-xs text-muted-foreground shrink-0">
                  {t.confirmed_at}{" "}
                  {new Date(s.confirmed_at).toLocaleDateString(
                    locale === "ru" ? "ru-RU" : "en-US",
                  )}
                </span>
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  );
}

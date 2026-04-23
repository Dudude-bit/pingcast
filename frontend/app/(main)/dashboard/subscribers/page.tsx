"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { ArrowLeft, Mail, Users } from "lucide-react";
import type { components } from "@/lib/openapi-types";

type Subscriber = components["schemas"]["StatusSubscriber"];

export default function SubscribersPage() {
  const [subs, setSubs] = useState<Subscriber[] | null>(null);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    fetch("/api/me/subscribers", { credentials: "include" })
      .then((r) =>
        r.ok
          ? r.json()
          : Promise.reject(new Error(`HTTP ${r.status}`)),
      )
      .then((d: Subscriber[]) => setSubs(d))
      .catch((e) => setErr(e instanceof Error ? e.message : "load failed"));
  }, []);

  return (
    <div className="container mx-auto px-4 py-12 max-w-3xl">
      <Link
        href="/dashboard"
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mb-6"
      >
        <ArrowLeft className="h-3.5 w-3.5" /> Back to dashboard
      </Link>

      <div className="flex items-center gap-3">
        <div className="inline-flex h-9 w-9 items-center justify-center rounded-md bg-primary/10 text-primary">
          <Users className="h-5 w-5" />
        </div>
        <h1 className="text-2xl font-bold tracking-tight">Status-page subscribers</h1>
      </div>
      <p className="mt-3 text-sm text-muted-foreground max-w-xl">
        Confirmed email subscribers to your public status page. Pending
        (unconfirmed) subscriptions don&apos;t show — they&apos;ve either
        clicked the confirm link or they haven&apos;t and don&apos;t
        count. Every outbound email carries a one-click unsubscribe.
      </p>

      {err ? (
        <p className="mt-8 text-sm text-destructive">Failed to load subscribers: {err}</p>
      ) : subs === null ? (
        <p className="mt-8 text-sm text-muted-foreground">Loading…</p>
      ) : subs.length === 0 ? (
        <div className="mt-8 rounded-lg border border-border/60 bg-card p-8 text-center">
          <Mail className="h-8 w-8 text-muted-foreground mx-auto mb-3" />
          <p className="font-medium">No confirmed subscribers yet.</p>
          <p className="mt-2 text-sm text-muted-foreground max-w-md mx-auto">
            The subscribe box is live on your public status page. When your
            first customer opts in and confirms, they&apos;ll show up here.
          </p>
        </div>
      ) : (
        <>
          <p className="mt-6 text-sm text-muted-foreground">
            {subs.length} {subs.length === 1 ? "subscriber" : "subscribers"}
          </p>
          <ul className="mt-3 rounded-lg border border-border/60 bg-card divide-y divide-border/40">
            {subs.map((s) => (
              <li
                key={s.id}
                className="flex items-center justify-between gap-4 px-5 py-3 flex-wrap"
              >
                <span className="font-mono text-sm truncate">{s.email}</span>
                <span className="text-xs text-muted-foreground shrink-0">
                  Confirmed {new Date(s.confirmed_at).toLocaleDateString()}
                </span>
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  );
}

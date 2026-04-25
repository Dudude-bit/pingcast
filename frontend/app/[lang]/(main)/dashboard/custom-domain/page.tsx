"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import {
  ArrowLeft,
  Globe,
  Trash2,
  AlertTriangle,
  CheckCircle2,
  Clock,
  Loader2,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toast } from "sonner";
import type { components } from "@/lib/openapi-types";
import { useLocale } from "@/components/i18n/locale-provider";

type CustomDomain = components["schemas"]["CustomDomain"];

export default function CustomDomainPage() {
  const { dict, locale } = useLocale();
  const t = dict.dashboard_custom_domain;
  const [domains, setDomains] = useState<CustomDomain[] | null>(null);
  const [hostname, setHostname] = useState("");
  const [busy, setBusy] = useState(false);
  const [reloadTick, setReloadTick] = useState(0);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      const res = await fetch("/api/custom-domains", { credentials: "include" });
      if (!cancelled && res.ok) {
        setDomains((await res.json()) as CustomDomain[]);
      }
    };
    void load();
    const tm = setInterval(() => void load(), 15_000);
    return () => {
      cancelled = true;
      clearInterval(tm);
    };
  }, [reloadTick]);

  const load = () => setReloadTick((n) => n + 1);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      const res = await fetch("/api/custom-domains", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ hostname }),
        credentials: "include",
      });
      if (res.status === 402) {
        toast.error(t.pro_required);
        return;
      }
      if (!res.ok) {
        const body = await res.json().catch(() => null);
        toast.error(body?.error?.message ?? `${t.add_failed} (HTTP ${res.status}).`);
        return;
      }
      setHostname("");
      toast.success(t.added);
      load();
    } finally {
      setBusy(false);
    }
  }

  async function remove(id: number) {
    if (!confirm(t.remove_confirm)) return;
    const res = await fetch(`/api/custom-domains/${id}`, {
      method: "DELETE",
      credentials: "include",
    });
    if (res.ok) {
      toast.success(t.removed);
      load();
    } else {
      toast.error(`${t.remove_failed} (HTTP ${res.status}).`);
    }
  }

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
          <Globe className="h-5 w-5" />
        </div>
        <h1 className="text-2xl font-bold tracking-tight">{t.title}</h1>
      </div>
      <p className="mt-3 text-sm text-muted-foreground max-w-xl">{t.subtitle}</p>

      <form
        onSubmit={submit}
        className="mt-8 rounded-lg border border-border/60 bg-card p-5 space-y-4"
      >
        <h2 className="text-sm font-semibold">{t.add_heading}</h2>
        <div className="space-y-2">
          <Label htmlFor="hostname">{t.hostname_label}</Label>
          <Input
            id="hostname"
            type="text"
            placeholder={t.hostname_placeholder}
            value={hostname}
            onChange={(e) => setHostname(e.target.value)}
            required
            disabled={busy}
          />
          <p className="text-xs text-muted-foreground">{t.hostname_help}</p>
        </div>
        <Button type="submit" disabled={busy || !hostname}>
          {busy ? t.adding : t.add_button}
        </Button>
      </form>

      <section className="mt-10">
        <h2 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground mb-3">
          {t.list_heading}
        </h2>
        {domains === null ? (
          <p className="text-sm text-muted-foreground">{dict.common.loading}</p>
        ) : domains.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t.list_empty}</p>
        ) : (
          <ul className="space-y-4">
            {domains.map((d) => (
              <li key={d.id} className="rounded-lg border border-border/60 bg-card p-5">
                <div className="flex items-start justify-between gap-4 flex-wrap">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <code className="font-medium">{d.hostname}</code>
                      <StatusPill status={d.status} t={t} />
                    </div>
                  </div>
                  <button
                    onClick={() => remove(d.id)}
                    className="text-muted-foreground hover:text-destructive transition-colors"
                    aria-label={`${t.remove} ${d.hostname}`}
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>

                {d.status === "pending" || d.status === "failed" ? (
                  <Instructions domain={d} t={t} />
                ) : null}

                {d.last_error ? (
                  <div className="mt-4 flex items-start gap-2 rounded-md border border-destructive/40 bg-destructive/5 px-3 py-2 text-xs text-destructive">
                    <AlertTriangle className="h-3.5 w-3.5 shrink-0 mt-0.5" />
                    <span>{d.last_error}</span>
                  </div>
                ) : null}
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}

function StatusPill({
  status,
  t,
}: {
  status: CustomDomain["status"];
  t: ReturnType<typeof useLocale>["dict"]["dashboard_custom_domain"];
}) {
  const cfg: Record<
    CustomDomain["status"],
    { label: string; cls: string; icon: React.ReactNode }
  > = {
    pending: {
      label: t.status_pending,
      cls: "bg-amber-500/15 text-amber-700 dark:text-amber-300 border-amber-500/30",
      icon: <Clock className="h-3 w-3" />,
    },
    validated: {
      label: t.status_validated,
      cls: "bg-blue-500/15 text-blue-700 dark:text-blue-300 border-blue-500/30",
      icon: <Loader2 className="h-3 w-3 animate-spin" />,
    },
    active: {
      label: t.status_active,
      cls: "bg-emerald-500/15 text-emerald-700 dark:text-emerald-300 border-emerald-500/30",
      icon: <CheckCircle2 className="h-3 w-3" />,
    },
    failed: {
      label: t.status_failed,
      cls: "bg-red-500/15 text-red-700 dark:text-red-300 border-red-500/30",
      icon: <AlertTriangle className="h-3 w-3" />,
    },
  };
  const c = cfg[status];
  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium border ${c.cls}`}
    >
      {c.icon}
      {c.label}
    </span>
  );
}

function Instructions({
  domain,
  t,
}: {
  domain: CustomDomain;
  t: ReturnType<typeof useLocale>["dict"]["dashboard_custom_domain"];
}) {
  return (
    <div className="mt-4 rounded-md bg-muted/40 p-4 space-y-4 text-sm">
      <p className="font-medium">{t.instructions_heading}</p>
      <p className="text-xs text-muted-foreground">{t.instructions_intro}</p>
      <div>
        <p className="font-medium">{t.instructions_cname_label}</p>
        <p className="text-xs text-muted-foreground mt-1">{t.instructions_cname_value}</p>
      </div>
      <div>
        <p className="font-medium">{t.instructions_token_label}</p>
        <p className="text-xs text-muted-foreground mt-1">{t.instructions_token_value}</p>
        <pre className="mt-2 overflow-x-auto rounded border border-border/60 bg-background p-3 text-xs font-mono">
          <code>{`/.well-known/pingcast/${domain.validation_token}`}</code>
        </pre>
        <pre className="mt-2 overflow-x-auto rounded border border-border/60 bg-background p-3 text-xs font-mono">
          <code>{domain.validation_token}</code>
        </pre>
      </div>
      <p className="text-xs text-muted-foreground">{t.instructions_check}</p>
    </div>
  );
}

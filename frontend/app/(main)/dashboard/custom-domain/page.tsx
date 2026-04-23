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

type CustomDomain = components["schemas"]["CustomDomain"];

export default function CustomDomainPage() {
  const [domains, setDomains] = useState<CustomDomain[] | null>(null);
  const [hostname, setHostname] = useState("");
  const [busy, setBusy] = useState(false);

  const [reloadTick, setReloadTick] = useState(0);

  useEffect(() => {
    // Cancellation flag: if the component unmounts or the effect
    // re-runs before the fetch resolves, we must not setState on a
    // dead tree. React 19's set-state-in-effect rule requires this
    // guard even for async work.
    let cancelled = false;
    const load = async () => {
      const res = await fetch("/api/custom-domains", { credentials: "include" });
      if (!cancelled && res.ok) {
        setDomains((await res.json()) as CustomDomain[]);
      }
    };
    void load();
    // Status flips pending → validated → active on the server-side
    // worker's 60s cycle. Re-poll so the user sees progress without a
    // manual refresh.
    const t = setInterval(() => void load(), 15_000);
    return () => {
      cancelled = true;
      clearInterval(t);
    };
  }, [reloadTick]);

  // Legacy callers still invoke load() after mutations. Route those
  // through a reload-tick so we stay on a single effect-driven path.
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
        toast.error("Custom domains are a Pro feature.");
        return;
      }
      if (!res.ok) {
        const body = await res.json().catch(() => null);
        toast.error(body?.error?.message ?? `Request failed (HTTP ${res.status}).`);
        return;
      }
      setHostname("");
      toast.success("Domain registered. Follow the CNAME instructions below.");
      await load();
    } finally {
      setBusy(false);
    }
  }

  async function remove(id: number) {
    const res = await fetch(`/api/custom-domains/${id}`, {
      method: "DELETE",
      credentials: "include",
    });
    if (res.ok) {
      toast.success("Domain removed.");
      await load();
    } else {
      toast.error(`Delete failed (HTTP ${res.status}).`);
    }
  }

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
          <Globe className="h-5 w-5" />
        </div>
        <h1 className="text-2xl font-bold tracking-tight">Custom domain</h1>
      </div>
      <p className="mt-3 text-sm text-muted-foreground max-w-xl">
        Point <code>status.yourcompany.com</code> at PingCast via a CNAME +
        a one-time <code>.well-known</code> probe. We&apos;ll validate and
        issue a TLS cert automatically. Pro feature.
      </p>

      <form
        onSubmit={submit}
        className="mt-8 rounded-lg border border-border/60 bg-card p-5 space-y-4"
      >
        <div className="space-y-2">
          <Label htmlFor="hostname">Hostname</Label>
          <Input
            id="hostname"
            type="text"
            placeholder="status.yourcompany.com"
            value={hostname}
            onChange={(e) => setHostname(e.target.value)}
            required
            disabled={busy}
          />
          <p className="text-xs text-muted-foreground">
            Lowercase, no path, no port. Subdomains are fine; apex claims
            under our own domain are rejected.
          </p>
        </div>
        <Button type="submit" disabled={busy || !hostname}>
          {busy ? "Requesting…" : "Add domain"}
        </Button>
      </form>

      <section className="mt-10">
        <h2 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground mb-3">
          Your domains
        </h2>
        {domains === null ? (
          <p className="text-sm text-muted-foreground">Loading…</p>
        ) : domains.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            No domains registered yet. Add one above.
          </p>
        ) : (
          <ul className="space-y-4">
            {domains.map((d) => (
              <li key={d.id} className="rounded-lg border border-border/60 bg-card p-5">
                <div className="flex items-start justify-between gap-4 flex-wrap">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <code className="font-medium">{d.hostname}</code>
                      <StatusPill status={d.status} />
                    </div>
                    <p className="mt-1 text-xs text-muted-foreground">
                      Registered {new Date(d.created_at).toLocaleString()}
                    </p>
                  </div>
                  <button
                    onClick={() => remove(d.id)}
                    className="text-muted-foreground hover:text-destructive transition-colors"
                    aria-label={`Remove ${d.hostname}`}
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>

                {d.status === "pending" || d.status === "failed" ? (
                  <Instructions domain={d} />
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

function StatusPill({ status }: { status: CustomDomain["status"] }) {
  const cfg: Record<CustomDomain["status"], { label: string; cls: string; icon: React.ReactNode }> = {
    pending: {
      label: "Pending",
      cls: "bg-amber-500/15 text-amber-700 dark:text-amber-300 border-amber-500/30",
      icon: <Clock className="h-3 w-3" />,
    },
    validated: {
      label: "Validated · issuing cert",
      cls: "bg-blue-500/15 text-blue-700 dark:text-blue-300 border-blue-500/30",
      icon: <Loader2 className="h-3 w-3 animate-spin" />,
    },
    active: {
      label: "Active",
      cls: "bg-emerald-500/15 text-emerald-700 dark:text-emerald-300 border-emerald-500/30",
      icon: <CheckCircle2 className="h-3 w-3" />,
    },
    failed: {
      label: "Failed",
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

function Instructions({ domain }: { domain: CustomDomain }) {
  const edgeHost = "edge.pingcast.io"; // TODO: source from env once the edge is real
  return (
    <div className="mt-4 rounded-md bg-muted/40 p-4 space-y-4 text-sm">
      <div>
        <p className="font-medium">Step 1 — Set a CNAME at your DNS</p>
        <pre className="mt-2 overflow-x-auto rounded border border-border/60 bg-background p-3 text-xs font-mono">
          <code>{`CNAME  ${domain.hostname}  →  ${edgeHost}`}</code>
        </pre>
      </div>
      <div>
        <p className="font-medium">
          Step 2 — Serve this token at{" "}
          <code>/.well-known/pingcast/{domain.validation_token}</code>
        </p>
        <p className="mt-1 text-xs text-muted-foreground">
          Token body (just this string, no headers other than Content-Type:
          text/plain):
        </p>
        <pre className="mt-2 overflow-x-auto rounded border border-border/60 bg-background p-3 text-xs font-mono">
          <code>{domain.validation_token}</code>
        </pre>
        <p className="mt-2 text-xs text-muted-foreground">
          Our validation worker probes this every 60 seconds. As soon as
          it matches, status flips to <em>validated</em> and we request
          a TLS cert.
        </p>
      </div>
    </div>
  );
}

"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { ArrowLeft, Wrench, Trash2, CheckCircle2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { toast } from "sonner";
import type { components } from "@/lib/openapi-types";
import { useLocale } from "@/components/i18n/locale-provider";

type MaintenanceWindow = components["schemas"]["MaintenanceWindow"];
type Monitor = components["schemas"]["MonitorWithUptime"];

export default function MaintenanceDashboardPage() {
  const { dict, locale } = useLocale();
  const t = dict.dashboard_maintenance;
  const [windows, setWindows] = useState<MaintenanceWindow[] | null>(null);
  const [monitors, setMonitors] = useState<Monitor[] | null>(null);
  const [monitorId, setMonitorId] = useState("");
  const [startsAt, setStartsAt] = useState(defaultStartISO());
  const [endsAt, setEndsAt] = useState(defaultEndISO());
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [reloadTick, setReloadTick] = useState(0);
  const [now, setNow] = useState<number>(() => Date.now());

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const [wRes, mRes] = await Promise.all([
        fetch("/api/maintenance-windows", { credentials: "include" }),
        fetch("/api/monitors", { credentials: "include" }),
      ]);
      if (cancelled) return;
      if (wRes.ok) setWindows((await wRes.json()) as MaintenanceWindow[]);
      if (mRes.ok) setMonitors((await mRes.json()) as Monitor[]);
    })();
    return () => {
      cancelled = true;
    };
  }, [reloadTick]);

  useEffect(() => {
    const tm = setInterval(() => setNow(Date.now()), 60_000);
    return () => clearInterval(tm);
  }, []);

  const load = () => setReloadTick((n) => n + 1);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      const res = await fetch("/api/maintenance-windows", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          monitor_id: monitorId,
          starts_at: new Date(startsAt).toISOString(),
          ends_at: new Date(endsAt).toISOString(),
          reason,
        }),
        credentials: "include",
      });
      if (res.status === 402) {
        toast.error(t.pro_required);
        return;
      }
      if (!res.ok) {
        const body = await res.json().catch(() => null);
        toast.error(body?.error?.message ?? `${t.schedule_failed} (HTTP ${res.status}).`);
        return;
      }
      setReason("");
      toast.success(t.scheduled);
      load();
    } finally {
      setBusy(false);
    }
  }

  async function remove(id: number) {
    if (!confirm(t.delete_confirm)) return;
    const res = await fetch(`/api/maintenance-windows/${id}`, {
      method: "DELETE",
      credentials: "include",
    });
    if (res.ok) {
      toast.success(t.deleted);
      load();
    } else {
      toast.error(`${t.delete_failed} (HTTP ${res.status}).`);
    }
  }

  const dateLocale = locale === "ru" ? "ru-RU" : "en-US";

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
          <Wrench className="h-5 w-5" />
        </div>
        <h1 className="text-2xl font-bold tracking-tight">{t.title}</h1>
      </div>
      <p className="mt-3 text-sm text-muted-foreground max-w-xl">{t.subtitle}</p>

      <form
        onSubmit={submit}
        className="mt-8 rounded-lg border border-border/60 bg-card p-5 space-y-4"
      >
        <h2 className="text-sm font-semibold">{t.schedule_heading}</h2>
        <div className="space-y-2">
          <Label htmlFor="monitor">{t.monitor_label}</Label>
          <Select value={monitorId} onValueChange={(v) => v && setMonitorId(v)}>
            <SelectTrigger id="monitor">
              <SelectValue placeholder={t.monitor_label} />
            </SelectTrigger>
            <SelectContent>
              {(monitors ?? []).map((m) =>
                m.id ? (
                  <SelectItem key={m.id} value={m.id}>
                    {m.name}
                  </SelectItem>
                ) : null,
              )}
            </SelectContent>
          </Select>
        </div>

        <div className="grid sm:grid-cols-2 gap-3">
          <div className="space-y-2">
            <Label htmlFor="starts">{t.starts_label}</Label>
            <Input
              id="starts"
              type="datetime-local"
              value={startsAt}
              onChange={(e) => setStartsAt(e.target.value)}
              required
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="ends">{t.ends_label}</Label>
            <Input
              id="ends"
              type="datetime-local"
              value={endsAt}
              onChange={(e) => setEndsAt(e.target.value)}
              required
            />
          </div>
        </div>

        <div className="space-y-2">
          <Label htmlFor="reason">{t.reason_label}</Label>
          <Textarea
            id="reason"
            placeholder={t.reason_placeholder}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            required
            maxLength={500}
            rows={2}
          />
        </div>

        <Button type="submit" disabled={busy || !monitorId || !reason}>
          {busy ? t.scheduling : t.schedule}
        </Button>
      </form>

      <section className="mt-10">
        <h2 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground mb-3">
          {t.list_heading}
        </h2>
        {windows === null ? (
          <p className="text-sm text-muted-foreground">{dict.common.loading}</p>
        ) : windows.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t.list_empty}</p>
        ) : (
          <ul className="space-y-3">
            {windows.map((w) => {
              const start = new Date(w.starts_at).getTime();
              const end = new Date(w.ends_at).getTime();
              const active = now >= start && now < end;
              const past = now >= end;
              return (
                <li
                  key={w.id}
                  className="rounded-lg border border-border/60 bg-card p-4"
                >
                  <div className="flex items-start justify-between gap-4 flex-wrap">
                    <div className="min-w-0">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="font-medium truncate">{w.reason}</span>
                        {active ? (
                          <span className="inline-flex items-center gap-1 rounded-full bg-amber-500/15 text-amber-700 dark:text-amber-300 border border-amber-500/30 px-2 py-0.5 text-[11px] font-medium">
                            {dict.common.active}
                          </span>
                        ) : past ? (
                          <span className="inline-flex items-center gap-1 rounded-full bg-muted text-muted-foreground border px-2 py-0.5 text-[11px] font-medium">
                            <CheckCircle2 className="h-3 w-3" />
                          </span>
                        ) : (
                          <span className="inline-flex items-center gap-1 rounded-full bg-blue-500/15 text-blue-700 dark:text-blue-300 border border-blue-500/30 px-2 py-0.5 text-[11px] font-medium">
                            {dict.common.pending}
                          </span>
                        )}
                      </div>
                      <div className="text-xs text-muted-foreground mt-1">
                        {new Date(w.starts_at).toLocaleString(dateLocale)} →{" "}
                        {new Date(w.ends_at).toLocaleString(dateLocale)}
                      </div>
                    </div>
                    <button
                      onClick={() => remove(w.id)}
                      className="text-muted-foreground hover:text-destructive shrink-0"
                      aria-label={dict.common.delete}
                    >
                      <Trash2 className="h-4 w-4" />
                    </button>
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </section>
    </div>
  );
}

function defaultStartISO() {
  const d = new Date(Date.now() + 10 * 60 * 1000);
  return isoLocal(d);
}

function defaultEndISO() {
  const d = new Date(Date.now() + 70 * 60 * 1000);
  return isoLocal(d);
}

function isoLocal(d: Date) {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

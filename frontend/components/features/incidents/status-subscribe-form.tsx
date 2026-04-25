"use client";

import { useState } from "react";
import { Mail, CheckCircle2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

// Inline subscribe form on the public /status/[slug] page. Posts to
// /api/status/:slug/subscribe with the visitor's locale (sourced from
// Accept-Language by the server). The backend stores the locale and
// uses it for both the confirmation email and every future incident
// notification, so each subscriber gets their language without any
// per-tenant config.
//
// Strings come from the server-side dict via `labels` props because
// /status/[slug] lives outside app/[lang]/ — the LocaleProvider isn't
// in scope on canonical status URLs (status.customer.com etc).
type Labels = {
  heading: string;
  placeholder: string;
  button: string;
  busy: string;
  helper: string;
  sentHeading: string;
  sentBody: string;
  failed: string;
};

export function StatusSubscribeForm({
  slug,
  locale,
  labels,
  accentStyle,
}: {
  slug: string;
  locale: string;
  labels: Labels;
  accentStyle?: React.CSSProperties;
}) {
  const [email, setEmail] = useState("");
  const [busy, setBusy] = useState(false);
  const [sent, setSent] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(
        `/api/status/${encodeURIComponent(slug)}/subscribe`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ email, locale }),
        },
      );
      if (!res.ok) {
        const body = await res.json().catch(() => null);
        throw new Error(body?.error?.message ?? `HTTP ${res.status}`);
      }
      setSent(true);
      setEmail("");
    } catch (e) {
      setError(e instanceof Error ? e.message : labels.failed);
    } finally {
      setBusy(false);
    }
  }

  if (sent) {
    return (
      <div className="rounded-lg border border-emerald-500/40 bg-emerald-500/5 p-4 flex items-start gap-3 text-sm">
        <CheckCircle2 className="h-4 w-4 shrink-0 mt-0.5 text-emerald-500" />
        <div>
          <p className="font-medium">{labels.sentHeading}</p>
          <p className="mt-1 text-muted-foreground">{labels.sentBody}</p>
        </div>
      </div>
    );
  }

  return (
    <form
      onSubmit={submit}
      className="rounded-lg border border-border/60 bg-card p-4"
    >
      <div className="flex items-center gap-2 mb-3 text-sm font-medium">
        <Mail className="h-4 w-4 text-muted-foreground" />
        {labels.heading}
      </div>
      <div className="flex flex-col sm:flex-row gap-2">
        <Input
          type="email"
          placeholder={labels.placeholder}
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
          disabled={busy}
          className="flex-1"
        />
        <Button type="submit" disabled={busy || !email} style={accentStyle}>
          {busy ? labels.busy : labels.button}
        </Button>
      </div>
      <p className="mt-2 text-xs text-muted-foreground">{labels.helper}</p>
      {error ? <p className="mt-2 text-xs text-destructive">{error}</p> : null}
    </form>
  );
}

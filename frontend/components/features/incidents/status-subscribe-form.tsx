"use client";

import { useState } from "react";
import { Mail, CheckCircle2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

// Inline subscribe form on the public /status/[slug] page. Posts to
// /api/status/:slug/subscribe; the backend sends a double-opt-in email
// and returns 202. On success we show a "check your inbox" message —
// vague by design so the endpoint can't enumerate which emails are
// already subscribed to which slug.
export function StatusSubscribeForm({
  slug,
  accentStyle,
}: {
  slug: string;
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
          body: JSON.stringify({ email }),
        },
      );
      if (!res.ok) {
        const body = await res.json().catch(() => null);
        throw new Error(body?.error?.message ?? `HTTP ${res.status}`);
      }
      setSent(true);
      setEmail("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Subscription failed.");
    } finally {
      setBusy(false);
    }
  }

  if (sent) {
    return (
      <div className="rounded-lg border border-emerald-500/40 bg-emerald-500/5 p-4 flex items-start gap-3 text-sm">
        <CheckCircle2 className="h-4 w-4 shrink-0 mt-0.5 text-emerald-500" />
        <div>
          <p className="font-medium">Check your inbox.</p>
          <p className="mt-1 text-muted-foreground">
            If that email isn&apos;t already subscribed, you&apos;ll get a
            confirmation link within a minute. Click it once and
            you&apos;re on the list.
          </p>
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
        Get email updates on incidents
      </div>
      <div className="flex flex-col sm:flex-row gap-2">
        <Input
          type="email"
          placeholder="you@example.com"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
          disabled={busy}
          className="flex-1"
        />
        <Button type="submit" disabled={busy || !email} style={accentStyle}>
          {busy ? "Subscribing…" : "Subscribe"}
        </Button>
      </div>
      <p className="mt-2 text-xs text-muted-foreground">
        Double opt-in. Unsubscribe in one click from any email.
      </p>
      {error ? (
        <p className="mt-2 text-xs text-destructive">{error}</p>
      ) : null}
    </form>
  );
}

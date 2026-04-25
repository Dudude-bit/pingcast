"use client";

import { useState } from "react";
import { useLocale } from "@/components/i18n/locale-provider";
import { track } from "@/lib/analytics";

// NewsletterForm posts to /api/newsletter/subscribe with the optional
// `source` tag so we can tell which placement converts. Returns 202 on
// any syntactically valid email (double-opt-in confirmation arrives via
// email). UI strings come from the locale context — every placement
// renders in the visitor's language without prop-drilling.
type Props = {
  source: string;
};

export function NewsletterForm({ source }: Props) {
  const { dict, locale } = useLocale();
  const f = dict.footer;
  const [email, setEmail] = useState("");
  const [status, setStatus] = useState<"idle" | "sending" | "ok" | "error">(
    "idle",
  );
  const [message, setMessage] = useState("");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!email) return;
    setStatus("sending");
    try {
      const res = await fetch("/api/newsletter/subscribe", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, source, locale }),
      });
      if (res.status === 202) {
        setStatus("ok");
        setMessage(f.newsletter_ok);
        setEmail("");
        track("newsletter_subscribed", { source, lang: locale });
      } else {
        const body = await res.json().catch(() => ({}));
        setStatus("error");
        setMessage(body?.error?.message ?? dict.common.error_generic);
      }
    } catch {
      setStatus("error");
      setMessage(dict.common.network_error);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-2">
      <div className="flex flex-col sm:flex-row gap-2">
        <input
          type="email"
          required
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          placeholder={f.newsletter_placeholder}
          disabled={status === "sending"}
          aria-label={f.newsletter_label}
          className="flex-1 rounded-md border border-border/60 bg-background px-3 py-2 text-sm placeholder:text-muted-foreground/60 focus:outline-none focus:ring-2 focus:ring-primary/40 disabled:opacity-60"
        />
        <button
          type="submit"
          disabled={status === "sending" || !email}
          className="rounded-md bg-foreground text-background px-4 py-2 text-sm font-medium hover:bg-foreground/90 disabled:opacity-60"
        >
          {status === "sending" ? "..." : f.newsletter_button}
        </button>
      </div>
      {status === "ok" && <p className="text-xs text-emerald-600">{message}</p>}
      {status === "error" && (
        <p className="text-xs text-destructive">{message}</p>
      )}
      {status === "idle" && (
        <p className="text-xs text-muted-foreground">{f.newsletter_helper}</p>
      )}
    </form>
  );
}

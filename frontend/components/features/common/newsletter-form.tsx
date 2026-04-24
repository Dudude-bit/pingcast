"use client";

import { useState } from "react";

// NewsletterForm posts to /api/newsletter/subscribe with the optional
// `source` tag so we can tell which placement converts. Returns 202 on
// any syntactically valid email (double-opt-in confirmation arrives via
// email). The UI stays minimal — one input, one button, inline status.
type Props = {
  source: string;
  placeholder?: string;
  label?: string;
};

export function NewsletterForm({
  source,
  placeholder = "you@company.com",
  label = "Subscribe",
}: Props) {
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
        body: JSON.stringify({ email, source }),
      });
      if (res.status === 202) {
        setStatus("ok");
        setMessage("Check your inbox to confirm.");
        setEmail("");
      } else {
        const body = await res.json().catch(() => ({}));
        setStatus("error");
        setMessage(
          body?.error?.message ??
            "Something went wrong. Try again in a moment.",
        );
      }
    } catch {
      setStatus("error");
      setMessage("Network error. Try again.");
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
          placeholder={placeholder}
          disabled={status === "sending"}
          aria-label="Email address for newsletter"
          className="flex-1 rounded-md border border-border/60 bg-background px-3 py-2 text-sm placeholder:text-muted-foreground/60 focus:outline-none focus:ring-2 focus:ring-primary/40 disabled:opacity-60"
        />
        <button
          type="submit"
          disabled={status === "sending" || !email}
          className="rounded-md bg-foreground text-background px-4 py-2 text-sm font-medium hover:bg-foreground/90 disabled:opacity-60"
        >
          {status === "sending" ? "..." : label}
        </button>
      </div>
      {status === "ok" && (
        <p className="text-xs text-emerald-600">{message}</p>
      )}
      {status === "error" && (
        <p className="text-xs text-destructive">{message}</p>
      )}
      {status === "idle" && (
        <p className="text-xs text-muted-foreground">
          1-2 emails a month. Unsubscribe in one click.
        </p>
      )}
    </form>
  );
}

"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { ArrowLeft, Sparkles, AlertTriangle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { toast } from "sonner";
import type { components } from "@/lib/openapi-types";

type Branding = components["schemas"]["Branding"];

export default function BrandingPage() {
  const [loaded, setLoaded] = useState(false);
  const [logoUrl, setLogoUrl] = useState("");
  const [accentColor, setAccentColor] = useState("#3b82f6");
  const [footer, setFooter] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    fetch("/api/me/branding", { credentials: "include" })
      .then((r) => (r.ok ? r.json() : null))
      .then((body: Branding | null) => {
        if (body) {
          setLogoUrl(body.logo_url ?? "");
          setAccentColor(body.accent_color ?? "#3b82f6");
          setFooter(body.custom_footer_text ?? "");
        }
        setLoaded(true);
      })
      .catch(() => setLoaded(true));
  }, []);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      const body: Branding = {
        logo_url: logoUrl.trim() || null,
        accent_color: accentColor || null,
        custom_footer_text: footer.trim() || null,
      };
      const res = await fetch("/api/me/branding", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
        credentials: "include",
      });
      if (res.status === 402) {
        toast.error(
          "Branding is a Pro feature. Upgrade from the dashboard to edit.",
        );
        return;
      }
      if (!res.ok) {
        toast.error(`Save failed (HTTP ${res.status}).`);
        return;
      }
      toast.success("Branding saved.");
    } finally {
      setBusy(false);
    }
  }

  if (!loaded) return null;

  return (
    <div className="container mx-auto px-4 py-12 max-w-2xl">
      <Link
        href="/dashboard"
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mb-6"
      >
        <ArrowLeft className="h-3.5 w-3.5" /> Back to dashboard
      </Link>

      <div className="flex items-center gap-3">
        <div className="inline-flex h-9 w-9 items-center justify-center rounded-md bg-primary/10 text-primary">
          <Sparkles className="h-5 w-5" />
        </div>
        <h1 className="text-2xl font-bold tracking-tight">Status-page branding</h1>
      </div>
      <p className="mt-3 text-sm text-muted-foreground">
        Pro-tier customisation for your public status page at{" "}
        <code>/status/&lt;your-slug&gt;</code>. Free users can preview here —
        values are stored, but the renderer ignores them until you upgrade.
      </p>

      <form
        onSubmit={submit}
        className="mt-8 space-y-5 rounded-lg border border-border/60 bg-card p-6"
      >
        <div className="space-y-2">
          <Label htmlFor="logo">Logo URL</Label>
          <Input
            id="logo"
            type="url"
            placeholder="https://yourcompany.com/logo.svg"
            value={logoUrl}
            onChange={(e) => setLogoUrl(e.target.value)}
          />
          <p className="text-xs text-muted-foreground">
            SVG or transparent PNG, hosted on your own domain. Rendered at
            48px height.
          </p>
        </div>

        <div className="space-y-2">
          <Label htmlFor="accent">Accent colour</Label>
          <div className="flex items-center gap-3">
            <Input
              id="accent"
              type="color"
              value={accentColor}
              onChange={(e) => setAccentColor(e.target.value)}
              className="w-20 h-10 p-1 cursor-pointer"
            />
            <Input
              type="text"
              value={accentColor}
              onChange={(e) => setAccentColor(e.target.value)}
              placeholder="#3b82f6"
              className="flex-1 font-mono"
            />
          </div>
          <p className="text-xs text-muted-foreground">
            Rendered as the <code>--brand-accent</code> CSS variable on your
            status page.
          </p>
        </div>

        <div className="space-y-2">
          <Label htmlFor="footer">Custom footer text</Label>
          <Textarea
            id="footer"
            rows={3}
            placeholder="Need support? Email help@yourcompany.com"
            value={footer}
            onChange={(e) => setFooter(e.target.value)}
          />
          <p className="text-xs text-muted-foreground">
            Replaces the &quot;Powered by PingCast&quot; watermark on your Pro
            status page.
          </p>
        </div>

        <div className="flex items-center gap-3 pt-2">
          <Button type="submit" disabled={busy}>
            {busy ? "Saving…" : "Save branding"}
          </Button>
          <Link
            href="/status/your-slug"
            className="text-sm text-muted-foreground hover:text-foreground underline underline-offset-4"
            target="_blank"
          >
            Preview status page →
          </Link>
        </div>
      </form>

      <div className="mt-6 flex items-start gap-3 rounded-md border border-amber-500/40 bg-amber-500/5 px-4 py-3 text-sm">
        <AlertTriangle className="h-4 w-4 shrink-0 mt-0.5 text-amber-600 dark:text-amber-400" />
        <p className="text-muted-foreground">
          Branding is a <strong>Pro</strong> feature. Save still works on
          Free, but the public status page ignores the values until you
          upgrade.
        </p>
      </div>
    </div>
  );
}

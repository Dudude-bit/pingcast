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
import { useLocale } from "@/components/i18n/locale-provider";

type Branding = components["schemas"]["Branding"];

export default function BrandingPage() {
  const { dict, locale } = useLocale();
  const t = dict.dashboard_branding;
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
        toast.error(t.pro_required);
        return;
      }
      if (!res.ok) {
        toast.error(`${t.save_failed} (HTTP ${res.status}).`);
        return;
      }
      toast.success(t.saved);
    } finally {
      setBusy(false);
    }
  }

  if (!loaded) return null;

  return (
    <div className="container mx-auto px-4 py-12 max-w-2xl">
      <Link
        href={`/${locale}/dashboard`}
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mb-6"
      >
        <ArrowLeft className="h-3.5 w-3.5" /> {dict.common.back_to_dashboard}
      </Link>

      <div className="flex items-center gap-3">
        <div className="inline-flex h-9 w-9 items-center justify-center rounded-md bg-primary/10 text-primary">
          <Sparkles className="h-5 w-5" />
        </div>
        <h1 className="text-2xl font-bold tracking-tight">{t.title}</h1>
      </div>
      <p className="mt-3 text-sm text-muted-foreground">{t.subtitle}</p>

      <form
        onSubmit={submit}
        className="mt-8 space-y-5 rounded-lg border border-border/60 bg-card p-6"
      >
        <div className="space-y-2">
          <Label htmlFor="logo">{t.logo_label}</Label>
          <Input
            id="logo"
            type="url"
            placeholder="https://yourcompany.com/logo.svg"
            value={logoUrl}
            onChange={(e) => setLogoUrl(e.target.value)}
          />
          <p className="text-xs text-muted-foreground">{t.logo_help}</p>
        </div>

        <div className="space-y-2">
          <Label htmlFor="accent">{t.accent_label}</Label>
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
          <p className="text-xs text-muted-foreground">{t.accent_help}</p>
        </div>

        <div className="space-y-2">
          <Label htmlFor="footer">{t.footer_label}</Label>
          <Textarea
            id="footer"
            rows={3}
            placeholder="Need support? Email help@yourcompany.com"
            value={footer}
            onChange={(e) => setFooter(e.target.value)}
          />
          <p className="text-xs text-muted-foreground">{t.footer_help}</p>
        </div>

        <div className="flex items-center gap-3 pt-2">
          <Button type="submit" disabled={busy}>
            {busy ? t.saving : t.save}
          </Button>
          <Link
            href={`/status/your-slug`}
            className="text-sm text-muted-foreground hover:text-foreground underline underline-offset-4"
            target="_blank"
          >
            {t.preview_open}
          </Link>
        </div>
      </form>

      <div className="mt-6 flex items-start gap-3 rounded-md border border-amber-500/40 bg-amber-500/5 px-4 py-3 text-sm">
        <AlertTriangle className="h-4 w-4 shrink-0 mt-0.5 text-amber-600 dark:text-amber-400" />
        <p className="text-muted-foreground">
          <strong>{t.pro_required}</strong> {t.pro_required_sub}
        </p>
      </div>
    </div>
  );
}

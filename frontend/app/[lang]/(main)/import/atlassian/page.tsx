"use client";

import { useState } from "react";
import Link from "next/link";
import { ArrowLeft, UploadCloud, CheckCircle2, AlertTriangle } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { components } from "@/lib/openapi-types";
import { useLocale } from "@/components/i18n/locale-provider";

type ImportResult = components["schemas"]["AtlassianImportResult"];

export default function AtlassianImportPage() {
  const { dict, locale } = useLocale();
  const t = dict.import_atlassian;
  const [file, setFile] = useState<File | null>(null);
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<ImportResult | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!file) return;
    setBusy(true);
    setError(null);
    setResult(null);
    try {
      const json = await file.text();
      const res = await fetch("/api/import/atlassian", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: json,
        credentials: "include",
      });
      if (res.status === 402) {
        throw new Error(t.pro_required);
      }
      if (!res.ok) {
        const body = await res.json().catch(() => null);
        throw new Error(
          body?.error?.message ?? `${t.import_failed} (HTTP ${res.status}).`,
        );
      }
      setResult((await res.json()) as ImportResult);
    } catch (e) {
      setError(e instanceof Error ? e.message : t.import_failed);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="container mx-auto px-4 py-12 max-w-2xl">
      <Link
        href={`/${locale}/dashboard`}
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mb-6"
      >
        <ArrowLeft className="h-3.5 w-3.5" /> {dict.common.back_to_dashboard}
      </Link>

      <h1 className="text-3xl font-bold tracking-tight">{t.title}</h1>
      <p className="mt-3 text-sm text-muted-foreground leading-relaxed">{t.subtitle}</p>

      <form
        onSubmit={submit}
        className="mt-8 rounded-lg border border-border/60 bg-card p-6 space-y-4"
      >
        <label
          htmlFor="atlassian-export"
          className="flex flex-col items-center justify-center gap-3 rounded-md border-2 border-dashed border-border/60 bg-muted/20 px-6 py-10 cursor-pointer hover:bg-muted/30 transition-colors"
        >
          <UploadCloud className="h-8 w-8 text-muted-foreground" />
          <div className="text-sm text-center">
            {file ? (
              <>
                <span className="font-medium">{file.name}</span>
                <span className="text-muted-foreground">
                  {" · "}
                  {(file.size / 1024).toFixed(1)} KB
                </span>
              </>
            ) : (
              <>
                <span className="font-medium">{t.drop_label}</span>
                <span className="text-muted-foreground block text-xs mt-1">
                  {t.drop_label_helper}
                </span>
              </>
            )}
          </div>
          <input
            id="atlassian-export"
            type="file"
            accept="application/json,.json"
            className="sr-only"
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
          />
        </label>

        <Button type="submit" disabled={!file || busy} className="w-full">
          {busy ? t.importing : t.import}
        </Button>
      </form>

      {error ? (
        <div className="mt-6 flex items-start gap-3 rounded-md border border-destructive/40 bg-destructive/5 px-4 py-3 text-sm text-destructive">
          <AlertTriangle className="h-4 w-4 shrink-0 mt-0.5" />
          <p>{error}</p>
        </div>
      ) : null}

      {result ? (
        <div className="mt-6 flex items-start gap-3 rounded-md border border-emerald-500/40 bg-emerald-500/5 px-4 py-3 text-sm">
          <CheckCircle2 className="h-4 w-4 shrink-0 mt-0.5 text-emerald-500" />
          <div>
            <p className="font-medium">{t.imported}</p>
            <p className="mt-1 text-muted-foreground">
              {t.result_summary
                .replace("{monitors}", String(result.monitors_created))
                .replace(
                  "{monitors_word}",
                  result.monitors_created === 1 ? t.monitors_one : t.monitors_many,
                )
                .replace("{incidents}", String(result.incidents_created))
                .replace(
                  "{incidents_word}",
                  result.incidents_created === 1 ? t.incidents_one : t.incidents_many,
                )
                .replace("{updates}", String(result.updates_created))}
              {result.components_skipped > 0
                ? (result.components_skipped === 1
                    ? t.result_skipped_one
                    : t.result_skipped_many
                  ).replace("{n}", String(result.components_skipped))
                : ""}
            </p>
            <Link
              href={`/${locale}/dashboard`}
              className="mt-2 inline-block text-sm underline underline-offset-4 hover:text-foreground"
            >
              {t.back_link}
            </Link>
          </div>
        </div>
      ) : null}
    </div>
  );
}

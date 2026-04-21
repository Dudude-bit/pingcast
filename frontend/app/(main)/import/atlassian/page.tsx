"use client";

import { useState } from "react";
import Link from "next/link";
import { ArrowLeft, UploadCloud, CheckCircle2, AlertTriangle } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { components } from "@/lib/openapi-types";

type ImportResult = components["schemas"]["AtlassianImportResult"];

export default function AtlassianImportPage() {
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
        throw new Error(
          "Atlassian import is a Pro feature. Upgrade from the dashboard and try again.",
        );
      }
      if (!res.ok) {
        const body = await res.json().catch(() => null);
        throw new Error(
          body?.error?.message ?? `Import failed (HTTP ${res.status}).`,
        );
      }
      setResult((await res.json()) as ImportResult);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Import failed.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="container mx-auto px-4 py-12 max-w-2xl">
      <Link
        href="/dashboard"
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mb-6"
      >
        <ArrowLeft className="h-3.5 w-3.5" /> Back to dashboard
      </Link>

      <h1 className="text-3xl font-bold tracking-tight">
        Import from Atlassian Statuspage
      </h1>
      <p className="mt-3 text-sm text-muted-foreground leading-relaxed">
        Upload the JSON export from your Statuspage admin. We&apos;ll create
        equivalent monitors, incidents (with full state history), and
        preserved update timelines inside one atomic transaction. Components
        without a probe URL are skipped.
      </p>

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
                  {" "}
                  · {(file.size / 1024).toFixed(1)} KB
                </span>
              </>
            ) : (
              <>
                <span className="font-medium">Choose a JSON export</span>
                <span className="text-muted-foreground block text-xs mt-1">
                  (schema_version &quot;1.0&quot;)
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
          {busy ? "Importing…" : "Import"}
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
            <p className="font-medium">Import complete.</p>
            <p className="mt-1 text-muted-foreground">
              {result.monitors_created} monitor
              {result.monitors_created === 1 ? "" : "s"},{" "}
              {result.incidents_created} incident
              {result.incidents_created === 1 ? "" : "s"},{" "}
              {result.updates_created} timeline entries.
              {result.components_skipped > 0
                ? ` ${result.components_skipped} component${result.components_skipped === 1 ? "" : "s"} skipped (no probe URL).`
                : ""}
            </p>
            <Link
              href="/dashboard"
              className="mt-2 inline-block text-sm underline underline-offset-4 hover:text-foreground"
            >
              Back to dashboard →
            </Link>
          </div>
        </div>
      ) : null}
    </div>
  );
}

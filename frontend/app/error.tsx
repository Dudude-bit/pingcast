"use client";

import { useEffect, useMemo } from "react";
import Link from "next/link";
import { AlertTriangle } from "lucide-react";
import { Button, buttonVariants } from "@/components/ui/button";
import enDict from "@/dictionaries/en.json";
import ruDict from "@/dictionaries/ru.json";

// Root error boundary — sits outside any layout so it can't read the
// LocaleProvider context. We pick the locale client-side from
// navigator.language and fall through to EN. Both dictionaries are
// imported statically because dynamic import in an error boundary
// would itself be a failure point.
const DICTS = { en: enDict, ru: ruDict } as const;

function pickLang(): "en" | "ru" {
  if (typeof navigator === "undefined") return "en";
  const primary = (navigator.language ?? "en").split("-")[0]!.toLowerCase();
  return primary === "ru" ? "ru" : "en";
}

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error(error);
  }, [error]);

  const lang = useMemo(() => pickLang(), []);
  const t = DICTS[lang].errors;

  return (
    <div className="container mx-auto px-4 py-24 max-w-md text-center">
      <AlertTriangle className="mx-auto h-10 w-10 text-red-500" />
      <h1 className="mt-4 text-2xl font-bold tracking-tight">{t.error_title}</h1>
      <p className="mt-2 text-sm text-muted-foreground">{t.error_body}</p>
      <div className="mt-6 flex items-center justify-center gap-3">
        <Button onClick={reset}>{t.error_retry}</Button>
        <Link href={`/${lang}`} className={buttonVariants({ variant: "ghost" })}>
          {t.error_home}
        </Link>
      </div>
      {error.digest ? (
        <p className="mt-6 text-xs text-muted-foreground font-mono">
          {t.error_id}: {error.digest}
        </p>
      ) : null}
    </div>
  );
}

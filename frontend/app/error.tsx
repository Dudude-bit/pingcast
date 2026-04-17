"use client";

import { useEffect } from "react";
import Link from "next/link";
import { AlertTriangle } from "lucide-react";
import { Button, buttonVariants } from "@/components/ui/button";

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

  return (
    <div className="container mx-auto px-4 py-24 max-w-md text-center">
      <AlertTriangle className="mx-auto h-10 w-10 text-red-500" />
      <h1 className="mt-4 text-2xl font-bold tracking-tight">
        Something went wrong
      </h1>
      <p className="mt-2 text-sm text-muted-foreground">
        An unexpected error occurred. Please try again in a moment.
      </p>
      <div className="mt-6 flex items-center justify-center gap-3">
        <Button onClick={reset}>Try again</Button>
        <Link href="/" className={buttonVariants({ variant: "ghost" })}>
          Go home
        </Link>
      </div>
      {error.digest ? (
        <p className="mt-6 text-xs text-muted-foreground font-mono">
          Error ID: {error.digest}
        </p>
      ) : null}
    </div>
  );
}

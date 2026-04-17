"use client";

import { useState } from "react";
import { useAPIKeys } from "@/lib/queries";
import { Skeleton } from "@/components/ui/skeleton";
import { CreateAPIKeyForm } from "@/components/features/api-keys/create-api-key-form";
import { RevealKeyDialog } from "@/components/features/api-keys/reveal-key-dialog";
import { APIKeyRow } from "@/components/features/api-keys/api-key-row";
import { KeyRound } from "lucide-react";

export default function APIKeysPage() {
  const { data, isLoading, error } = useAPIKeys();
  const [revealedKey, setRevealedKey] = useState<string | null>(null);

  return (
    <div className="container mx-auto px-4 py-8 max-w-2xl space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">API keys</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Authenticate programmatic access. Use the{" "}
          <code className="font-mono text-xs bg-muted px-1 py-0.5 rounded">
            Authorization: Bearer ...
          </code>{" "}
          header.
        </p>
      </div>

      <CreateAPIKeyForm onCreated={setRevealedKey} />

      <div>
        <h2 className="text-base font-semibold mb-3">Your keys</h2>
        {isLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-20 w-full rounded-lg" />
            <Skeleton className="h-20 w-full rounded-lg" />
          </div>
        ) : error ? (
          <div className="rounded-lg border border-red-500/30 bg-red-500/5 p-6 text-sm text-red-700 dark:text-red-400">
            Failed to load keys: {error.message}
          </div>
        ) : !data || data.length === 0 ? (
          <div className="rounded-lg border border-dashed border-border/60 bg-card py-12 px-6 text-center">
            <KeyRound className="mx-auto h-8 w-8 text-muted-foreground/60" />
            <p className="mt-3 text-sm text-muted-foreground">
              No API keys yet. Create your first key above.
            </p>
          </div>
        ) : (
          <div className="space-y-2">
            {data.map((k) => (
              <APIKeyRow key={k.id} k={k} />
            ))}
          </div>
        )}
      </div>

      <RevealKeyDialog
        rawKey={revealedKey}
        onClose={() => setRevealedKey(null)}
      />
    </div>
  );
}

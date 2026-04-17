"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Trash2, KeyRound } from "lucide-react";
import type { APIKey } from "@/lib/queries";
import { ConfirmDestructiveDialog } from "@/components/features/common/confirm-destructive-dialog";
import { useRevokeAPIKey } from "@/lib/mutations";

function formatDate(iso?: string | null) {
  if (!iso) return "never";
  return new Date(iso).toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

export function APIKeyRow({ k }: { k: APIKey }) {
  const [revokeOpen, setRevokeOpen] = useState(false);
  const revoke = useRevokeAPIKey();

  return (
    <div className="flex items-start gap-4 rounded-lg border border-border/60 bg-card px-4 py-3">
      <div className="mt-0.5 text-muted-foreground">
        <KeyRound className="h-4 w-4" />
      </div>
      <div className="flex-1 min-w-0">
        <div className="font-medium truncate">{k.name}</div>
        <div className="mt-1 flex flex-wrap gap-1">
          {k.scopes?.map((s) => (
            <span
              key={s}
              className="text-xs font-mono px-1.5 py-0.5 rounded bg-muted text-muted-foreground"
            >
              {s}
            </span>
          ))}
        </div>
        <div className="mt-2 text-xs text-muted-foreground">
          Created {formatDate(k.created_at)} · Last used {formatDate(k.last_used_at)}
          {k.expires_at ? <> · Expires {formatDate(k.expires_at)}</> : null}
        </div>
      </div>
      <Button
        variant="ghost"
        size="icon-sm"
        onClick={() => setRevokeOpen(true)}
        aria-label="Revoke key"
        className="text-red-600 hover:bg-red-500/10"
      >
        <Trash2 className="h-4 w-4" />
      </Button>

      <ConfirmDestructiveDialog
        open={revokeOpen}
        onOpenChange={setRevokeOpen}
        title="Revoke API key?"
        description={
          <>
            {k.name}
            <br />
            Requests using this key will fail immediately. This cannot be undone.
          </>
        }
        confirmLabel="Revoke"
        pendingLabel="Revoking…"
        pending={revoke.isPending}
        onConfirm={async () => {
          if (k.id) await revoke.mutateAsync(k.id);
          setRevokeOpen(false);
        }}
      />
    </div>
  );
}

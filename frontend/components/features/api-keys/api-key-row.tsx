"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Trash2, KeyRound } from "lucide-react";
import type { APIKey } from "@/lib/queries";
import { ConfirmDestructiveDialog } from "@/components/features/common/confirm-destructive-dialog";
import { useRevokeAPIKey } from "@/lib/mutations";
import { useLocale } from "@/components/i18n/locale-provider";

export function APIKeyRow({ k }: { k: APIKey }) {
  const { dict, locale } = useLocale();
  const t = dict.api_keys;
  const [revokeOpen, setRevokeOpen] = useState(false);
  const revoke = useRevokeAPIKey();

  function formatDate(iso?: string | null) {
    if (!iso) return t.never_used;
    return new Date(iso).toLocaleDateString(locale === "ru" ? "ru-RU" : "en-US", {
      year: "numeric",
      month: "short",
      day: "numeric",
    });
  }

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
          {t.created_at} {formatDate(k.created_at)} · {t.last_used}{" "}
          {formatDate(k.last_used_at)}
          {k.expires_at ? (
            <>
              {" · "}
              {t.expires} {formatDate(k.expires_at)}
            </>
          ) : null}
        </div>
      </div>
      <Button
        variant="ghost"
        size="icon-sm"
        onClick={() => setRevokeOpen(true)}
        aria-label={t.row_revoke_label}
        className="text-red-600 hover:bg-red-500/10"
      >
        <Trash2 className="h-4 w-4" />
      </Button>

      <ConfirmDestructiveDialog
        open={revokeOpen}
        onOpenChange={setRevokeOpen}
        title={t.revoke_dialog_title}
        description={t.revoke_dialog_body.replace("{name}", k.name ?? "")}
        confirmLabel={t.revoke}
        pendingLabel={t.revoking}
        pending={revoke.isPending}
        onConfirm={async () => {
          if (k.id) await revoke.mutateAsync(k.id);
          setRevokeOpen(false);
        }}
      />
    </div>
  );
}

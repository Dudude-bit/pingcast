"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { buttonVariants } from "@/components/ui/button";
import { MoreHorizontal, Pencil, Trash2 } from "lucide-react";
import type { Channel } from "@/lib/queries";
import { StatusDot } from "@/components/features/monitors/status-badge";
import { ConfirmDestructiveDialog } from "@/components/features/common/confirm-destructive-dialog";
import { useDeleteChannel } from "@/lib/mutations";

export function ChannelRow({ ch }: { ch: Channel }) {
  const router = useRouter();
  const [deleteOpen, setDeleteOpen] = useState(false);
  const del = useDeleteChannel();

  return (
    <div className="group flex items-center gap-4 rounded-lg border border-border/60 bg-card px-4 py-3 hover:border-border hover:bg-accent/30 transition-colors">
      <StatusDot status={ch.is_enabled ? "up" : "unknown"} />

      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="font-medium truncate">{ch.name}</span>
          {!ch.is_enabled ? (
            <span className="text-xs px-1.5 py-0.5 rounded bg-zinc-500/15 text-zinc-600 dark:text-zinc-400">
              disabled
            </span>
          ) : null}
        </div>
        <div className="text-xs text-muted-foreground capitalize mt-0.5">
          {ch.type}
        </div>
      </div>

      <DropdownMenu>
        <DropdownMenuTrigger
          className={buttonVariants({ variant: "ghost", size: "icon-sm" })}
          aria-label="Row actions"
        >
          <MoreHorizontal className="h-4 w-4" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem
            onClick={() => router.push(`/channels/${ch.id}/edit`)}
          >
            <Pencil className="mr-2 h-4 w-4" /> Edit
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            onClick={() => setDeleteOpen(true)}
            className="text-red-600 focus:text-red-600 focus:bg-red-500/10"
          >
            <Trash2 className="mr-2 h-4 w-4" /> Delete
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <ConfirmDestructiveDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title="Delete channel?"
        description={
          <>
            {ch.name}
            <br />
            Monitors bound to this channel will lose this alert destination.
          </>
        }
        pending={del.isPending}
        onConfirm={async () => {
          if (ch.id) await del.mutateAsync(ch.id);
          setDeleteOpen(false);
        }}
      />
    </div>
  );
}

"use client";

import { useRouter } from "next/navigation";
import { useDeleteMonitor } from "@/lib/mutations";
import { ConfirmDestructiveDialog } from "@/components/features/common/confirm-destructive-dialog";

interface Props {
  monitorId: string;
  monitorName: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  redirectOnSuccess?: string;
}

export function DeleteMonitorDialog({
  monitorId,
  monitorName,
  open,
  onOpenChange,
  redirectOnSuccess,
}: Props) {
  const del = useDeleteMonitor();
  const router = useRouter();

  const onConfirm = async () => {
    await del.mutateAsync(monitorId);
    onOpenChange(false);
    if (redirectOnSuccess) router.push(redirectOnSuccess);
  };

  return (
    <ConfirmDestructiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title="Delete monitor?"
      description={
        <>
          {monitorName}
          <br />
          This permanently removes the monitor and all its check history. This
          cannot be undone.
        </>
      }
      confirmLabel="Delete"
      pendingLabel="Deleting…"
      pending={del.isPending}
      onConfirm={onConfirm}
    />
  );
}

"use client";

import { useRouter } from "next/navigation";
import { useDeleteMonitor } from "@/lib/mutations";
import { ConfirmDestructiveDialog } from "@/components/features/common/confirm-destructive-dialog";
import { useLocale } from "@/components/i18n/locale-provider";

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
  const { dict } = useLocale();
  const t = dict.monitors;
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
      title={t.delete_dialog_title}
      description={t.delete_dialog_body.replace("{name}", monitorName)}
      confirmLabel={dict.common.delete}
      pendingLabel={dict.common.deleting}
      pending={del.isPending}
      onConfirm={onConfirm}
    />
  );
}

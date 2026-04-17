"use client";

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { useDeleteMonitor } from "@/lib/mutations";
import { useRouter } from "next/navigation";

interface Props {
  monitorId: string;
  monitorName: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  redirectOnSuccess?: string;
}

/**
 * Controlled delete-confirmation dialog. Parent owns open state and
 * triggers the dialog by setting open=true (e.g., from a dropdown-menu item).
 */
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
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete monitor?</AlertDialogTitle>
          <AlertDialogDescription>
            {monitorName}
            <br />
            This permanently removes the monitor and all its check history.
            This cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            onClick={onConfirm}
            disabled={del.isPending}
            className="bg-red-600 text-white hover:bg-red-700"
          >
            {del.isPending ? "Deleting…" : "Delete"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

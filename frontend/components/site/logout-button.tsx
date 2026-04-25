"use client";

import { useTransition } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { apiFetch } from "@/lib/api";
import { useLocale } from "@/components/i18n/locale-provider";

/**
 * Client-side logout. Calls /api/auth/logout, refreshes server-side
 * navbar (which checks the session cookie), then sends the user home
 * in their current locale.
 */
export function LogoutButton() {
  const { dict, locale } = useLocale();
  const router = useRouter();
  const [pending, startTransition] = useTransition();

  const onClick = async () => {
    try {
      await apiFetch<void>("/auth/logout", { method: "POST" });
    } catch {
      // Server may have already expired the session — still log out locally.
    }
    startTransition(() => {
      router.push(`/${locale}`);
      router.refresh();
      toast.success(dict.nav.logout);
    });
  };

  return (
    <Button
      type="button"
      variant="ghost"
      size="sm"
      onClick={onClick}
      disabled={pending}
    >
      {pending ? dict.common.loading : dict.nav.logout}
    </Button>
  );
}

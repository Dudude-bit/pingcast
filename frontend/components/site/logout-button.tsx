"use client";

import { useTransition } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { apiFetch } from "@/lib/api";

/**
 * Client-side logout. The previous form-POST pattern returned 204 No
 * Content which left the browser on a blank page and never refreshed
 * the SSR-rendered navbar — the user stayed logged in from the UI
 * perspective until they clicked to another page.
 *
 * Fetch-based flow: call /api/auth/logout, then router.refresh() to
 * re-run the server-side sessionCookie() check in the navbar, then
 * push the user home.
 */
export function LogoutButton() {
  const router = useRouter();
  const [pending, startTransition] = useTransition();

  const onClick = async () => {
    try {
      await apiFetch<void>("/auth/logout", { method: "POST" });
    } catch {
      // Server may have already expired the session — still log out locally.
    }
    startTransition(() => {
      router.push("/");
      router.refresh();
      toast.success("Signed out");
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
      {pending ? "Signing out…" : "Logout"}
    </Button>
  );
}

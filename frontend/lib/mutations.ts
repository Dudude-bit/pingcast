"use client";

import {
  useMutation,
  useQueryClient,
  type QueryClient,
  type UseMutationOptions,
} from "@tanstack/react-query";
import { apiFetch, ApiError } from "./api";
import { toast } from "sonner";
import type { components } from "./openapi-types";

type CreateReq = components["schemas"]["CreateMonitorRequest"];
type UpdateReq = components["schemas"]["UpdateMonitorRequest"];
type Monitor = components["schemas"]["Monitor"];

/**
 * mutationErrorToast centralises the shape of toast errors produced
 * by every mutation. Instead of nine near-identical onError handlers
 * all emitting "Create failed: {some message}", we branch once on the
 * ApiError.code so the user gets intent-specific guidance:
 *
 *   - RATE_LIMITED       → "Slow down — try again in a minute."
 *   - VALIDATION_FAILED  → just the server's message (already specific)
 *   - UNAUTHORIZED       → "You're signed out — please log in again."
 *   - other              → "{verb} failed: {message}" with the verb from
 *                          the call site (Create / Update / Delete …)
 *
 * Non-ApiError errors (network drops, JSON parse failures) still get
 * a "{verb} failed: …" prefix so the user can see what action broke.
 */
function mutationErrorToast(verb: string): (e: Error) => void {
  return (e) => {
    if (e instanceof ApiError) {
      switch (e.code) {
        case "RATE_LIMITED":
          toast.error("Slow down — too many requests. Try again in a minute.");
          return;
        case "VALIDATION_FAILED":
        case "CONFLICT":
          toast.error(e.message);
          return;
        case "UNAUTHORIZED":
          toast.error("You're signed out — please log in again.");
          return;
      }
    }
    toast.error(`${verb} failed: ${e.message}`);
  };
}

/**
 * onMutationSuccess builds the onSuccess callback every CRUD hook
 * needed by hand: invalidate one or more query keys + (optional) toast.
 * `success` may be a static string ("Monitor created") or a function of
 * the mutation result (toggle pause flips between two messages). Pass
 * `undefined` for hooks that suppress the toast (e.g. useCreateAPIKey
 * — the parent UI shows the new secret instead).
 */
function onMutationSuccess<TResult, TArg>(
  qc: QueryClient,
  invalidate: readonly (readonly string[])[],
  success?: string | ((result: TResult, arg: TArg) => string),
): NonNullable<UseMutationOptions<TResult, Error, TArg>["onSuccess"]> {
  return (result, arg) => {
    for (const key of invalidate) {
      qc.invalidateQueries({ queryKey: [...key] });
    }
    if (success === undefined) return;
    const msg = typeof success === "function" ? success(result, arg) : success;
    if (msg) toast.success(msg);
  };
}

// --- Monitors ---

export function useCreateMonitor() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: CreateReq) =>
      apiFetch<Monitor>("/monitors", { method: "POST", body }),
    onSuccess: onMutationSuccess(qc, [["monitors"]], "Monitor created"),
    onError: mutationErrorToast("Create"),
  });
}

export function useUpdateMonitor(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: UpdateReq) =>
      apiFetch<Monitor>(`/monitors/${id}`, { method: "PUT", body }),
    onSuccess: onMutationSuccess(qc, [["monitors"], ["monitors", id]], "Monitor updated"),
    onError: mutationErrorToast("Update"),
  });
}

export function useDeleteMonitor() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/monitors/${id}`, { method: "DELETE" }),
    onSuccess: onMutationSuccess(qc, [["monitors"]], "Monitor deleted"),
    onError: mutationErrorToast("Delete"),
  });
}

export function useTogglePause() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<Monitor>(`/monitors/${id}/pause`, { method: "POST" }),
    onSuccess: onMutationSuccess<Monitor, string>(
      qc,
      [["monitors"]],
      (updated) => (updated.is_paused ? "Monitor paused" : "Monitor resumed"),
    ),
    onError: mutationErrorToast("Toggle"),
  });
}

// --- Channels ---
type Channel = components["schemas"]["NotificationChannel"];
type CreateChannelReq = components["schemas"]["CreateChannelRequest"];
type UpdateChannelReq = components["schemas"]["UpdateChannelRequest"];

export function useCreateChannel() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: CreateChannelReq) =>
      apiFetch<Channel>("/channels", { method: "POST", body }),
    onSuccess: onMutationSuccess(qc, [["channels"]], "Channel created"),
    onError: mutationErrorToast("Create"),
  });
}

export function useUpdateChannel(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: UpdateChannelReq) =>
      apiFetch<Channel>(`/channels/${id}`, { method: "PUT", body }),
    onSuccess: onMutationSuccess(qc, [["channels"]], "Channel updated"),
    onError: mutationErrorToast("Update"),
  });
}

export function useDeleteChannel() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/channels/${id}`, { method: "DELETE" }),
    onSuccess: onMutationSuccess(qc, [["channels"]], "Channel deleted"),
    onError: mutationErrorToast("Delete"),
  });
}

// --- API Keys ---
type CreateAPIKeyReq = components["schemas"]["CreateAPIKeyRequest"];
type APIKeyCreated = components["schemas"]["APIKeyCreated"];

export function useCreateAPIKey() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: CreateAPIKeyReq) =>
      apiFetch<APIKeyCreated>("/api-keys", { method: "POST", body }),
    // Suppress success toast — the parent reveals the new secret instead.
    onSuccess: onMutationSuccess(qc, [["api-keys"]]),
    onError: mutationErrorToast("Create"),
  });
}

export function useRevokeAPIKey() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/api-keys/${id}`, { method: "DELETE" }),
    onSuccess: onMutationSuccess(qc, [["api-keys"]], "API key revoked"),
    onError: mutationErrorToast("Revoke"),
  });
}

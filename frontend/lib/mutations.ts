"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
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

export function useCreateMonitor() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: CreateReq) =>
      apiFetch<Monitor>("/monitors", { method: "POST", body }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["monitors"] });
      toast.success("Monitor created");
    },
    onError: mutationErrorToast("Create"),
  });
}

export function useUpdateMonitor(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: UpdateReq) =>
      apiFetch<Monitor>(`/monitors/${id}`, { method: "PUT", body }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["monitors"] });
      qc.invalidateQueries({ queryKey: ["monitors", id] });
      toast.success("Monitor updated");
    },
    onError: mutationErrorToast("Update"),
  });
}

export function useDeleteMonitor() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/monitors/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["monitors"] });
      toast.success("Monitor deleted");
    },
    onError: mutationErrorToast("Delete"),
  });
}

export function useTogglePause() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<Monitor>(`/monitors/${id}/pause`, { method: "POST" }),
    onSuccess: (updated, id) => {
      qc.invalidateQueries({ queryKey: ["monitors"] });
      qc.invalidateQueries({ queryKey: ["monitors", id] });
      toast.success(updated.is_paused ? "Monitor paused" : "Monitor resumed");
    },
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
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["channels"] });
      toast.success("Channel created");
    },
    onError: mutationErrorToast("Create"),
  });
}

export function useUpdateChannel(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: UpdateChannelReq) =>
      apiFetch<Channel>(`/channels/${id}`, { method: "PUT", body }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["channels"] });
      toast.success("Channel updated");
    },
    onError: mutationErrorToast("Update"),
  });
}

export function useDeleteChannel() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/channels/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["channels"] });
      toast.success("Channel deleted");
    },
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
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["api-keys"] });
    },
    onError: mutationErrorToast("Create"),
  });
}

export function useRevokeAPIKey() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/api-keys/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["api-keys"] });
      toast.success("API key revoked");
    },
    onError: mutationErrorToast("Revoke"),
  });
}

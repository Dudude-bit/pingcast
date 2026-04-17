"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "./api";
import { toast } from "sonner";
import type { components } from "./openapi-types";

type CreateReq = components["schemas"]["CreateMonitorRequest"];
type UpdateReq = components["schemas"]["UpdateMonitorRequest"];
type Monitor = components["schemas"]["Monitor"];

export function useCreateMonitor() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: CreateReq) =>
      apiFetch<Monitor>("/monitors", { method: "POST", body }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["monitors"] });
      toast.success("Monitor created");
    },
    onError: (e: Error) => toast.error(`Create failed: ${e.message}`),
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
    onError: (e: Error) => toast.error(`Update failed: ${e.message}`),
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
    onError: (e: Error) => toast.error(`Delete failed: ${e.message}`),
  });
}

export function useTogglePause() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<Monitor>(`/monitors/${id}/pause`, { method: "POST" }),
    onSuccess: (_, id) => {
      qc.invalidateQueries({ queryKey: ["monitors"] });
      qc.invalidateQueries({ queryKey: ["monitors", id] });
    },
    onError: (e: Error) => toast.error(`Toggle failed: ${e.message}`),
  });
}

"use client";

import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "./api";
import type { components } from "./openapi-types";

export type Monitor = components["schemas"]["Monitor"];
export type MonitorWithUptime = components["schemas"]["MonitorWithUptime"];
export type MonitorDetail = components["schemas"]["MonitorDetail"];
export type MonitorTypeInfo = components["schemas"]["MonitorTypeInfo"];
export type Channel = components["schemas"]["NotificationChannel"];
export type Incident = components["schemas"]["Incident"];

export function useMonitors() {
  return useQuery({
    queryKey: ["monitors"],
    queryFn: () => apiFetch<MonitorWithUptime[]>("/monitors"),
    refetchInterval: 15_000,
  });
}

export function useMonitor(id: string | undefined) {
  return useQuery({
    queryKey: ["monitors", id],
    queryFn: () => apiFetch<MonitorDetail>(`/monitors/${id}`),
    enabled: Boolean(id),
    refetchInterval: 15_000,
  });
}

export function useMonitorTypes() {
  return useQuery({
    queryKey: ["monitor-types"],
    queryFn: () => apiFetch<MonitorTypeInfo[]>("/monitor-types"),
    staleTime: Infinity,
  });
}

export function useChannels() {
  return useQuery({
    queryKey: ["channels"],
    queryFn: () => apiFetch<Channel[]>("/channels"),
  });
}

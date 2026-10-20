// React Query hooks over lib/api — the portal's data-fetching convention.
// Key factories keep invalidation centralised.

"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import * as api from "./api/resources";

export const keys = {
  me: ["me"] as const,
  clients: ["clients"] as const,
  client: (id: string) => ["clients", id] as const,
  customers: (clientId: string) => ["clients", clientId, "customers"] as const,
  apiKeys: (clientId: string) => ["clients", clientId, "api-keys"] as const,
  masters: (clientId: string) => ["clients", clientId, "masters"] as const,
  variants: (customerId: string) =>
    ["customers", customerId, "variants"] as const,
  request: (id: string) => ["requests", id] as const,
  requests: (filter: { client_id?: string; status?: string; limit?: number }) =>
    ["requests", "list", filter] as const,
  usage: (days: number, clientId?: string) =>
    ["usage", days, clientId ?? ""] as const,
};

export const useMe = (enabled = true) =>
  useQuery({ queryKey: keys.me, queryFn: api.auth.me, enabled });

export const useClients = () =>
  useQuery({ queryKey: keys.clients, queryFn: api.clients.list });

export const useClient = (id: string) =>
  useQuery({
    queryKey: keys.client(id),
    queryFn: () => api.clients.get(id),
    enabled: Boolean(id),
  });

export const useCustomers = (clientId: string) =>
  useQuery({
    queryKey: keys.customers(clientId),
    queryFn: () => api.customers.list(clientId),
    enabled: Boolean(clientId),
  });

export const useMasters = (clientId: string) =>
  useQuery({
    queryKey: keys.masters(clientId),
    queryFn: () => api.masters.list(clientId),
    enabled: Boolean(clientId),
  });

export const useApiKeys = (clientId: string) =>
  useQuery({
    queryKey: keys.apiKeys(clientId),
    queryFn: () => api.apiKeys.list(clientId),
    enabled: Boolean(clientId),
  });

export const useRequestView = (id: string) =>
  useQuery({
    queryKey: keys.request(id),
    queryFn: () => api.requests.get(id),
    enabled: Boolean(id),
  });

export const useRequests = (
  filter: { client_id?: string; status?: string; limit?: number } = {},
) =>
  useQuery({
    queryKey: keys.requests(filter),
    queryFn: () => api.requests.list(filter),
  });

export const useUsage = (days = 7, clientId?: string) =>
  useQuery({
    queryKey: keys.usage(days, clientId),
    queryFn: () => api.usage.get(days, clientId),
  });

export const useLogin = () =>
  useMutation({
    mutationFn: ({ email, password }: { email: string; password: string }) =>
      api.auth.login(email, password),
  });

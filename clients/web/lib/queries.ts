// React Query hooks over lib/api — the portal's data-fetching convention.
// Key factories keep invalidation centralised.

"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { ApiError } from "./api/client";
import * as api from "./api/resources";

export const keys = {
  me: ["me"] as const,
  clients: ["clients"] as const,
  client: (id: string) => ["clients", id] as const,
  customers: (clientId: string) => ["clients", clientId, "customers"] as const,
  customer: (id: string) => ["customers", id] as const,
  apiKeys: (clientId: string) => ["clients", clientId, "api-keys"] as const,
  masters: (clientId: string) => ["clients", clientId, "masters"] as const,
  master: (id: string) => ["template-masters", id] as const,
  variants: (customerId: string) =>
    ["customers", customerId, "variants"] as const,
  variant: (masterId: string, customerId: string) =>
    ["template-variants", masterId, customerId] as const,
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

export const useCustomer = (id: string) =>
  useQuery({
    queryKey: keys.customer(id),
    queryFn: () => api.customers.get(id),
    enabled: Boolean(id),
  });

export const useMasters = (clientId: string) =>
  useQuery({
    queryKey: keys.masters(clientId),
    queryFn: () => api.masters.list(clientId),
    enabled: Boolean(clientId),
  });

export const useMaster = (id: string) =>
  useQuery({
    queryKey: keys.master(id),
    queryFn: () => api.masters.get(id),
    enabled: Boolean(id),
  });

export const useCustomerVariants = (customerId: string) =>
  useQuery({
    queryKey: keys.variants(customerId),
    queryFn: () => api.variants.listForCustomer(customerId),
    enabled: Boolean(customerId),
  });

// Saved variant for a (master, customer) pair — a 404 means "no override
// yet", which the editor treats as an empty overlay rather than an error.
export const useVariant = (masterId: string, customerId: string) =>
  useQuery({
    queryKey: keys.variant(masterId, customerId),
    queryFn: () => api.variants.get(masterId, customerId),
    enabled: Boolean(masterId) && Boolean(customerId),
    retry: (count, err) =>
      !(err instanceof ApiError && err.status === 404) && count < 2,
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

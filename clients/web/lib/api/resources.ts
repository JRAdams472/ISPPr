// Resource functions for the management API — one per endpoint in
// docs/management-api.md. All paths are under /api/v1/admin.

import { get, patch, post, put } from "./client";
import type {
  AdminUser,
  ApiKey,
  Client,
  Customer,
  LoginResponse,
  RequestView,
  TemplateMaster,
  TemplateVariant,
  Usage,
} from "./types";

const A = "/api/v1/admin";

export const auth = {
  login: (email: string, password: string) =>
    post<LoginResponse>(`${A}/login`, { email, password }),
  logout: () => post<undefined>(`${A}/logout`),
  me: () => get<AdminUser>(`${A}/me`),
};

export const clients = {
  list: () => get<Client[]>(`${A}/clients`),
  create: (body: { name: string; status?: string }) =>
    post<Client>(`${A}/clients`, body),
  get: (id: string) => get<Client>(`${A}/clients/${id}`),
  update: (id: string, body: { name?: string; status?: string }) =>
    patch<Client>(`${A}/clients/${id}`, body),
};

export const customers = {
  list: (clientId: string) =>
    get<Customer[]>(`${A}/clients/${clientId}/customers`),
  create: (
    clientId: string,
    body: { name: string; external_ref?: string; webhook_url?: string },
  ) => post<Customer>(`${A}/clients/${clientId}/customers`, body),
  get: (id: string) => get<Customer>(`${A}/customers/${id}`),
  update: (
    id: string,
    body: { name?: string; external_ref?: string; webhook_url?: string },
  ) => patch<Customer>(`${A}/customers/${id}`, body),
};

export const apiKeys = {
  list: (clientId: string) =>
    get<ApiKey[]>(`${A}/clients/${clientId}/api-keys`),
  create: (clientId: string, body: { label?: string; scopes?: string[] }) =>
    post<ApiKey>(`${A}/clients/${clientId}/api-keys`, body),
  rotate: (id: string) => post<ApiKey>(`${A}/api-keys/${id}/rotate`),
  revoke: (id: string) => post<ApiKey>(`${A}/api-keys/${id}/revoke`),
};

export const masters = {
  list: (clientId: string) =>
    get<TemplateMaster[]>(`${A}/clients/${clientId}/template-masters`),
  create: (
    clientId: string,
    body: {
      name: string;
      html: string;
      css?: string;
      defaults?: Record<string, unknown>;
    },
  ) => post<TemplateMaster>(`${A}/clients/${clientId}/template-masters`, body),
  publish: (
    clientId: string,
    name: string,
    body: { html: string; css?: string; defaults?: Record<string, unknown> },
  ) =>
    post<TemplateMaster>(
      `${A}/clients/${clientId}/template-masters/${name}/versions`,
      body,
    ),
  get: (id: string) => get<TemplateMaster>(`${A}/template-masters/${id}`),
};

export const variants = {
  upsert: (body: {
    master_id: string;
    customer_id: string;
    overlay?: Record<string, unknown>;
  }) => put<TemplateVariant>(`${A}/template-variants`, body),
  get: (masterId: string, customerId: string) =>
    get<TemplateVariant>(
      `${A}/template-variants?master_id=${masterId}&customer_id=${customerId}`,
    ),
  listForCustomer: (customerId: string) =>
    get<TemplateVariant[]>(`${A}/customers/${customerId}/variants`),
};

export const requests = {
  get: (id: string) => get<RequestView>(`${A}/requests/${id}`),
};

export const usage = {
  get: (days = 7) => get<Usage>(`${A}/usage?days=${days}`),
};

// DTOs for the management REST API — 1:1 with docs/management-api.md.
// Secret material never round-trips: ApiKey.plaintext exists only on
// create/rotate responses.

export interface AdminUser {
  id: string;
  email: string;
  name: string;
  role: string;
}

export interface LoginResponse {
  token: string;
  user: AdminUser;
}

export interface Client {
  id: string;
  name: string;
  status: string;
  created_at: string;
  updated_at: string;
  /** Present on list responses only. */
  customers?: number;
  active_keys?: number;
}

export interface Customer {
  id: string;
  client_id: string;
  external_ref?: string;
  name: string;
  webhook_url?: string;
  created_at: string;
  updated_at: string;
}

export interface ApiKey {
  id: string;
  client_id: string;
  label?: string;
  scopes: string[];
  created_at: string;
  rotated_at?: string;
  revoked_at?: string;
  last_used_at?: string;
  /** Present exactly once — on create/rotate responses only. */
  key?: string;
}

export interface TemplateMaster {
  id: string;
  client_id: string;
  name: string;
  version: number;
  html: string;
  css: string;
  defaults: Record<string, unknown>;
  created_at: string;
}

export interface VariantOverlay {
  css_patch?: string;
  toggles?: Record<string, boolean>;
  branding?: Record<string, string>;
}

export interface TemplateVariant {
  id: string;
  master_id: string;
  customer_id: string;
  overlay: VariantOverlay;
  updated_at: string;
}

export interface RequestView {
  id: string;
  client_id: string;
  customer_id: string;
  package_name: string;
  status: string;
  error?: string;
  metrics?: Record<string, unknown>;
  webhook_status: "NONE" | "PENDING" | "DELIVERED" | "EXHAUSTED";
  webhook_log: { at: string; attempt: number; code?: number; error?: string }[];
  created_at: string;
}

export interface UsageDay {
  day: string;
  client_id: string;
  customer_id: string;
  requests: number;
  completed: number;
  failed: number;
}

export interface Usage {
  days: number;
  by_day: UsageDay[];
  latency: { p50_ms: number; p95_ms: number; samples: number };
}

/** RFC 7807 problem-details body returned on errors. */
export interface ProblemDetails {
  type?: string;
  title?: string;
  status: number;
  detail?: string;
}

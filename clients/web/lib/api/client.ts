// Typed fetch layer for the management API: session Bearer auth, RFC 7807
// problem+json error mapping, JSON in/out. Browser-side only — the portal
// stores the session token via `getToken`/`setToken`.

import type { ProblemDetails } from "./types";

const base = () => process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";

export class ApiError extends Error {
  constructor(
    public status: number,
    public problem: ProblemDetails,
  ) {
    super(problem.detail ?? problem.title ?? `HTTP ${status}`);
    this.name = "ApiError";
  }
}

const TOKEN_KEY = "isppr_admin_token";

export function getToken(): string {
  if (typeof window === "undefined") return "";
  return window.localStorage.getItem(TOKEN_KEY) ?? "";
}

export function setToken(token: string) {
  if (typeof window === "undefined") return;
  if (token) window.localStorage.setItem(TOKEN_KEY, token);
  else window.localStorage.removeItem(TOKEN_KEY);
}

export async function request<T>(
  method: string,
  path: string,
  body?: unknown,
): Promise<T> {
  const headers: Record<string, string> = {};
  if (body !== undefined) headers["Content-Type"] = "application/json";
  const token = getToken();
  if (token) headers.Authorization = `Bearer ${token}`;

  const res = await fetch(`${base()}${path}`, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  });

  if (res.status === 204) return undefined as T;

  const text = await res.text();
  let data: unknown;
  try {
    data = text ? JSON.parse(text) : undefined;
  } catch {
    data = undefined;
  }
  if (!res.ok) {
    const problem: ProblemDetails =
      data && typeof data === "object" && "status" in data
        ? (data as ProblemDetails)
        : { status: res.status, detail: res.statusText };
    throw new ApiError(res.status, problem);
  }
  return data as T;
}

export const get = <T>(path: string) => request<T>("GET", path);
export const post = <T>(path: string, body?: unknown) =>
  request<T>("POST", path, body);
export const put = <T>(path: string, body: unknown) =>
  request<T>("PUT", path, body);
export const patch = <T>(path: string, body: unknown) =>
  request<T>("PATCH", path, body);

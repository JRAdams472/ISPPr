import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

const push = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push, replace: vi.fn() }),
  usePathname: () => "/clients",
  useParams: () => ({ id: "cli_1" }),
}));

import { Providers } from "../app/providers";
import ClientsPage from "../app/(portal)/clients/page";
import ClientDetail from "../app/(portal)/clients/[id]/page";

// URL-routed fetch stub: each endpoint returns its own fixture, verbs
// recorded for mutation assertions. /me controls the role under test.

const ME_ADMIN = { id: "u1", email: "a@b.c", name: "Admin", role: "admin" };
const CLIENT = {
  id: "cli_1",
  name: "Acme Corp",
  status: "active",
  created_at: "2026-01-05T00:00:00Z",
  updated_at: "2026-01-05T00:00:00Z",
  customers: 2,
  active_keys: 3,
};
const KEY = {
  id: "key_1",
  client_id: "cli_1",
  label: "prod",
  scopes: ["generate"],
  created_at: "2026-01-06T00:00:00Z",
};
const CUSTOMER = {
  id: "cust_1",
  client_id: "cli_1",
  name: "Cust One",
  external_ref: "E-9",
  created_at: "2026-01-06T00:00:00Z",
  updated_at: "2026-01-06T00:00:00Z",
};
const USAGE = {
  days: 30,
  by_day: [
    { day: new Date().toISOString().slice(0, 10), client_id: "cli_1", customer_id: "cust_1", requests: 3, completed: 2, failed: 1 },
  ],
  latency: { p50_ms: 12, p95_ms: 40, samples: 3 },
};
const FAILED_REQ = {
  id: "req_f1",
  client_id: "cli_1",
  customer_id: "cust_1",
  package_name: "invoices",
  status: "FAILED",
  error: "render blew up",
  webhook_status: "NONE",
  webhook_log: [],
  created_at: "2026-01-07T10:00:00Z",
};

function stubFetch(routes: Record<string, unknown> = {}) {
  const calls: { url: string; method: string; body?: unknown }[] = [];
  const defaults: Record<string, unknown> = {
    "/me": ME_ADMIN,
    "/clients/cli_1/api-keys": [KEY],
    "/clients/cli_1/customers": [CUSTOMER],
    "/clients/cli_1": CLIENT,
    "/usage": USAGE,
    "/requests": [FAILED_REQ],
    "/clients": [CLIENT],
  };
  const all = { ...defaults, ...routes };
  const mock = vi.fn(async (url: string, init?: RequestInit) => {
    const method = init?.method ?? "GET";
    calls.push({
      url,
      method,
      body: init?.body ? JSON.parse(init.body as string) : undefined,
    });
    for (const match of Object.keys(all)) {
      if (url.includes(match)) {
        const v = all[match];
        const payload =
          typeof v === "function"
            ? (v as (i?: RequestInit) => unknown)(init)
            : v;
        return new Response(JSON.stringify(payload), {
          status: method === "POST" ? 201 : 200,
        });
      }
    }
    return new Response(JSON.stringify({ status: 404 }), { status: 404 });
  });
  vi.stubGlobal("fetch", mock);
  return calls;
}

function stubClipboard() {
  const writeText = vi.fn(async () => undefined);
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText },
    configurable: true,
  });
  return writeText;
}

describe("/clients list", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    push.mockClear();
  });

  it("renders rows with counts and navigates on row click", async () => {
    stubFetch();
    const { findByText, getByText } = render(
      <Providers>
        <ClientsPage />
      </Providers>,
    );
    expect(await findByText("Acme Corp")).toBeTruthy();
    expect(getByText("active")).toBeTruthy();
    expect(getByText("2")).toBeTruthy();
    expect(getByText("3")).toBeTruthy();

    fireEvent.click(getByText("Acme Corp"));
    expect(push).toHaveBeenCalledWith("/clients/cli_1");
  });

  it("creates a client from the dialog and navigates to it", async () => {
    const calls = stubFetch({
      "/clients": (init?: RequestInit) =>
        init?.method === "POST"
          ? { ...CLIENT, id: "cli_new", name: "New Co" }
          : [CLIENT],
    });
    const { findByText, getByRole, getByLabelText } = render(
      <Providers>
        <ClientsPage />
      </Providers>,
    );
    fireEvent.click(await findByText("New client"));
    fireEvent.change(getByLabelText("Name"), { target: { value: "New Co" } });
    fireEvent.click(getByRole("button", { name: "Create client" }));
    await waitFor(() =>
      expect(push).toHaveBeenCalledWith("/clients/cli_new"),
    );
    const post = calls.find((c) => c.method === "POST");
    expect(post?.body).toEqual({ name: "New Co" });
  });

  it("hides write controls for viewer sessions", async () => {
    stubFetch({ "/me": { ...ME_ADMIN, role: "viewer" } });
    const { findByText, queryByText } = render(
      <Providers>
        <ClientsPage />
      </Providers>,
    );
    expect(await findByText("Acme Corp")).toBeTruthy();
    await waitFor(() => expect(queryByText("New client")).toBeNull());
    expect(queryByText("Disable")).toBeNull();
  });

  it("shows an empty state when there are no clients", async () => {
    stubFetch({ "/clients": [] });
    const { findByText } = render(
      <Providers>
        <ClientsPage />
      </Providers>,
    );
    expect(await findByText("No clients yet")).toBeTruthy();
  });

  it("shows an error state on fetch failure", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async () =>
          new Response('{"status":500,"detail":"db down"}', { status: 500 }),
      ),
    );
    // Providers retries 5xx by design — this suite asserts the terminal
    // error surface, so it uses a retry-free client instead of waiting out
    // the backoff.
    const qc = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const { findByText } = render(
      <QueryClientProvider client={qc}>
        <ClientsPage />
      </QueryClientProvider>,
    );
    expect(await findByText("Something went wrong")).toBeTruthy();
  });
});

describe("/clients/[id] detail", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("renders client header, keys, customers, metrics, and failures", async () => {
    stubFetch();
    const { findByText, getByText, findAllByText } = render(
      <Providers>
        <ClientDetail />
      </Providers>,
    );
    expect(await findByText("Acme Corp")).toBeTruthy();
    expect(getByText("API keys")).toBeTruthy();
    expect(await findByText("prod")).toBeTruthy();
    expect(await findByText("Cust One")).toBeTruthy();
    expect(getByText("p50 latency")).toBeTruthy();
    expect(getByText("12")).toBeTruthy();
    expect((await findAllByText("render blew up")).length).toBeGreaterThan(0);
  });

  it("creates a key and shows the plaintext once in the dialog", async () => {
    const calls = stubFetch({
      "/clients/cli_1/api-keys": (init?: RequestInit) =>
        init?.method === "POST"
          ? { ...KEY, key: "isppr_secret_once" }
          : [KEY],
    });
    const writeText = stubClipboard();
    const { findByText, getByRole, getByLabelText, getByText } = render(
      <Providers>
        <ClientDetail />
      </Providers>,
    );
    fireEvent.click(await findByText("New key"));
    fireEvent.change(getByLabelText(/Label/), {
      target: { value: "ci runner" },
    });
    fireEvent.click(getByRole("button", { name: "Create key" }));

    expect(await findByText("isppr_secret_once")).toBeTruthy();
    const post = calls.find((c) => c.method === "POST");
    expect(post?.body).toEqual({ label: "ci runner", scopes: ["generate"] });

    fireEvent.click(getByText("Copy"));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith("isppr_secret_once"));
    expect(await findByText("Copied")).toBeTruthy();
  });

  it("rotate returns a new plaintext in the dialog", async () => {
    stubFetch({
      "/api-keys": (init?: RequestInit) =>
        init?.method === "POST" ? { ...KEY, key: "isppr_rotated" } : {},
    });
    const { findByText, findAllByTitle } = render(
      <Providers>
        <ClientDetail />
      </Providers>,
    );
    const [rotateBtn] = await findAllByTitle("Rotate key");
    fireEvent.click(rotateBtn);
    expect(await findByText("isppr_rotated")).toBeTruthy();
    expect(await findByText("Rotated key")).toBeTruthy();
  });

  it("revoke posts to the revoke endpoint", async () => {
    const calls = stubFetch({ "/api-keys": () => ({ ...KEY, revoked_at: "2026-01-08T00:00:00Z" }) });
    const { findAllByTitle } = render(
      <Providers>
        <ClientDetail />
      </Providers>,
    );
    const [revokeBtn] = await findAllByTitle("Revoke key");
    fireEvent.click(revokeBtn);
    await waitFor(() =>
      expect(
        calls.some(
          (c) => c.method === "POST" && c.url.includes("/api-keys/key_1/revoke"),
        ),
      ).toBe(true),
    );
  });

  it("disables the client from the header action", async () => {
    const calls = stubFetch();
    const { findByText } = render(
      <Providers>
        <ClientDetail />
      </Providers>,
    );
    fireEvent.click(await findByText("Disable client"));
    await waitFor(() =>
      expect(
        calls.some(
          (c) =>
            c.method === "PATCH" &&
            c.url.includes("/clients/cli_1") &&
            (c.body as { status?: string })?.status === "disabled",
        ),
      ).toBe(true),
    );
  });

  it("hides mutating controls for viewers", async () => {
    stubFetch({ "/me": { ...ME_ADMIN, role: "viewer" } });
    const { findByText, queryByText } = render(
      <Providers>
        <ClientDetail />
      </Providers>,
    );
    expect(await findByText("Acme Corp")).toBeTruthy();
    await waitFor(() => expect(queryByText("New key")).toBeNull());
    expect(queryByText("Disable client")).toBeNull();
  });
});

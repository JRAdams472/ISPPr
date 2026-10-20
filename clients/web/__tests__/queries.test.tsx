import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { Providers } from "../app/providers";
import {
  keys,
  useApiKeys,
  useClient,
  useClients,
  useCustomer,
  useCustomers,
  useCustomerVariants,
  useLogin,
  useMaster,
  useMasters,
  useMe,
  useRequests,
  useRequestView,
  useUsage,
  useVariant,
} from "../lib/queries";

function wrapper({ children }: { children: React.ReactNode }) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

const ok = (data: unknown) =>
  vi.fn(async () => new Response(JSON.stringify(data), { status: 200 }));

describe("queries", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("useMe resolves the session user", async () => {
    vi.stubGlobal("fetch", ok({ id: "u1", email: "a@b.c" }));
    const { result } = renderHook(() => useMe(), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.email).toBe("a@b.c");
  });

  it("useClients resolves the list", async () => {
    vi.stubGlobal("fetch", ok([{ id: "cli_1", name: "Acme" }]));
    const { result } = renderHook(() => useClients(), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.[0].name).toBe("Acme");
  });

  it("disabled hooks skip fetch until an id is set", async () => {
    const fetchMock = ok([]);
    vi.stubGlobal("fetch", fetchMock);
    renderHook(() => useMasters(""), { wrapper });
    renderHook(() => useRequestView(""), { wrapper });
    await new Promise((r) => setTimeout(r, 20));
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("useUsage passes the window and client filter", async () => {
    const fetchMock = ok({ days: 14, by_day: [], latency: {} });
    vi.stubGlobal("fetch", fetchMock);
    const { result } = renderHook(() => useUsage(14, "cli_1"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    const [url] = fetchMock.mock.calls[0] as unknown as [string];
    expect(url).toContain("days=14");
    expect(url).toContain("client_id=cli_1");
  });

  it("useApiKeys and useRequests fetch their lists", async () => {
    const fetchMock = ok([]);
    vi.stubGlobal("fetch", fetchMock);
    const k = renderHook(() => useApiKeys("cli_1"), { wrapper });
    const r = renderHook(
      () => useRequests({ client_id: "cli_1", status: "FAILED", limit: 5 }),
      { wrapper },
    );
    await waitFor(() => expect(k.result.current.isSuccess).toBe(true));
    await waitFor(() => expect(r.result.current.isSuccess).toBe(true));
    const urls = fetchMock.mock.calls.map(
      (x) => (x as unknown as [string])[0],
    );
    expect(urls).toContain(
      "http://localhost:8090/api/v1/admin/clients/cli_1/api-keys",
    );
    expect(urls).toContain(
      "http://localhost:8090/api/v1/admin/requests?client_id=cli_1&status=FAILED&limit=5",
    );
  });

  it("useLogin posts credentials", async () => {
    const fetchMock = ok({ token: "t", user: { id: "u" } });
    vi.stubGlobal("fetch", fetchMock);
    const { result } = renderHook(() => useLogin(), { wrapper });
    await result.current.mutateAsync({ email: "a@b.c", password: "pw" });
    const [url, init] = fetchMock.mock.calls[0] as unknown as [
      string,
      RequestInit,
    ];
    expect(url).toContain("/login");
    expect(init.method).toBe("POST");
  });

  it("useClient and useCustomers fetch by id", async () => {
    const fetchMock = ok({ id: "cli_1" });
    vi.stubGlobal("fetch", fetchMock);
    const c = renderHook(() => useClient("cli_1"), { wrapper });
    const cu = renderHook(() => useCustomers("cli_1"), { wrapper });
    await waitFor(() => expect(c.result.current.isSuccess).toBe(true));
    await waitFor(() => expect(cu.result.current.isSuccess).toBe(true));
    const urls = fetchMock.mock.calls.map(
      (x) => (x as unknown as [string])[0],
    );
    expect(urls).toContain("http://localhost:8090/api/v1/admin/clients/cli_1");
    expect(urls).toContain(
      "http://localhost:8090/api/v1/admin/clients/cli_1/customers",
    );
  });

  it("Providers retry policy skips 4xx and retries 5xx", async () => {
    const fail401 = vi.fn(
      async () => new Response("{}", { status: 401, statusText: "no" }),
    );
    vi.stubGlobal("fetch", fail401);
    const a = renderHook(() => useClients(), { wrapper: Providers });
    await waitFor(() => expect(a.result.current.isError).toBe(true));
    expect(fail401).toHaveBeenCalledTimes(1);

    const fail500 = vi.fn(
      async () => new Response("{}", { status: 500, statusText: "boom" }),
    );
    vi.stubGlobal("fetch", fail500);
    renderHook(() => useClients(), { wrapper: Providers });
    await waitFor(() => expect(fail500.mock.calls.length).toBeGreaterThan(1), {
      timeout: 5000,
    });
  });

  it("customer/master/variant hooks fetch their resources", async () => {
    const fetchMock = ok({ id: "x" });
    vi.stubGlobal("fetch", fetchMock);
    const c = renderHook(() => useCustomer("cust_1"), { wrapper });
    const m = renderHook(() => useMaster("tmpl_1"), { wrapper });
    const vs = renderHook(() => useCustomerVariants("cust_1"), { wrapper });
    const v = renderHook(() => useVariant("tmpl_1", "cust_1"), { wrapper });
    await waitFor(() => expect(c.result.current.isSuccess).toBe(true));
    await waitFor(() => expect(m.result.current.isSuccess).toBe(true));
    await waitFor(() => expect(vs.result.current.isSuccess).toBe(true));
    await waitFor(() => expect(v.result.current.isSuccess).toBe(true));
    const urls = fetchMock.mock.calls.map(
      (x) => (x as unknown as [string])[0],
    );
    expect(urls).toContain(
      "http://localhost:8090/api/v1/admin/customers/cust_1",
    );
    expect(urls).toContain(
      "http://localhost:8090/api/v1/admin/template-masters/tmpl_1",
    );
    expect(urls).toContain(
      "http://localhost:8090/api/v1/admin/customers/cust_1/variants",
    );
    expect(urls).toContain(
      "http://localhost:8090/api/v1/admin/template-variants?master_id=tmpl_1&customer_id=cust_1",
    );
  });

  it("useVariant surfaces 404 as error without retrying", async () => {
    const notFound = vi.fn(
      async () =>
        new Response(JSON.stringify({ status: 404, title: "nf" }), {
          status: 404,
        }),
    );
    vi.stubGlobal("fetch", notFound);
    const { result } = renderHook(() => useVariant("tmpl_1", "cust_1"), {
      wrapper: Providers,
    });
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(notFound).toHaveBeenCalledTimes(1);
  });

  it("key factories stay stable", () => {
    expect(keys.clients).toEqual(["clients"]);
    expect(keys.customers("c1")).toEqual(["clients", "c1", "customers"]);
    expect(keys.customer("cu1")).toEqual(["customers", "cu1"]);
    expect(keys.master("t1")).toEqual(["template-masters", "t1"]);
    expect(keys.variants("cu1")).toEqual(["customers", "cu1", "variants"]);
    expect(keys.variant("t1", "cu1")).toEqual([
      "template-variants",
      "t1",
      "cu1",
    ]);
    expect(keys.request("r1")).toEqual(["requests", "r1"]);
    expect(keys.requests({ client_id: "c1" })).toEqual([
      "requests",
      "list",
      { client_id: "c1" },
    ]);
    expect(keys.usage(7)).toEqual(["usage", 7, ""]);
    expect(keys.usage(7, "c1")).toEqual(["usage", 7, "c1"]);
  });
});

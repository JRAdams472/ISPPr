import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { Providers } from "../app/providers";
import {
  keys,
  useClient,
  useClients,
  useCustomers,
  useLogin,
  useMasters,
  useMe,
  useRequestView,
  useUsage,
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

  it("useUsage passes the window", async () => {
    const fetchMock = ok({ days: 14, by_day: [], latency: {} });
    vi.stubGlobal("fetch", fetchMock);
    const { result } = renderHook(() => useUsage(14), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    const [url] = fetchMock.mock.calls[0] as unknown as [string];
    expect(url).toContain("days=14");
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

  it("key factories stay stable", () => {
    expect(keys.clients).toEqual(["clients"]);
    expect(keys.customers("c1")).toEqual(["clients", "c1", "customers"]);
    expect(keys.request("r1")).toEqual(["requests", "r1"]);
    expect(keys.usage(7)).toEqual(["usage", 7]);
  });
});

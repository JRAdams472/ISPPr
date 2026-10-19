import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError, getToken, post, request, setToken } from "../lib/api/client";
import { clients, usage } from "../lib/api/resources";

describe("api client", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });
  afterEach(() => vi.unstubAllGlobals());

  it("builds URLs against the API base and sends JSON", async () => {
    const fetchMock = vi.fn(
      async () =>
        new Response(JSON.stringify({ id: "cli_1", name: "Acme" }), {
          status: 201,
        }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const created = await clients.create({ name: "Acme" });
    expect(created).toEqual({ id: "cli_1", name: "Acme" });

    const [url, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("http://localhost:8090/api/v1/admin/clients");
    expect(init.method).toBe("POST");
    expect((init.headers as Record<string, string>)["Content-Type"]).toBe(
      "application/json",
    );
    expect(init.body).toBe('{"name":"Acme"}');
  });

  it("sends the stored session token as a Bearer header", async () => {
    setToken("sess_abc");
    const fetchMock = vi.fn(
      async () => new Response(JSON.stringify([]), { status: 200 }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await clients.list();
    const [, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect((init.headers as Record<string, string>).Authorization).toBe(
      "Bearer sess_abc",
    );
    expect(getToken()).toBe("sess_abc");

    setToken("");
    expect(getToken()).toBe("");
  });

  it("maps problem+json bodies into ApiError", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async () =>
          new Response(
            JSON.stringify({
              type: "about:blank",
              title: "Not Found",
              status: 404,
              detail: "client cli_x not found",
            }),
            { status: 404 },
          ),
      ),
    );

    const err = await clients.get("cli_x").catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).status).toBe(404);
    expect((err as ApiError).message).toBe("client cli_x not found");
    expect((err as ApiError).problem.title).toBe("Not Found");
  });

  it("survives non-problem error bodies", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response("upstream exploded", { status: 502 })),
    );
    await expect(request("GET", "/x")).rejects.toBeInstanceOf(ApiError);
  });

  it("returns undefined for 204 responses", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response(null, { status: 204 })),
    );
    await expect(post("/api/v1/admin/logout")).resolves.toBeUndefined();
  });

  it("encodes the usage window in the query string", async () => {
    const fetchMock = vi.fn(
      async () =>
        new Response(
          JSON.stringify({ days: 30, by_day: [], latency: {} }),
          { status: 200 },
        ),
    );
    vi.stubGlobal("fetch", fetchMock);
    await usage.get(30);
    const [url] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("http://localhost:8090/api/v1/admin/usage?days=30");
  });
});

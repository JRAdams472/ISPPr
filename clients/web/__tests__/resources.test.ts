import { afterEach, describe, expect, it, vi } from "vitest";
import {
  apiKeys,
  auth,
  clients,
  customers,
  masters,
  requests,
  usage,
  variants,
} from "../lib/api/resources";

// One assertion set per endpoint: URL + method + body shape, verified
// against a mocked fetch. Response typing is compile-time.

type Call = { url: string; method: string; body?: Record<string, unknown> };

function stubFetch() {
  const calls: Call[] = [];
  const mock = vi.fn(async (url: string, init?: RequestInit) => {
    calls.push({
      url,
      method: init?.method ?? "GET",
      body: init?.body ? JSON.parse(init.body as string) : undefined,
    });
    return new Response("{}", { status: 200 });
  });
  vi.stubGlobal("fetch", mock);
  return calls;
}

const A = "http://localhost:8090/api/v1/admin";

describe("resources", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("auth endpoints", async () => {
    const calls = stubFetch();
    await auth.login("a@b.c", "pw");
    await auth.logout();
    await auth.me();
    expect(calls[0]).toEqual({
      url: `${A}/login`,
      method: "POST",
      body: { email: "a@b.c", password: "pw" },
    });
    expect(calls[1].url).toBe(`${A}/logout`);
    expect(calls[2].url).toBe(`${A}/me`);
  });

  it("client CRUD", async () => {
    const calls = stubFetch();
    await clients.list();
    await clients.create({ name: "Acme", status: "active" });
    await clients.get("cli_1");
    await clients.update("cli_1", { status: "disabled" });
    expect(calls[0].url).toBe(`${A}/clients`);
    expect(calls[1]).toEqual({
      url: `${A}/clients`,
      method: "POST",
      body: { name: "Acme", status: "active" },
    });
    expect(calls[2].url).toBe(`${A}/clients/cli_1`);
    expect(calls[3].method).toBe("PATCH");
    expect(calls[3].body).toEqual({ status: "disabled" });
  });

  it("customer CRUD is nested under clients", async () => {
    const calls = stubFetch();
    await customers.list("cli_1");
    await customers.create("cli_1", { name: "C", webhook_url: "https://x" });
    await customers.get("cust_1");
    await customers.update("cust_1", { external_ref: "E-1" });
    expect(calls[0].url).toBe(`${A}/clients/cli_1/customers`);
    expect(calls[1].url).toBe(`${A}/clients/cli_1/customers`);
    expect(calls[1].body?.webhook_url).toBe("https://x");
    expect(calls[2].url).toBe(`${A}/customers/cust_1`);
    expect(calls[3].body).toEqual({ external_ref: "E-1" });
  });

  it("api key lifecycle", async () => {
    const calls = stubFetch();
    await apiKeys.list("cli_1");
    await apiKeys.create("cli_1", { label: "prod", scopes: ["generate"] });
    await apiKeys.rotate("key_1");
    await apiKeys.revoke("key_1");
    expect(calls[0].url).toBe(`${A}/clients/cli_1/api-keys`);
    expect(calls[1].body).toEqual({ label: "prod", scopes: ["generate"] });
    expect(calls[2].url).toBe(`${A}/api-keys/key_1/rotate`);
    expect(calls[3].url).toBe(`${A}/api-keys/key_1/revoke`);
  });

  it("master versioning endpoints", async () => {
    const calls = stubFetch();
    await masters.list("cli_1");
    await masters.create("cli_1", { name: "invoice", html: "<p>x</p>" });
    await masters.publish("cli_1", "invoice", { html: "<p>v2</p>" });
    await masters.get("tm_1");
    expect(calls[0].url).toBe(`${A}/clients/cli_1/template-masters`);
    expect(calls[2].url).toBe(
      `${A}/clients/cli_1/template-masters/invoice/versions`,
    );
    expect(calls[3].url).toBe(`${A}/template-masters/tm_1`);
  });

  it("variant endpoints", async () => {
    const calls = stubFetch();
    await variants.upsert({ master_id: "tm_1", customer_id: "cust_1" });
    await variants.get("tm_1", "cust_1");
    await variants.listForCustomer("cust_1");
    expect(calls[0].method).toBe("PUT");
    expect(calls[0].url).toBe(`${A}/template-variants`);
    expect(calls[1].url).toBe(
      `${A}/template-variants?master_id=tm_1&customer_id=cust_1`,
    );
    expect(calls[2].url).toBe(`${A}/customers/cust_1/variants`);
  });

  it("variant preview posts the draft overlay and returns a Blob", async () => {
    const calls = stubFetch();
    const blob = await variants.preview({
      master_id: "tm_1",
      overlay: { toggles: { footer: false } },
      data: { doc_title: "Hi" },
    });
    expect(blob).toBeInstanceOf(Blob);
    const call = calls[0];
    expect(call.method).toBe("POST");
    expect(call.url).toBe(`${A}/template-variants/preview`);
    expect(call.body).toEqual({
      master_id: "tm_1",
      overlay: { toggles: { footer: false } },
      data: { doc_title: "Hi" },
    });
  });

  it("postBlob maps problem+json errors to ApiError", async () => {
    const { ApiError } = await import("../lib/api/client");
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async () =>
          new Response(
            JSON.stringify({ status: 400, detail: "invalid overlay" }),
            { status: 400 },
          ),
      ),
    );
    await expect(
      variants.preview({ master_id: "tm_1" }),
    ).rejects.toBeInstanceOf(ApiError);
    await expect(
      variants.preview({ master_id: "tm_1" }),
    ).rejects.toMatchObject({ status: 400 });
  });

  it("request ops view", async () => {
    const calls = stubFetch();
    await requests.get("req_1");
    await requests.list({ client_id: "cli_1", status: "FAILED", limit: 10 });
    await requests.list();
    expect(calls[0].url).toBe(`${A}/requests/req_1`);
    expect(calls[1].url).toBe(
      `${A}/requests?client_id=cli_1&status=FAILED&limit=10`,
    );
    expect(calls[2].url).toBe(`${A}/requests`);
  });

  it("usage window + optional client filter", async () => {
    const calls = stubFetch();
    await usage.get(30);
    await usage.get(14, "cli_1");
    expect(calls[0].url).toBe(`${A}/usage?days=30`);
    expect(calls[1].url).toBe(`${A}/usage?days=14&client_id=cli_1`);
  });
});

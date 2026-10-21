import { test, expect } from "@playwright/test";
import { API_URL, TENANT_KEY, DEMO_CUSTOMER } from "./helpers";

// Tenant API loop — no browser state needed; exercises the same path a
// production client would (generate → poll status → presigned download).

test("generate → status → download completes end-to-end", async ({
  request,
}) => {
  const res = await request.post(`${API_URL}/api/v1/packages/generate`, {
    headers: { "X-API-Key": TENANT_KEY },
    data: {
      customer_id: DEMO_CUSTOMER.id,
      package_name: `api-e2e-${Date.now()}`,
      documents: [
        {
          template_id: "tmpl_invoice",
          payload: {
            title: "API e2e",
            total: 7,
            due: "2026-11-01",
            items: [{ desc: "item", price: 7 }],
          },
        },
      ],
    },
  });
  expect([200, 201, 202]).toContain(res.status());
  const created = (await res.json()) as { request_id: string };
  expect(created.request_id).toMatch(/^req_/);

  // Poll until terminal — the sync budget may return before the worker
  // finishes, so the status route is the contract being verified.
  let download: string | undefined;
  await expect
    .poll(
      async () => {
        const s = await request.get(
          `${API_URL}/api/v1/packages/${created.request_id}`,
          { headers: { "X-API-Key": TENANT_KEY } },
        );
        const body = (await s.json()) as {
          status: string;
          results?: { download_url?: string };
        };
        download = body.results?.download_url ?? download;
        return body.status;
      },
      { timeout: 30_000 },
    )
    .toBe("COMPLETED");
  expect(download).toBeTruthy();

  const url = download!.startsWith("http")
    ? download!
    : `${API_URL}${download}`;
  const dl = await request.get(url, { maxRedirects: 5 });
  expect(dl.ok()).toBeTruthy();
  expect((await dl.body()).byteLength).toBeGreaterThan(100);
});

test("tenant auth and idempotency hold at the edge", async ({ request }) => {
  // Missing key → 401 problem.
  const noAuth = await request.post(`${API_URL}/api/v1/packages/generate`, {
    data: {},
  });
  expect(noAuth.status()).toBe(401);
  expect(noAuth.headers()["content-type"]).toContain("application/problem+json");

  // Same Idempotency-Key replays to the same request_id — no duplicate work.
  const key = `e2e-${Date.now()}`;
  const body = {
    customer_id: DEMO_CUSTOMER.id,
    package_name: key,
    documents: [
      {
        template_id: "tmpl_invoice",
        payload: { title: "idem", total: 1, due: "2026-11-01", items: [] },
      },
    ],
  };
  const first = await request.post(`${API_URL}/api/v1/packages/generate`, {
    headers: { "X-API-Key": TENANT_KEY, "Idempotency-Key": key },
    data: body,
  });
  const second = await request.post(`${API_URL}/api/v1/packages/generate`, {
    headers: { "X-API-Key": TENANT_KEY, "Idempotency-Key": key },
    data: body,
  });
  const a = (await first.json()) as { request_id: string };
  const b = (await second.json()) as { request_id: string };
  expect(b.request_id).toBe(a.request_id);
});

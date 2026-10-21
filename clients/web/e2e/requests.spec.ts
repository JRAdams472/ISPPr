import { test, expect, request as playwrightRequest } from "@playwright/test";
import { API_URL, TENANT_KEY, DEMO_CUSTOMER } from "./helpers";

// Request-monitoring surface. Each run mints a real package through the
// tenant API so the list and detail pages have known data to render.

async function generatePackage(
  request: import("@playwright/test").APIRequestContext,
) {
  const res = await request.post(`${API_URL}/api/v1/packages/generate`, {
    headers: { "X-API-Key": TENANT_KEY },
    data: {
      customer_id: DEMO_CUSTOMER.id,
      package_name: `e2e-${Date.now()}`,
      // Two documents keep the output a ZIP even if the deployment
      // enables single-doc skip-zip.
      documents: [0, 1].map((i) => ({
        template_id: "tmpl_invoice",
        payload: {
          title: `E2E invoice ${i}`,
          total: 42,
          due: "2026-11-01",
          items: [{ desc: "service", price: 42 }],
        },
      })),
    },
  });
  expect(res.ok()).toBeTruthy();
  const body = (await res.json()) as { request_id: string; status: string };
  return body.request_id;
}

test("requests list renders the generated row and drills into detail", async ({
  page,
  request,
}) => {
  const rid = await generatePackage(request);
  // Give the worker a beat in case the sync budget returned QUEUED.
  await expect
    .poll(async () => {
      const res = await request.get(`${API_URL}/api/v1/packages/${rid}`, {
        headers: { "X-API-Key": TENANT_KEY },
      });
      return res.ok() ? ((await res.json()) as { status: string }).status : "";
    })
    .toBe("COMPLETED");

  await page.goto("/requests");
  await expect(
    page.getByRole("heading", { name: "Requests" }),
  ).toBeVisible();
  const row = page.getByRole("row", { name: new RegExp(rid) });
  await expect(row).toBeVisible();
  await expect(row.getByText(/COMPLETED/i)).toBeVisible();
  await row.click();

  // Detail: identity, metrics waterfall, live download link.
  await expect(page.getByText(rid)).toBeVisible();
  await expect(page.getByText(/total/i)).toBeVisible();
  await expect(
    page.getByRole("link", { name: /download/i }),
  ).toBeVisible();
});

test("detail download URL returns the artifact", async ({ request }) => {
  const rid = await generatePackage(request);
  const ctx = await playwrightRequest.newContext();
  try {
    let download: string | undefined;
    for (let i = 0; i < 30; i++) {
      const res = await ctx.get(`${API_URL}/api/v1/packages/${rid}`, {
        headers: { "X-API-Key": TENANT_KEY },
      });
      const body = (await res.json()) as {
        status: string;
        results?: { download_url?: string };
      };
      if (body.status === "COMPLETED" && body.results?.download_url) {
        download = body.results.download_url;
        break;
      }
      await new Promise((r) => setTimeout(r, 500));
    }
    expect(download, "request never completed with a download").toBeTruthy();

    // LocalStore serves a relative /api/v1/... path; S3 is absolute.
    const url = download!.startsWith("http")
      ? download!
      : `${API_URL}${download}`;
    const dl = await ctx.get(url, { maxRedirects: 5 });
    expect(dl.ok()).toBeTruthy();
    const buf = await dl.body();
    // ZIP magic bytes — the artifact made the full loop.
    expect(buf.subarray(0, 2).toString()).toBe("PK");
  } finally {
    await ctx.dispose();
  }
});

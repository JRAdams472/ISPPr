import { test, expect } from "@playwright/test";
import { inflateRawSync } from "node:zlib";
import { API_URL, TENANT_KEY, DEMO_CLIENT, DEMO_CUSTOMER } from "./helpers";

// First file entry from a ZIP via its central directory (single-doc
// packages contain one PDF; Go's archiver writes data descriptors, so
// the local header's sizes are unreliable).
function unzipFirst(buf: Buffer): Buffer {
  if (buf.subarray(0, 4).toString("latin1") !== "PK\x03\x04") return buf;
  const cd = buf.lastIndexOf(Buffer.from([0x50, 0x4b, 0x01, 0x02]));
  const method = buf.readUInt16LE(cd + 10);
  const clen = buf.readUInt32LE(cd + 20);
  const off = buf.readUInt32LE(cd + 42);
  const nlen = buf.readUInt16LE(off + 26);
  const elen = buf.readUInt16LE(off + 28);
  const data = buf.subarray(off + 30 + nlen + elen, off + 30 + nlen + elen + clen);
  return method === 0 ? data : inflateRawSync(data);
}

// MPE-54 close-out e2e: visual create → page setup → data table →
// preview → publish → tenant API generate → real PDF asserts.

const SAMPLE = { items: [{ title: "Widget", price: 19.5 }, { title: "Bolt", price: 0.75 }] };

test("template editor round-trip ends in a generated landscape PDF", async ({
  page,
  request,
}) => {
  const name = `e2e_tpl_${Date.now()}`;

  // New-template flow scoped to the demo client.
  await page.goto(`/templates/new?client=${DEMO_CLIENT.id}`);
  await page.getByLabel("Template name").fill(name);
  await page.locator(".ProseMirror").click();
  await page.keyboard.type("Invoice");

  // Page setup: A3 landscape + bottom margin + numbered footer.
  await page.getByRole("tab", { name: "Page setup" }).click();
  await page.locator("#page-size").selectOption("A3");
  await page.getByRole("radio", { name: "landscape" }).click();
  await page.getByLabel("bottom margin").fill("0.8");
  await page.locator("#page-footer").fill(
    '<div style="font-size:9px;text-align:center;width:100%">Page <span class="pageNumber"></span> of <span class="totalPages"></span></div>',
  );

  // Data-bound table via the Insert menu.
  await page.getByTitle("Insert merge field").click();
  await page.getByRole("button", { name: /Data table/ }).click();
  await page.getByLabel("Array path").fill("items");
  await page.getByLabel(/Columns/).fill("Item=title\nPrice=price");
  await page.getByRole("button", { name: "Insert table" }).click();
  await expect(page.locator("table.data-table")).toBeVisible();

  // Draft preview renders the bound rows + page setup.
  await page.getByRole("tab", { name: "Preview" }).click();
  const cm = page.locator(".cm-content").last();
  await cm.click();
  await page.keyboard.press("ControlOrMeta+A");
  await page.keyboard.insertText(JSON.stringify(SAMPLE));
  await page.getByRole("button", { name: "Render preview" }).click();
  await expect(page.locator('iframe[title="Template preview PDF"]')).toBeVisible({
    timeout: 30_000,
  });

  // Save → create v1; then publish v2 through the same editor.
  await page.getByRole("button", { name: "Create template" }).click();
  await page.waitForURL(/\/templates\/masters\//);
  // generate resolves documents[].template_id by master name, not id.
  await expect(page.getByRole("button", { name: "Publish new version" })).toBeVisible();
  await page.getByRole("button", { name: "Publish new version" }).click();
  await expect(page.getByText("Published v2")).toBeVisible();

  // Tenant API: generate a package against the new master.
  const created = await request.post(`${API_URL}/api/v1/packages/generate`, {
    headers: { "X-API-Key": TENANT_KEY, "Idempotency-Key": `e2e_${name}` },
    data: {
      customer_id: DEMO_CUSTOMER.id,
      package_name: name,
      documents: [
        { template_id: name, filename: "invoice", payload: SAMPLE },
      ],
    },
  });
  expect(created.ok()).toBeTruthy();
  const { request_id } = (await created.json()) as { request_id: string };

  let download = "";
  await expect
    .poll(
      async () => {
        const s = await request.get(
          `${API_URL}/api/v1/packages/${request_id}`,
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

  const url = download.startsWith("http") ? download : `${API_URL}${download}`;
  const dl = await request.get(url, { maxRedirects: 5 });
  expect(dl.ok()).toBeTruthy();
  const pdf = unzipFirst(await dl.body());
  // Assert the PDF magic and the landscape MediaBox (A3 ≈ 1190.55×841.89pt).
  expect(pdf.subarray(0, 4).toString()).toBe("%PDF");
  expect(pdf.toString("latin1")).toMatch(/MediaBox \[0 0 119[01]/);
});

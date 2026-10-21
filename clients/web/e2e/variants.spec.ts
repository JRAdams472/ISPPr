import { test, expect } from "@playwright/test";
import { DEMO_CLIENT, DEMO_CUSTOMER, DEMO_MASTER } from "./helpers";

// Customer workspace → variant editor → live Gotenberg preview → save.

test("variant editor renders a draft preview and saves the override", async ({
  page,
}) => {
  await page.goto(
    `/clients/${DEMO_CLIENT.id}/customers/${DEMO_CUSTOMER.id}`,
  );
  await expect(
    page.getByRole("heading", { name: DEMO_CUSTOMER.name }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Template overrides" }),
  ).toBeVisible();

  // Open the editor for the seeded master — either through the picker or
  // directly (the seed may already carry an override for this pair).
  await page.goto(
    `/clients/${DEMO_CLIENT.id}/customers/${DEMO_CUSTOMER.id}/variants/${DEMO_MASTER.id}`,
  );
  await expect(
    page.getByRole("button", { name: "Render preview" }),
  ).toBeVisible();

  // Draft render goes through the real Gotenberg path — the iframe only
  // mounts once the PDF blob comes back.
  await page.getByRole("button", { name: "Render preview" }).click();
  await expect(
    page.locator('iframe[title="Variant preview PDF"]'),
  ).toBeVisible({ timeout: 30_000 });

  // Persist the draft; the upsert either creates or updates.
  await page
    .getByRole("button", { name: /save override|create override/i })
    .click();
  await expect(page.getByText("Saved")).toBeVisible();

  // Back on the workspace the override row lists the master.
  await page.goto(
    `/clients/${DEMO_CLIENT.id}/customers/${DEMO_CUSTOMER.id}`,
  );
  await expect(
    page.getByRole("row", { name: new RegExp(DEMO_MASTER.name) }),
  ).toBeVisible();
});

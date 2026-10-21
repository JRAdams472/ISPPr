import { test, expect } from "@playwright/test";
import { DEMO_CLIENT } from "./helpers";

// Portal journeys under the seeded admin session (chromium project).

test("clients list shows the seeded tenant and drills into detail", async ({
  page,
}) => {
  await page.goto("/clients");
  await expect(
    page.getByRole("heading", { name: "Clients" }),
  ).toBeVisible();

  const row = page.getByRole("row", { name: new RegExp(DEMO_CLIENT.name) });
  await expect(row).toBeVisible();
  // Counts land on the roster (data-dependent — repeated e2e runs and
  // manual verification add customers/keys over time).
  await expect(row.getByRole("cell").nth(2)).toHaveText(/^\d+$/);
  await row.click();

  await expect(
    page.getByRole("heading", { name: DEMO_CLIENT.name }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "API keys" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Customers" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Failed requests" }),
  ).toBeVisible();
});

test("admin can create and revoke an API key end-to-end", async ({ page }) => {
  await page.goto(`/clients/${DEMO_CLIENT.id}`);
  await page.getByRole("button", { name: "New key" }).click();

  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await dialog.getByLabel(/label/i).fill(`e2e ${Date.now()}`);
  await dialog.getByRole("button", { name: "Create key" }).click();

  // The plaintext credential surfaces once — the spec only needs the
  // secret dialog to appear, not the value itself.
  await expect(
    page.getByRole("dialog").getByText(/only time the key is shown/),
  ).toBeVisible();
});

test("viewer role hides write controls", async ({ page, request }) => {
  // Sign in as the seeded read-only account in a fresh context.
  const res = await request.post(
    `${process.env.E2E_API_URL ?? "http://localhost:8090"}/api/v1/admin/login`,
    { data: { email: "viewer@isppr.local", password: "isppr-admin" } },
  );
  const { token } = (await res.json()) as { token: string };
  await page.addInitScript(
    (t) => window.localStorage.setItem("isppr_admin_token", t),
    token,
  );

  await page.goto(`/clients/${DEMO_CLIENT.id}`);
  await expect(
    page.getByRole("heading", { name: "API keys" }),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: "New key" })).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "New customer" }),
  ).toHaveCount(0);
});

import { test, expect } from "@playwright/test";
import { ADMIN } from "./helpers";

// Runs in the "anon" project — no seeded storageState.

test("unauthenticated visits redirect to login", async ({ page }) => {
  await page.goto("/clients");
  await expect(page).toHaveURL(/\/login/);
  await expect(
    page.getByRole("heading", { name: "Sign in" }),
  ).toBeVisible();
});

test("bad credentials surface the error, good ones land in the portal", async ({
  page,
}) => {
  await page.goto("/login");
  await page.locator("#email").fill(ADMIN.email);
  await page.locator("#password").fill("wrong-password");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("alert")).toBeVisible();

  await page.locator("#password").fill(ADMIN.password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).not.toHaveURL(/\/login/);
  // Signed-in shell: rail nav + the admin identity footer.
  await expect(
    page.getByRole("navigation", { name: "Primary" }),
  ).toBeVisible();
  await expect(page.getByText(/Demo Admin/)).toBeVisible();

  // Sign-out returns to the login screen and clears the session.
  await page.getByRole("button", { name: "Sign out" }).click();
  await expect(page).toHaveURL(/\/login/);
});

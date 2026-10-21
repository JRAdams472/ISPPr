import { test as setup, expect } from "@playwright/test";
import { mkdirSync, writeFileSync } from "node:fs";
import { dirname } from "node:path";
import { AUTH_FILE } from "../playwright.config";
import { API_URL, ADMIN } from "./helpers";

// Mint a portal session through the real login endpoint and persist it as
// the localStorage token RequireAuth reads — dependent projects start
// already signed in, so each spec doesn't pay the login round-trip.
setup("authenticate as the seeded admin", async ({ request, baseURL }) => {
  const res = await request.post(`${API_URL}/api/v1/admin/login`, {
    data: { email: ADMIN.email, password: ADMIN.password },
  });
  expect(res.ok()).toBeTruthy();
  const { token } = (await res.json()) as { token: string };
  expect(token).toBeTruthy();

  mkdirSync(dirname(AUTH_FILE), { recursive: true });
  writeFileSync(
    AUTH_FILE,
    JSON.stringify({
      cookies: [],
      origins: [
        {
          origin: baseURL ?? "http://localhost:3100",
          localStorage: [{ name: "isppr_admin_token", value: token }],
        },
      ],
    }),
  );
});

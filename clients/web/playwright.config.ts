import { defineConfig, devices } from "@playwright/test";

export const AUTH_FILE = "e2e/.auth/user.json";

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  // CI emits a JUnit report alongside the HTML report so per-test
  // durations are identifiable from the uploaded artifact.
  reporter: process.env.CI
    ? [["html"], ["junit", { outputFile: "test-results/junit.xml" }]]
    : "html",
  use: {
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:3100",
    trace: "on-first-retry",
  },
  projects: [
    { name: "setup", testMatch: /auth\.setup\.ts/ },
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"], storageState: AUTH_FILE },
      testIgnore: [/auth\.spec\.ts/, /api\.spec\.ts/],
      dependencies: ["setup"],
    },
    {
      // Unauthenticated journeys — login form, redirect gating.
      name: "anon",
      use: { ...devices["Desktop Chrome"], storageState: { cookies: [], origins: [] } },
      testMatch: /auth\.spec\.ts/,
    },
    {
      // API-level flows (X-API-Key generate→status→download) need no
      // browser state; they still run in a browser-less project for the
      // shared runner/report.
      name: "api",
      use: { ...devices["Desktop Chrome"], storageState: { cookies: [], origins: [] } },
      testMatch: /api\.spec\.ts/,
    },
  ],
});

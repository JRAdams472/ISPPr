import path from "node:path";
import { defineConfig } from "vitest/config";

export default defineConfig({
  resolve: {
    alias: { "@": path.resolve(__dirname, ".") },
  },
  test: {
    environment: "jsdom",
    // Playwright specs live in e2e/ — they're run by `npm run test:e2e`,
    // not vitest.
    exclude: ["e2e/**", "node_modules/**"],
    coverage: {
      provider: "v8",
      reporter: ["text", "lcov"],
      include: ["lib/**", "app/**/*.tsx", "components/**"],
      exclude: ["**/*.test.*", "**/__tests__/**"],
    },
  },
});

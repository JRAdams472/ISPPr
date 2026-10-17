import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    environment: "jsdom",
    coverage: {
      provider: "v8",
      reporter: ["text", "lcov"],
      include: ["lib/**", "app/**/*.tsx", "components/**"],
      exclude: ["**/*.test.*", "**/__tests__/**"],
    },
  },
});

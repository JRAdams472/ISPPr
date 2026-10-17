import { afterEach, describe, expect, it, vi } from "vitest";
import { health } from "../lib/api";

describe("health", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("returns parsed JSON on success", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        new Response(JSON.stringify({ status: "ok" }), { status: 200 }),
      ),
    );
    await expect(health()).resolves.toEqual({ status: "ok" });
  });

  it("throws with the HTTP status on failure", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response("boom", { status: 502 })),
    );
    await expect(health()).rejects.toThrow("502");
  });
});

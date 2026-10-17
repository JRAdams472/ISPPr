import { describe, expect, it } from "vitest";
import { archiveFilename } from "../lib/naming";

describe("archiveFilename", () => {
  it("joins package name and request id with a .zip suffix", () => {
    expect(archiveFilename("Q4_Onboarding_Pack", "guid_b3f982c1")).toBe(
      "Q4_Onboarding_Pack_guid_b3f982c1.zip",
    );
  });
});

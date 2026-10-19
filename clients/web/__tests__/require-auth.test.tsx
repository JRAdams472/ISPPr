import { cleanup, render, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { getToken, setToken } from "@/lib/api/client";
import { Providers } from "../app/providers";
import { RequireAuth } from "../components/require-auth";

const replace = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
}));

function renderGate() {
  return render(
    <Providers>
      <RequireAuth>
        <div data-testid="protected">secret</div>
      </RequireAuth>
    </Providers>,
  );
}

describe("RequireAuth", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    replace.mockClear();
    setToken("");
  });

  it("redirects to /login with no stored token", async () => {
    const { queryByTestId } = renderGate();
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/login"));
    expect(queryByTestId("protected")).toBeNull();
  });

  it("clears a dead session and redirects on 401", async () => {
    setToken("stale");
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response("{}", { status: 401 })),
    );
    const { queryByTestId } = renderGate();
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/login"));
    expect(getToken()).toBe("");
    expect(queryByTestId("protected")).toBeNull();
  });

  it("renders children once the session validates", async () => {
    setToken("good");
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async () =>
          new Response(JSON.stringify({ id: "u1", role: "admin" }), {
            status: 200,
          }),
      ),
    );
    const { findByTestId } = renderGate();
    expect(await findByTestId("protected")).toBeTruthy();
    expect(replace).not.toHaveBeenCalled();
  });
});

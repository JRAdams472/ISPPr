import { cleanup, fireEvent, render, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { getToken, setToken } from "@/lib/api/client";
import Login from "../app/login/page";
import { Providers } from "../app/providers";

const replace = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
}));

function renderLogin() {
  return render(
    <Providers>
      <Login />
    </Providers>,
  );
}

describe("login page", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    replace.mockClear();
    setToken("");
  });

  it("renders the form", () => {
    const { getByLabelText, getByRole } = renderLogin();
    expect(getByLabelText("Email")).toBeTruthy();
    expect(getByLabelText("Password")).toBeTruthy();
    expect(getByRole("button", { name: "Sign in" })).toBeTruthy();
  });

  it("stores the session token and redirects on success", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async () =>
          new Response(
            JSON.stringify({ token: "sess_1", user: { id: "u1" } }),
            { status: 200 },
          ),
      ),
    );
    const { getByLabelText, getByRole } = renderLogin();
    fireEvent.change(getByLabelText("Email"), {
      target: { value: "admin@isppr.local" },
    });
    fireEvent.change(getByLabelText("Password"), {
      target: { value: "pw" },
    });
    fireEvent.click(getByRole("button", { name: "Sign in" }));

    await waitFor(() => expect(replace).toHaveBeenCalledWith("/"));
    expect(getToken()).toBe("sess_1");
  });

  it("shows the problem detail on failure", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async () =>
          new Response(
            JSON.stringify({ status: 401, detail: "invalid credentials" }),
            { status: 401 },
          ),
      ),
    );
    const { getByLabelText, getByRole, findByRole } = renderLogin();
    fireEvent.change(getByLabelText("Email"), {
      target: { value: "a@b.c" },
    });
    fireEvent.change(getByLabelText("Password"), {
      target: { value: "bad" },
    });
    fireEvent.click(getByRole("button", { name: "Sign in" }));

    const alert = await findByRole("alert");
    expect(alert.textContent).toBe("invalid credentials");
    expect(getToken()).toBe("");
    expect(replace).not.toHaveBeenCalled();
  });
});

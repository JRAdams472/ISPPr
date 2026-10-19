import { cleanup, render } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { Providers } from "../app/providers";
import { Shell } from "../components/shell";
import { NAV, navActive } from "../lib/nav";

// usePathname is a router hook — stub it per test.
vi.mock("next/navigation", () => ({
  usePathname: vi.fn(() => "/clients"),
  useRouter: () => ({ replace: vi.fn() }),
}));

function renderShell(children: React.ReactNode) {
  vi.stubGlobal(
    "fetch",
    vi.fn(
      async () =>
        new Response(
          JSON.stringify({ id: "u1", name: "Demo Admin", role: "admin" }),
          { status: 200 },
        ),
    ),
  );
  return render(<Providers><Shell>{children}</Shell></Providers>);
}

describe("Shell", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("renders every primary nav item", () => {
    const { getByRole } = renderShell(<div data-testid="child" />);
    const nav = getByRole("navigation", { name: "Primary" });
    for (const item of NAV) {
      expect(
        nav.querySelector(`a[href="${item.href}"]`)?.textContent,
      ).toBe(item.label);
    }
  });

  it("marks the current section as the current page", () => {
    const { getByRole } = renderShell(<div />);
    const clients = getByRole("link", { name: "Clients" });
    const requests = getByRole("link", { name: "Requests" });
    expect(clients.getAttribute("aria-current")).toBe("page");
    expect(requests.getAttribute("aria-current")).toBeNull();
  });

  it("renders children in the content area", () => {
    const { getByTestId } = renderShell(<div data-testid="child" />);
    expect(getByTestId("child")).toBeTruthy();
  });
});

describe("navActive", () => {
  it("matches the dashboard exactly", () => {
    expect(navActive("/", "/")).toBe(true);
    expect(navActive("/clients", "/")).toBe(false);
  });

  it("matches section prefixes", () => {
    expect(navActive("/clients/cli_1", "/clients")).toBe(true);
    expect(navActive("/usage", "/requests")).toBe(false);
  });
});

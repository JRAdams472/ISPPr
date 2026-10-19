import { cleanup, render } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { Shell } from "../components/shell";
import { NAV, navActive } from "../lib/nav";

// usePathname is a router hook — stub it per test.
vi.mock("next/navigation", () => ({
  usePathname: vi.fn(() => "/clients"),
}));

describe("Shell", () => {
  afterEach(cleanup);

  it("renders every primary nav item", () => {
    const { getByRole } = render(
      <Shell>
        <div data-testid="child" />
      </Shell>,
    );
    const nav = getByRole("navigation", { name: "Primary" });
    for (const item of NAV) {
      expect(
        nav.querySelector(`a[href="${item.href}"]`)?.textContent,
      ).toBe(item.label);
    }
  });

  it("marks the current section as the current page", () => {
    const { getByRole } = render(
      <Shell>
        <div />
      </Shell>,
    );
    const clients = getByRole("link", { name: "Clients" });
    const requests = getByRole("link", { name: "Requests" });
    expect(clients.getAttribute("aria-current")).toBe("page");
    expect(requests.getAttribute("aria-current")).toBeNull();
  });

  it("renders children in the content area", () => {
    const { getByTestId } = render(
      <Shell>
        <div data-testid="child" />
      </Shell>,
    );
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

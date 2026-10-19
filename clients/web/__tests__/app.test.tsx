import { cleanup, render } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

// next/font performs a build-time font fetch — stub it for jsdom.
vi.mock("next/font/google", () => ({
  Geist: () => ({ variable: "--font-geist-sans", className: "" }),
  Geist_Mono: () => ({ variable: "--font-geist-mono", className: "" }),
}));

import RootLayout from "../app/layout";
import Dashboard from "../app/(portal)/page";

describe("app shell", () => {
  afterEach(cleanup);

  it("renders the dashboard page", () => {
    const { getByRole } = render(<Dashboard />);
    expect(getByRole("heading", { name: "Dashboard" })).toBeTruthy();
  });

  it("wraps children in html/body", () => {
    const { container } = render(
      <RootLayout>
        <div data-testid="child" />
      </RootLayout>,
    );
    // React hoists <html>/<body> attrs onto the real document root.
    expect(document.documentElement.lang).toBe("en");
    expect(container.querySelector("[data-testid=child]")).toBeTruthy();
  });
});

import { cleanup, render } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import RootLayout from "../app/layout";
import Home from "../app/page";

describe("app shell", () => {
  afterEach(cleanup);

  it("renders the landing page", () => {
    const { getByRole, getByText } = render(<Home />);
    expect(getByRole("heading", { name: "ISPPr" })).toBeTruthy();
    expect(getByText(/Document Generation Engine/)).toBeTruthy();
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

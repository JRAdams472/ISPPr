import { cleanup, render } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  usePathname: vi.fn(() => "/"),
}));

import PortalLayout from "../app/(portal)/layout";
import Clients from "../app/(portal)/clients/page";
import Requests from "../app/(portal)/requests/page";
import Templates from "../app/(portal)/templates/page";
import Usage from "../app/(portal)/usage/page";
import { Providers } from "../app/providers";

describe("portal pages", () => {
  afterEach(cleanup);

  it.each([
    [Clients, "Clients"],
    [Templates, "Templates"],
    [Requests, "Requests"],
    [Usage, "Usage"],
  ])("renders the %s stub with its header", (Page, title) => {
    const { getByRole } = render(<Page />);
    expect(getByRole("heading", { name: title })).toBeTruthy();
  });

  it("wraps pages in the shell with nav", () => {
    const { getByRole } = render(
      <PortalLayout>
        <Clients />
      </PortalLayout>,
    );
    expect(getByRole("navigation", { name: "Primary" })).toBeTruthy();
    expect(getByRole("heading", { name: "Clients" })).toBeTruthy();
  });

  it("Providers supplies a query client", () => {
    const { getByText } = render(
      <Providers>
        <div>inside</div>
      </Providers>,
    );
    expect(getByText("inside")).toBeTruthy();
  });
});

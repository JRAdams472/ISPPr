import { cleanup, render } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  usePathname: vi.fn(() => "/"),
  useRouter: () => ({ replace: vi.fn(), push: vi.fn() }),
  useParams: () => ({}),
}));

// The auth gate is covered by its own test — render it through here.
vi.mock("@/components/require-auth", () => ({
  RequireAuth: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

import PortalLayout from "../app/(portal)/layout";
import Clients from "../app/(portal)/clients/page";
import Requests from "../app/(portal)/requests/page";
import Templates from "../app/(portal)/templates/page";
import Usage from "../app/(portal)/usage/page";
import { Providers } from "../app/providers";

// Minimal fetch router: admin session + empty lists for every resource.
function stubFetch() {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string) => {
      if (url.includes("/me")) {
        return new Response(
          JSON.stringify({ id: "u1", name: "A", role: "admin" }),
          { status: 200 },
        );
      }
      return new Response("[]", { status: 200 });
    }),
  );
}

describe("portal pages", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it.each([
    [Templates, "Templates"],
    [Requests, "Requests"],
    [Usage, "Usage"],
  ])("renders the %s stub with its header", (Page, title) => {
    stubFetch();
    const { getByRole } = render(
      <Providers>
        <Page />
      </Providers>,
    );
    expect(getByRole("heading", { name: title })).toBeTruthy();
  });

  it("renders the clients list behind its header", async () => {
    stubFetch();
    const { getByRole, findByText } = render(
      <Providers>
        <Clients />
      </Providers>,
    );
    expect(getByRole("heading", { name: "Clients" })).toBeTruthy();
    expect(await findByText("No clients yet")).toBeTruthy();
  });

  it("wraps pages in the shell with nav", () => {
    stubFetch();
    const { getByRole } = render(
      <Providers>
        <PortalLayout>
          <Clients />
        </PortalLayout>
      </Providers>,
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

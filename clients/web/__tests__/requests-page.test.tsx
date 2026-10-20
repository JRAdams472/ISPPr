import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

const params = { id: "req_1" };

vi.mock("next/navigation", () => ({
  usePathname: vi.fn(() => "/requests"),
  useRouter: () => ({ replace: vi.fn(), push: vi.fn() }),
  useParams: () => params,
}));

vi.mock("@/components/require-auth", () => ({
  RequireAuth: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

import RequestDetail from "../app/(portal)/requests/[id]/page";
import Requests from "../app/(portal)/requests/page";
import { Providers } from "../app/providers";

const requestRow = {
  id: "req_1",
  client_id: "cli_a",
  customer_id: "cust_a",
  package_name: "invoices",
  status: "COMPLETED",
  metrics: {
    total_documents: 2,
    parsing_duration_ms: 5,
    rendering_duration_ms: 120,
    compression_duration_ms: 10,
    total_duration_ms: 140,
  },
  webhook_status: "DELIVERED",
  webhook_log: [{ at: "2026-10-20T10:00:00Z", attempt: 1, code: 200 }],
  created_at: "2026-10-20T10:00:00Z",
};

function stubFetch(detail: Record<string, unknown> = {}) {
  const calls: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string) => {
      calls.push(url);
      if (url.includes("/me")) {
        return new Response(
          JSON.stringify({ id: "u1", name: "A", role: "viewer" }),
          { status: 200 },
        );
      }
      if (url.includes("/requests/req_1")) {
        return new Response(JSON.stringify({ ...requestRow, ...detail }), {
          status: 200,
        });
      }
      if (url.includes("/requests")) {
        return new Response(JSON.stringify([requestRow]), { status: 200 });
      }
      return new Response("[]", { status: 200 });
    }),
  );
  return calls;
}

describe("requests monitor", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("lists requests with status and duration", async () => {
    stubFetch();
    render(
      <Providers>
        <Requests />
      </Providers>,
    );
    expect(await screen.findByText("invoices")).toBeTruthy();
    expect(screen.getByText("req_1")).toBeTruthy();
    expect(screen.getByText("140ms")).toBeTruthy();
    expect(screen.getAllByText("completed").length).toBeGreaterThan(0);
  });

  it("shows the empty state when no requests match", async () => {
    const calls = stubFetch();
    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string) => {
        calls.push(url);
        if (url.includes("/me")) {
          return new Response(
            JSON.stringify({ id: "u1", name: "A", role: "viewer" }),
            { status: 200 },
          );
        }
        if (url.includes("/clients") && !url.includes("/requests")) {
          return new Response(
            JSON.stringify([
              { id: "cli_a", name: "Acme", status: "active", created_at: "" },
            ]),
            { status: 200 },
          );
        }
        return new Response("[]", { status: 200 });
      }),
    );
    render(
      <Providers>
        <Requests />
      </Providers>,
    );
    expect(await screen.findByText("No requests match")).toBeTruthy();
    expect(await screen.findByText("Acme")).toBeTruthy();

    fireEvent.change(screen.getByLabelText(/Client/), {
      target: { value: "cli_a" },
    });
    expect(
      calls.some((u) => u.includes("/requests") && u.includes("client_id=cli_a")),
    ).toBe(true);
  });

  it("shows the error state and retries", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string) => {
        if (url.includes("/me")) {
          return new Response(
            JSON.stringify({ id: "u1", name: "A", role: "viewer" }),
            { status: 200 },
          );
        }
        return new Response(JSON.stringify({ detail: "db down" }), {
          status: 500,
        });
      }),
    );
    // Providers retries 5xx by design — assert the terminal error surface
    // with a retry-free client instead of waiting out the backoff.
    const qc = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={qc}>
        <Requests />
      </QueryClientProvider>,
    );
    expect(await screen.findByText("Something went wrong")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Try again" })).toBeTruthy();
  });

  it("passes the status filter to the API", async () => {
    const calls = stubFetch();
    render(
      <Providers>
        <Requests />
      </Providers>,
    );
    await screen.findByText("invoices");
    fireEvent.change(screen.getByLabelText(/Status/), {
      target: { value: "FAILED" },
    });
    await screen.findByText("invoices");
    expect(
      calls.some((u) => u.includes("/requests") && u.includes("status=FAILED")),
    ).toBe(true);
  });

  it("renders the detail waterfall, webhook log, and download link", async () => {
    stubFetch({
      results: {
        output_type: "ZIP",
        filename: "invoices_req_1.zip",
        download_url: "http://minio/presigned/abc",
      },
    });
    render(
      <Providers>
        <RequestDetail />
      </Providers>,
    );
    expect(await screen.findByText("Rendering")).toBeTruthy();
    expect(screen.getByText("120ms")).toBeTruthy();
    expect(screen.getByText("140ms")).toBeTruthy();
    expect(screen.getByText("2 documents in this package")).toBeTruthy();
    expect(screen.getByText("HTTP 200")).toBeTruthy();
    const dl = screen.getByRole("link", { name: /Download output/ });
    expect(dl.getAttribute("href")).toBe("http://minio/presigned/abc");
  });

  it("hides download and shows the error on a failed request", async () => {
    stubFetch({
      status: "FAILED",
      error: "render blew up",
      metrics: {}, // failed rows can carry an empty metrics object
      webhook_status: "EXHAUSTED",
      webhook_log: [
        { at: "2026-10-20T10:00:00Z", attempt: 1, code: 500 },
        { at: "2026-10-20T10:00:05Z", attempt: 2, error: "connection refused" },
      ],
    });
    render(
      <Providers>
        <RequestDetail />
      </Providers>,
    );
    expect(await screen.findByText("render blew up")).toBeTruthy();
    // Empty metrics → no waterfall section at all.
    expect(screen.queryByText("Render phases")).toBeNull();
    expect(screen.queryByRole("link", { name: /Download output/ })).toBeNull();
    expect(screen.getByText("HTTP 500")).toBeTruthy();
    expect(screen.getByText("connection refused")).toBeTruthy();
  });
});

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// Page-shell coverage for /templates/new and /templates/masters/[id] —
// the editor itself is replaced with a props-dump stub; these tests own
// the wiring: client carry-through, role gate, load/error states.

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useParams: () => ({ id: "tmpl_1" }),
  usePathname: () => "/templates",
  useSearchParams: () => new URLSearchParams("client=cli_1"),
}));

const editorProps = vi.fn();
vi.mock("@/components/templates/template-editor", () => ({
  TemplateEditor: (props: Record<string, unknown>) => {
    editorProps(props);
    return <div data-testid="editor-stub" />;
  },
}));

import NewTemplatePage from "../app/(portal)/templates/new/page";
import MasterEditorPage from "../app/(portal)/templates/masters/[id]/page";

function qc() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

const MASTER = {
  id: "tmpl_1",
  client_id: "cli_9",
  name: "invoice",
  version: 4,
  html: "<p>x</p>",
  css: "",
  defaults: {},
  created_at: "2026-01-01T00:00:00Z",
};

function stubFetch(opts: { role?: string; master?: unknown; status?: number } = {}) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string) => {
      if (url.includes("/me"))
        return new Response(
          JSON.stringify({ id: "u1", name: "A", role: opts.role ?? "admin" }),
          { status: 200 },
        );
      if (url.includes("/template-masters/tmpl_1")) {
        if (opts.status === 404)
          return new Response(
            JSON.stringify({ status: 404, detail: "not found" }),
            { status: 404 },
          );
        return new Response(JSON.stringify(opts.master ?? MASTER), {
          status: 200,
        });
      }
      if (url.includes("/clients"))
        return new Response(
          JSON.stringify([{ id: "cli_1", name: "Demo", status: "active" }]),
          { status: 200 },
        );
      return new Response("[]", { status: 200 });
    }),
  );
}

describe("template editor pages", () => {
  beforeEach(() => editorProps.mockClear());
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("new: carries the ?client param and admin role into the editor", async () => {
    stubFetch();
    render(
      <QueryClientProvider client={qc()}>
        <NewTemplatePage />
      </QueryClientProvider>,
    );
    await waitFor(() => expect(editorProps).toHaveBeenCalled());
    expect(editorProps.mock.calls[0][0]).toMatchObject({
      clientId: "cli_1",
      master: null,
      isAdmin: true,
    });
  });

  it("edit: loads the master and marks admin editable", async () => {
    stubFetch();
    render(
      <QueryClientProvider client={qc()}>
        <MasterEditorPage />
      </QueryClientProvider>,
    );
    await waitFor(() => expect(editorProps).toHaveBeenCalled());
    expect(editorProps.mock.calls[0][0]).toMatchObject({
      clientId: "cli_9",
      isAdmin: true,
    });
    expect((editorProps.mock.calls[0][0].master as { id: string }).id).toBe(
      "tmpl_1",
    );
  });

  it("edit: viewer role is not editable", async () => {
    stubFetch({ role: "viewer" });
    render(
      <QueryClientProvider client={qc()}>
        <MasterEditorPage />
      </QueryClientProvider>,
    );
    await waitFor(() => expect(editorProps).toHaveBeenCalled());
    expect(editorProps.mock.calls[0][0].isAdmin).toBe(false);
  });

  it("edit: missing master shows the error state", async () => {
    stubFetch({ status: 404 });
    render(
      <QueryClientProvider client={qc()}>
        <MasterEditorPage />
      </QueryClientProvider>,
    );
    expect(await screen.findByRole("alert")).toBeTruthy();
    expect(editorProps).not.toHaveBeenCalled();
  });
});

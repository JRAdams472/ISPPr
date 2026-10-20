import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const push = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push, replace: vi.fn() }),
  usePathname: () => "/clients/cli_1/customers/cust_1/variants/tmpl_1",
  useParams: () => ({
    id: "cli_1",
    customerId: "cust_1",
    masterId: "tmpl_1",
  }),
}));

// CodeMirror → plain textarea so jsdom can drive the editors.
vi.mock("@uiw/react-codemirror", () => ({
  default: ({
    value,
    onChange,
    editable = true,
  }: {
    value: string;
    onChange?: (v: string) => void;
    editable?: boolean;
  }) => (
    <textarea
      data-testid="codemirror"
      value={value}
      readOnly={!editable}
      onChange={(e) => onChange?.(e.target.value)}
    />
  ),
}));

import { VariantEditor } from "../components/variants/variant-editor";
import VariantEditorPage from "../app/(portal)/clients/[id]/customers/[customerId]/variants/[masterId]/page";
import CustomerWorkspace from "../app/(portal)/clients/[id]/customers/[customerId]/page";

const ME = { id: "u1", email: "a@b.c", name: "Admin", role: "admin" };
const MASTER = {
  id: "tmpl_1",
  client_id: "cli_1",
  name: "invoice",
  version: 2,
  html: "<h1>{{doc_title}}</h1>{{#if blocks.footer}}<p>F</p>{{/if}}",
  css: "h1{}",
  defaults: {
    toggles: { footer: true, watermark: false },
    branding: { logo: "s3://brand/logo.png" },
  },
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};
const CUSTOMER = {
  id: "cust_1",
  client_id: "cli_1",
  name: "Cust One",
  created_at: "2026-01-02T00:00:00Z",
  updated_at: "2026-01-02T00:00:00Z",
};
const VARIANT = {
  id: "var_1",
  master_id: "tmpl_1",
  master_name: "invoice",
  customer_id: "cust_1",
  overlay: { toggles: { footer: false }, css_patch: ".p{color:blue}" },
  updated_at: "2026-01-03T00:00:00Z",
};

function wrapper() {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  function Wrap({ children }: { children: React.ReactNode }) {
    return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
  }
  return Wrap;
}

function stubFetch(handlers: Record<string, (init?: RequestInit) => Response>) {
  const calls: { url: string; method: string; body?: unknown }[] = [];
  const mock = vi.fn(async (url: string, init?: RequestInit) => {
    const method = init?.method ?? "GET";
    calls.push({
      url,
      method,
      body: init?.body ? JSON.parse(init.body as string) : undefined,
    });
    for (const match of Object.keys(handlers)) {
      if (url.includes(match)) return handlers[match](init);
    }
    return new Response(JSON.stringify({ status: 404, title: "nf" }), {
      status: 404,
    });
  });
  vi.stubGlobal("fetch", mock);
  return calls;
}

const json200 = (v: unknown) =>
  new Response(JSON.stringify(v), { status: 200 });
const nf = () =>
  new Response(JSON.stringify({ status: 404, title: "not found" }), {
    status: 404,
  });

function baseRoutes(variant: Response = nf()) {
  return {
    "/me": () => json200(ME),
    "/template-variants/preview": () =>
      new Response("%PDF-1.4 fake", {
        status: 200,
        headers: { "content-type": "application/pdf" },
      }),
    "/template-variants?": () => variant,
    "/template-variants": () => json200(VARIANT),
    "/template-masters/tmpl_1": () => json200(MASTER),
    "/customers/cust_1/variants": () => json200([VARIANT]),
    "/customers/cust_1": () => json200(CUSTOMER),
  };
}

describe("VariantEditor", () => {
  beforeEach(() => {
    URL.createObjectURL = vi.fn(() => "blob:fake");
    URL.revokeObjectURL = vi.fn();
  });
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    push.mockClear();
  });

  it("seeds from the saved variant: toggles, branding defaults, css patch", async () => {
    stubFetch(baseRoutes(json200(VARIANT)));
    render(<VariantEditor clientId="cli_1" customerId="cust_1" masterId="tmpl_1" />, {
      wrapper: wrapper(),
    });
    // footer overridden off (saved variant), watermark untouched
    const footer = await screen.findByLabelText("footer");
    expect((footer as HTMLInputElement).checked).toBe(false);
    const watermark = screen.getByLabelText("watermark");
    expect((watermark as HTMLInputElement).checked).toBe(false);
    // branding row shows the master default as placeholder
    const logo = screen.getByLabelText("Branding path for logo");
    expect((logo as HTMLInputElement).placeholder).toBe("s3://brand/logo.png");
    // css patch seeded into the editor
    const editors = await screen.findAllByTestId("codemirror");
    expect(editors.some((e) => (e as HTMLTextAreaElement).value.includes(".p{color:blue}"))).toBe(true);
    expect(screen.getByText("overridden · reset")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Save override" })).toBeTruthy();
  });

  it("404 variant renders an empty overlay with Create override", async () => {
    stubFetch(baseRoutes());
    render(<VariantEditor clientId="cli_1" customerId="cust_1" masterId="tmpl_1" />, {
      wrapper: wrapper(),
    });
    const footer = await screen.findByLabelText("footer");
    expect((footer as HTMLInputElement).checked).toBe(true); // master default
    expect(screen.getByRole("button", { name: "Create override" })).toBeTruthy();
    expect(screen.queryByText("overridden · reset")).toBeNull();
  });

  it("saves the composed overlay via PUT", async () => {
    const calls = stubFetch(baseRoutes());
    render(<VariantEditor clientId="cli_1" customerId="cust_1" masterId="tmpl_1" />, {
      wrapper: wrapper(),
    });
    const footer = await screen.findByLabelText("footer");
    fireEvent.click(footer); // override to false
    const logo = screen.getByLabelText("Branding path for logo");
    fireEvent.change(logo, { target: { value: "s3://cust/logo.png" } });
    fireEvent.click(screen.getByRole("button", { name: "Create override" }));
    await waitFor(() =>
      expect(calls.some((c) => c.method === "PUT")).toBe(true),
    );
    const put = calls.find((c) => c.method === "PUT");
    expect(put?.url).toContain("/template-variants");
    expect(put?.body).toEqual({
      master_id: "tmpl_1",
      customer_id: "cust_1",
      overlay: {
        toggles: { footer: false },
        branding: { logo: "s3://cust/logo.png" },
      },
    });
  });

  it("previews the draft overlay through the PDF endpoint", async () => {
    const calls = stubFetch(baseRoutes());
    render(<VariantEditor clientId="cli_1" customerId="cust_1" masterId="tmpl_1" />, {
      wrapper: wrapper(),
    });
    await screen.findByLabelText("footer");
    fireEvent.click(screen.getByRole("button", { name: "Render preview" }));
    await waitFor(() =>
      expect(
        calls.some((c) => c.url.includes("/template-variants/preview")),
      ).toBe(true),
    );
    const prev = calls.find((c) => c.url.includes("/template-variants/preview"));
    expect(prev?.method).toBe("POST");
    expect((prev?.body as { master_id: string }).master_id).toBe("tmpl_1");
    expect((prev?.body as { data: Record<string, unknown> }).data).toEqual({
      doc_title: "Sample",
    });
    const iframe = await screen.findByTitle("Variant preview PDF");
    expect(iframe.getAttribute("src")).toBe("blob:fake");
  });

  it("rejects invalid sample JSON before calling the API", async () => {
    const calls = stubFetch(baseRoutes());
    render(<VariantEditor clientId="cli_1" customerId="cust_1" masterId="tmpl_1" />, {
      wrapper: wrapper(),
    });
    await screen.findByLabelText("footer");
    const editors = screen.getAllByTestId("codemirror");
    const dataEditor = editors.find(
      (e) => (e as HTMLTextAreaElement).value.includes("doc_title"),
    ) as HTMLTextAreaElement;
    fireEvent.change(dataEditor, { target: { value: "{not json" } });
    fireEvent.click(screen.getByRole("button", { name: "Render preview" }));
    expect(await screen.findByRole("alert")).toBeTruthy();
    expect(calls.some((c) => c.url.includes("/preview"))).toBe(false);
  });

  it("reset restores a toggled block to the master default", async () => {
    const calls = stubFetch(baseRoutes(json200(VARIANT)));
    render(<VariantEditor clientId="cli_1" customerId="cust_1" masterId="tmpl_1" />, {
      wrapper: wrapper(),
    });
    const footer = await screen.findByLabelText("footer");
    expect((footer as HTMLInputElement).checked).toBe(false);
    fireEvent.click(screen.getByText("overridden · reset"));
    expect((footer as HTMLInputElement).checked).toBe(true);
    fireEvent.click(screen.getByRole("button", { name: "Save override" }));
    await waitFor(() => expect(calls.some((c) => c.method === "PUT")).toBe(true));
    const put = calls.find((c) => c.method === "PUT");
    expect(
      (put?.body as { overlay: { toggles?: Record<string, boolean> } })
        .overlay.toggles,
    ).toBeUndefined();
  });

  it("adds a custom branding key and saves it", async () => {
    const calls = stubFetch(baseRoutes());
    render(<VariantEditor clientId="cli_1" customerId="cust_1" masterId="tmpl_1" />, {
      wrapper: wrapper(),
    });
    await screen.findByLabelText("footer");
    fireEvent.change(screen.getByLabelText("New branding key"), {
      target: { value: "icon" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Add key" }));
    const icon = await screen.findByLabelText("Branding path for icon");
    fireEvent.change(icon, { target: { value: "s3://cust/icon.png" } });
    fireEvent.click(screen.getByRole("button", { name: "Create override" }));
    await waitFor(() => expect(calls.some((c) => c.method === "PUT")).toBe(true));
    const put = calls.find((c) => c.method === "PUT");
    expect(
      (put?.body as { overlay: { branding: Record<string, string> } }).overlay
        .branding,
    ).toEqual({ icon: "s3://cust/icon.png" });
  });

  it("shows an error when the master fails to load", async () => {
    stubFetch({
      ...baseRoutes(),
      "/template-masters/tmpl_1": () =>
        new Response(JSON.stringify({ status: 500, detail: "db down" }), {
          status: 500,
        }),
    });
    render(<VariantEditor clientId="cli_1" customerId="cust_1" masterId="tmpl_1" />, {
      wrapper: wrapper(),
    });
    expect(await screen.findByRole("alert")).toBeTruthy();
    expect(screen.getByText(/db down/)).toBeTruthy();
  });

  it("viewer role gets a read-only editor", async () => {
    stubFetch({ ...baseRoutes(), "/me": () => json200({ ...ME, role: "viewer" }) });
    render(<VariantEditor clientId="cli_1" customerId="cust_1" masterId="tmpl_1" />, {
      wrapper: wrapper(),
    });
    const footer = await screen.findByLabelText("footer");
    expect((footer as HTMLInputElement).disabled).toBe(true);
    expect(screen.queryByRole("button", { name: "Create override" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Save override" })).toBeNull();
  });
});

describe("CustomerWorkspace", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    push.mockClear();
  });

  it("lists saved overrides with master names and links to the editor", async () => {
    stubFetch({
      ...baseRoutes(json200(VARIANT)),
      "/clients/cli_1/template-masters": () => json200([MASTER]),
    });
    render(<CustomerWorkspace />, { wrapper: wrapper() });
    expect(await screen.findByText("Cust One")).toBeTruthy();
    expect(await screen.findByText("invoice")).toBeTruthy();
    expect(screen.getByText(/1 toggle\(s\)/)).toBeTruthy();
    const link = screen.getByRole("link", { name: "Edit" });
    expect(link.getAttribute("href")).toBe(
      "/clients/cli_1/customers/cust_1/variants/tmpl_1",
    );
  });

  it("empty overrides show guidance; picker lists masters for admins", async () => {
    stubFetch({
      ...baseRoutes(),
      "/customers/cust_1/variants": () => json200([]),
      "/clients/cli_1/template-masters": () => json200([MASTER]),
    });
    render(<CustomerWorkspace />, { wrapper: wrapper() });
    expect(await screen.findByText("No overrides yet")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "New override" }));
    const sel = await screen.findByLabelText("Choose a template master");
    fireEvent.change(sel, { target: { value: "tmpl_1" } });
    expect(push).toHaveBeenCalledWith(
      "/clients/cli_1/customers/cust_1/variants/tmpl_1",
    );
  });

  it("viewer sees overrides but no New override control", async () => {
    stubFetch({
      ...baseRoutes(json200(VARIANT)),
      "/me": () => json200({ ...ME, role: "viewer" }),
    });
    render(<CustomerWorkspace />, { wrapper: wrapper() });
    expect(await screen.findByText("invoice")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "New override" })).toBeNull();
    expect(screen.getByRole("link", { name: "View" })).toBeTruthy();
  });
});

describe("VariantEditorPage route", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("renders the editor inside the page chrome", async () => {
    stubFetch(baseRoutes());
    render(<VariantEditorPage />, { wrapper: wrapper() });
    expect(
      await screen.findByText("Cust One — template override"),
    ).toBeTruthy();
    expect(await screen.findByLabelText("footer")).toBeTruthy();
  });
});

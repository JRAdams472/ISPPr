import { Editor } from "@tiptap/core";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { editorExtensions } from "../components/templates/extensions";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useParams: () => ({}),
  usePathname: () => "/templates",
  useSearchParams: () => new URLSearchParams(),
}));

// CodeMirror → plain textarea so jsdom can drive source/sample panes.
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
import {
  advancedConstructs,
  exportHTML,
  fieldPaths,
  prepareForEditor,
} from "../lib/templates/serialize";

// Serialization + toolbar-state coverage for the MPE-51 editor. The
// pagination extension is left out — jsdom has no layout engine and the
// page scaffold is tested through exportHTML's DOMParser path.

function makeEditor(html: string) {
  return new Editor({
    extensions: editorExtensions({ paginate: false }),
    content: prepareForEditor(html, { wrap: false }),
  });
}

describe("template serialization", () => {
  let ed: Editor | null = null;
  afterEach(() => ed?.destroy());

  it("round-trips paragraphs, headings, and marks through the editor", () => {
    const html = "<h2>Title</h2><p>Body <strong>bold</strong> text</p>";
    ed = makeEditor(html);
    expect(exportHTML(ed.getHTML())).toBe(html);
  });

  it("round-trips merge fields as {{expr}} text", () => {
    ed = makeEditor("<p>Total: {{doc.total}} — {{upper name}}</p>");
    const out = exportHTML(ed.getHTML());
    expect(out).toContain("{{doc.total}}");
    expect(out).toContain("{{upper name}}");
  });

  it("strips pagination scaffolding on export", () => {
    const wrapped =
      '<div data-page="true"><div data-page-body="true"><p>One</p></div></div>' +
      '<div data-page="true"><div data-page-body="true"><p>Two</p></div></div>';
    expect(exportHTML(wrapped)).toBe("<p>One</p><p>Two</p>");
  });

  it("insertMergeField writes a chip that exports as a binding", () => {
    ed = makeEditor("<p>x</p>");
    ed.commands.insertMergeField("items[].price");
    expect(ed.getHTML()).toContain('class="merge-field-chip"');
    expect(exportHTML(ed.getHTML())).toContain("{{items[].price}}");
  });

  it("flags block helpers as advanced", () => {
    const bad = advancedConstructs(
      "<ul>{{#each items}}<li>{{this}}</li>{{/each}}</ul>{{{raw}}}{{else}}",
    );
    expect(bad.join(" ")).toContain("{{#each items}}");
    expect(bad.join(" ")).toContain("{{/each}}");
    expect(advancedConstructs("<p>{{total}} {{upper n}}</p>")).toEqual([]);
  });

  it("applies toolbar state: heading, bold, align, layout attrs", () => {
    ed = makeEditor("<p>Hello</p>");
    ed.commands.setContent("<p>Hello</p>");
    ed.commands.selectAll();
    ed.commands.setHeading({ level: 3 });
    ed.commands.toggleBold();
    ed.commands.setTextAlign("center");
    ed.commands.setBlockLayout({ lineHeight: "1.5", spaceAfter: "1em", indent: 2 });
    expect(ed.isActive("heading", { level: 3 })).toBe(true);
    expect(ed.isActive("bold")).toBe(true);
    const out = ed.getHTML().replace(/\s/g, "");
    expect(out).toContain("text-align:center");
    expect(out).toContain("line-height:1.5");
    expect(out).toContain("margin-bottom:1em");
    expect(out).toContain("margin-left:48px");
  });

  it("outdent stops at zero", () => {
    ed = makeEditor("<p>x</p>");
    ed.commands.selectAll();
    expect(ed.commands.outdentBlock()).toBe(false);
    ed.commands.indentBlock();
    ed.commands.indentBlock();
    expect(ed.getHTML()).toContain("margin-left: 48px");
    ed.commands.outdentBlock();
    ed.commands.outdentBlock();
    expect(ed.getHTML()).not.toContain("margin-left");
    expect(ed.commands.outdentBlock()).toBe(false);
  });

  it("flattens sample JSON into picker paths", () => {
    const paths = fieldPaths({
      doc_title: "x",
      due: "2026-01-01",
      items: [{ title: "t", price: 1 }],
      meta: { author: "a" },
    });
    expect(paths).toContain("doc_title");
    expect(paths).toContain("items[].title");
    expect(paths).toContain("meta.author");
    expect(paths).not.toContain("items");
  });
});

// The /templates list rolls masters up to the latest version per name.
describe("templates list page", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("lists latest versions per master name", async () => {
    const { default: Templates } = await import(
      "../app/(portal)/templates/page"
    );
    const { Providers } = await import("../app/providers");
    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string) => {
        if (url.includes("/me"))
          return new Response(
            JSON.stringify({ id: "u1", name: "A", role: "admin" }),
            { status: 200 },
          );
        if (url.includes("/clients/"))
          return new Response(
            JSON.stringify([
              {
                id: "tmpl_1",
                client_id: "cli_1",
                name: "invoice",
                version: 3,
                html: "<p>v3</p>",
                css: "",
                defaults: {},
                created_at: "2026-02-01T00:00:00Z",
              },
              {
                id: "tmpl_0",
                client_id: "cli_1",
                name: "invoice",
                version: 1,
                html: "<p>v1</p>",
                css: "",
                defaults: {},
                created_at: "2026-01-01T00:00:00Z",
              },
            ]),
            { status: 200 },
          );
        return new Response(
          JSON.stringify([{ id: "cli_1", name: "Demo", status: "active" }]),
          { status: 200 },
        );
      }),
    );
    render(
      <Providers>
        <Templates />
      </Providers>,
    );
    const row = await screen.findByText("invoice");
    expect(row).toBeTruthy();
    expect(await screen.findByText("v3")).toBeTruthy();
    expect(await screen.findByText("2")).toBeTruthy(); // versions count
    expect(screen.queryByText("v1")).toBeNull();
  });
});

// Component coverage for the editor shell, toolbar, and field picker.
// Runs unpaginated — jsdom has no layout engine for the page extension.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, waitFor } from "@testing-library/react";
import { EditorToolbar } from "../components/templates/toolbar";
import { FieldPicker } from "../components/templates/field-picker";
import { TemplateEditor } from "../components/templates/template-editor";

function qc() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

describe("EditorToolbar", () => {
  let ed: Editor | null = null;
  afterEach(() => {
    ed?.destroy();
    cleanup();
  });

  it("drives marks, blocks, alignment, and layout commands", () => {
    ed = makeEditor("<p>hello</p>");
    ed.commands.selectAll();
    const onField = vi.fn();
    render(<EditorToolbar editor={ed} onInsertField={onField} />);
    fireEvent.click(screen.getByTitle("Bold"));
    expect(ed.isActive("bold")).toBe(true);
    fireEvent.click(screen.getByTitle("Align center"));
    expect(ed.getHTML()).toContain("text-align: center");
    fireEvent.click(screen.getByTitle("Increase indent"));
    expect(ed.getHTML()).toContain("margin-left: 24px");
    fireEvent.change(screen.getByLabelText("Block type"), {
      target: { value: "h2" },
    });
    expect(ed.isActive("heading", { level: 2 })).toBe(true);
    fireEvent.change(screen.getByLabelText("Line height"), {
      target: { value: "1.5" },
    });
    expect(ed.getHTML()).toContain("line-height: 1.5");
    fireEvent.click(screen.getByTitle("Insert merge field"));
    expect(onField).toHaveBeenCalled();
  });
});

describe("FieldPicker", () => {
  let ed: Editor | null = null;
  afterEach(() => {
    ed?.destroy();
    cleanup();
  });

  it("inserts a picked path and a manual expression", () => {
    ed = makeEditor("<p></p>");
    render(
      <FieldPicker
        editor={ed}
        sampleJson='{"doc_title":"x","items":[{"price":1}]}'
        onClose={() => {}}
      />,
    );
    fireEvent.click(screen.getByText("{{doc_title}}"));
    expect(exportHTML(ed.getHTML())).toContain("{{doc_title}}");
  });

  it("inserts a manual path", () => {
    ed = makeEditor("<p></p>");
    render(
      <FieldPicker editor={ed} sampleJson="{}" onClose={() => {}} />,
    );
    fireEvent.change(screen.getByLabelText("Field path"), {
      target: { value: "invoice.number" },
    });
    fireEvent.click(screen.getByText("Insert"));
    expect(exportHTML(ed.getHTML())).toContain("{{invoice.number}}");
  });
});

describe("TemplateEditor", () => {
  const saved: unknown[] = [];
  beforeEach(() => {
    saved.length = 0;
    vi.stubGlobal("URL", {
      ...URL,
      createObjectURL: vi.fn(() => "blob:preview"),
      revokeObjectURL: vi.fn(),
    });
    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string, init?: RequestInit) => {
        if (url.includes("/preview")) {
          saved.push(JSON.parse(String(init?.body)));
          return new Response(new Blob(["%PDF"]), { status: 200 });
        }
        if (url.includes("/template-masters") && init?.method === "POST") {
          saved.push(JSON.parse(String(init.body)));
          return new Response(
            JSON.stringify({
              id: "tmpl_new",
              client_id: "cli_1",
              name: "tmpl_test",
              version: 1,
              html: "",
              css: "",
              defaults: {},
              created_at: "2026-01-01T00:00:00Z",
            }),
            { status: 201 },
          );
        }
        return new Response("{}", { status: 200 });
      }),
    );
  });
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("gates create on a valid name, previews, and saves", async () => {
    render(
      <QueryClientProvider client={qc()}>
        <TemplateEditor clientId="cli_1" master={null} isAdmin paginate={false} />
      </QueryClientProvider>,
    );
    await waitFor(() =>
      expect(document.querySelector(".ProseMirror")).toBeTruthy(),
    );
    const saveBtn = screen.getByText("Create template") as HTMLButtonElement;
    expect(saveBtn.disabled).toBe(true);
    fireEvent.change(screen.getByLabelText("Template name"), {
      target: { value: "tmpl_test" },
    });
    expect(saveBtn.disabled).toBe(false);

    // Live preview hits the draft endpoint with the exported body.
    fireEvent.click(screen.getByText("Render preview"));
    await waitFor(() => expect(saved.length).toBe(1));
    expect(saved[0]).toMatchObject({ html: expect.stringContaining("<p") });

    fireEvent.click(saveBtn);
    await waitFor(() => expect(saved.length).toBe(2));
    expect(saved[1]).toMatchObject({ name: "tmpl_test" });
  });

  it("switches visual → source with clean exported HTML and back", async () => {
    const master = {
      id: "tmpl_1",
      client_id: "cli_1",
      name: "invoice",
      version: 2,
      html: "<p>Hi {{doc_title}}</p>",
      css: "p{}",
      defaults: {},
      created_at: "2026-01-01T00:00:00Z",
    };
    render(
      <QueryClientProvider client={qc()}>
        <TemplateEditor clientId="cli_1" master={master} isAdmin paginate={false} />
      </QueryClientProvider>,
    );
    await waitFor(() =>
      expect(document.querySelector(".ProseMirror")).toBeTruthy(),
    );
    expect(document.querySelector(".merge-field-chip")).toBeTruthy();

    fireEvent.click(screen.getByRole("tab", { name: "source" }));
    const src = screen.getAllByTestId("codemirror")[0] as HTMLTextAreaElement;
    expect(src.value).toContain("{{doc_title}}");
    expect(src.value).not.toContain("merge-field-chip");

    fireEvent.click(screen.getByRole("tab", { name: "visual" }));
    await waitFor(() =>
      expect(document.querySelector(".merge-field-chip")).toBeTruthy(),
    );
  });

  it("pins source mode on block helpers", async () => {
    const master = {
      id: "tmpl_1",
      client_id: "cli_1",
      name: "invoice",
      version: 1,
      html: "{{#each items}}<p>{{this}}</p>{{/each}}",
      css: "",
      defaults: {},
      created_at: "2026-01-01T00:00:00Z",
    };
    render(
      <QueryClientProvider client={qc()}>
        <TemplateEditor clientId="cli_1" master={master} isAdmin paginate={false} />
      </QueryClientProvider>,
    );
    expect(await screen.findByRole("alert")).toHaveProperty(
      "textContent",
      expect.stringContaining("pinned to source"),
    );
  });

  it("renders read-only for viewers", async () => {
    render(
      <QueryClientProvider client={qc()}>
        <TemplateEditor clientId="cli_1" master={null} isAdmin={false} paginate={false} />
      </QueryClientProvider>,
    );
    await waitFor(() =>
      expect(document.querySelector(".ProseMirror")).toBeTruthy(),
    );
    expect(screen.queryByRole("toolbar")).toBeNull();
    expect(screen.queryByText("Create template")).toBeNull();
  });
});

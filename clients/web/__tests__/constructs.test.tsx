import { Editor } from "@tiptap/core";
import { afterEach, describe, expect, it } from "vitest";
import { editorExtensions } from "../components/templates/extensions";
import {
  exportHTML,
  liftConstructs,
  lowerConstructs,
  prepareForEditor,
  residualAdvanced,
  togglesFromHtml,
} from "../lib/templates/serialize";

// MPE-53 print constructs — source↔visual round-trip coverage.

function ed(html: string) {
  return new Editor({
    extensions: editorExtensions({ paginate: false }),
    content: prepareForEditor(html, { wrap: false }),
  });
}

describe("construct transforms", () => {
  it("lifts and lowers {{#if}} regions", () => {
    const src = "<p>a</p>{{#if blocks.promo}}<p>P</p>{{/if}}<p>b</p>";
    const lifted = liftConstructs(src);
    expect(lifted).toContain('class="cond-region" data-expr="blocks.promo"');
    expect(lowerConstructs(lifted)).toBe(src);
  });

  it("lifts and lowers each-in-tbody", () => {
    const src =
      "<table><thead><tr><th>T</th></tr></thead><tbody>{{#each items}}<tr><td>{{t}}</td></tr>{{/each}}</tbody></table>";
    const lifted = liftConstructs(src);
    expect(lifted).toContain('data-each="items"');
    expect(lowerConstructs(lifted)).toBe(src);
  });

  it("flags leftovers the canvas cannot represent", () => {
    expect(residualAdvanced("<p>{{#if x}}a{{/if}}</p>")).toEqual([]);
    expect(
      residualAdvanced("<p>{{#each items}}x{{/each}}</p>"), // each outside tbody
    ).toContain("{{#each items}}");
    expect(residualAdvanced("<p>{{#if x}}a{{else}}b{{/if}}</p>").length).toBeGreaterThan(0);
    expect(residualAdvanced("<p>{{{raw}}}</p>")).toContain("{{{raw}}");
  });

  it("collects toggle names from blocks.* conditions", () => {
    expect(
      togglesFromHtml("{{#if blocks.promo}}a{{/if}}{{#if blocks.footer}}b{{/if}}{{#if paid}}c{{/if}}"),
    ).toEqual(["footer", "promo"]);
  });
});

describe("construct nodes", () => {
  let e: Editor | null = null;
  afterEach(() => e?.destroy());

  it("round-trips a conditional region through the editor", () => {
    e = ed("<p>a</p>{{#if blocks.promo}}<p>P {{name}}</p>{{/if}}<p>b</p>");
    const out = exportHTML(e.getHTML());
    expect(out).toContain("{{#if blocks.promo}}");
    expect(out).toContain("{{name}}");
    expect(out).toContain("{{/if}}");
  });

  it("round-trips a bound data table with repeat header", () => {
    const src =
      "<table class=\"data-table\"><thead><tr><th>Item</th><th>Price</th></tr></thead><tbody>{{#each items}}<tr><td>{{title}}</td><td>{{price}}</td></tr>{{/each}}</tbody></table>";
    e = ed(src);
    const out = exportHTML(e.getHTML());
    expect(out).toContain("{{#each items}}");
    expect(out).toContain("{{title}}");
    expect(out).toContain("<th>Item</th>");
  });

  it("inserts a data table via insertContent attrs", () => {
    e = ed("<p></p>");
    e.commands.insertContent({
      type: "dataTable",
      attrs: {
        path: "items",
        repeatHeader: true,
        cols: [
          { label: "Item", expr: "title" },
          { label: "Qty", expr: "qty" },
        ],
      },
    });
    const out = exportHTML(e.getHTML());
    expect(out).toContain("{{#each items}}");
    expect(out).toContain("{{qty}}");
  });

  it("serializes breaks as break-before styles", () => {
    e = ed("<p></p>");
    e.commands.insertContent([
      { type: "pageBreak", attrs: { kind: "page" } },
      { type: "pageBreak", attrs: { kind: "recto" } },
    ]);
    const out = exportHTML(e.getHTML()).replace(/\s/g, "");
    expect(out).toContain("break-before:page");
    expect(out).toContain("break-before:recto");
  });

  it("wraps a selection in a conditional region", () => {
    e = ed("<p>hello</p>");
    e.commands.selectAll();
    e.commands.insertContent({
      type: "conditionalRegion",
      attrs: { expr: "blocks.promo" },
      content: [{ type: "paragraph" }],
    });
    expect(exportHTML(e.getHTML())).toContain("{{#if blocks.promo}}");
  });
});

// InsertMenu coverage — drives every construct through the popover.
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { InsertMenu } from "../components/templates/insert-menu";

describe("InsertMenu", () => {
  let e: Editor | null = null;
  afterEach(() => {
    e?.destroy();
    cleanup();
  });
  const open = (keys = ["logo"]) =>
    render(
      <InsertMenu
        editor={e!}
        brandingKeys={keys}
        onClose={() => {}}
        onPickField={() => {}}
      />,
    );

  it("inserts both break kinds from the menu", () => {
    e = ed("<p></p>");
    open();
    fireEvent.click(screen.getByRole("button", { name: /Page break/ }));
    let out = exportHTML(e.getHTML());
    expect(out).toContain("break-before");
    fireEvent.click(screen.getByRole("button", { name: /New sheet/ }));
    out = exportHTML(e.getHTML()).replace(/\s/g, "");
    expect(out).toContain("break-before:recto");
  });

  it("inserts a conditional region via the form", () => {
    e = ed("<p></p>");
    open();
    fireEvent.click(
      screen.getByRole("button", { name: /Conditional region/ }),
    );
    fireEvent.click(screen.getByText("Wrap selection / insert"));
    expect(exportHTML(e.getHTML())).toContain("{{#if blocks.}}");
  });

  it("inserts a bound table with repeat header", () => {
    e = ed("<p></p>");
    open();
    fireEvent.click(screen.getByRole("button", { name: /Data table/ }));
    fireEvent.click(screen.getByText("Insert table"));
    const out = exportHTML(e.getHTML());
    expect(out).toContain("{{#each items}}");
    expect(out).toContain("{{title}}");
    expect(out).toContain("<th>");
  });

  it("inserts a branding merge field", () => {
    e = ed("<p></p>");
    open();
    fireEvent.click(screen.getByRole("button", { name: /Branding slot/ }));
    fireEvent.click(screen.getByText(/Insert \{\{branding/));
    expect(exportHTML(e.getHTML())).toContain("{{branding.logo}}");
  });
});

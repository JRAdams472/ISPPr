import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { PageSetupPanel } from "../components/templates/page-setup";
import {
  ALL_SIZES,
  extPaperSize,
  pagePayload,
  printPayload,
  readPage,
  type PageSetup,
} from "../lib/templates/page-setup";

// MPE-52 page-setup: the defaults.page model + the panel that authors it.

describe("page-setup lib", () => {
  it("maps backend keys to pagination-extension sizes", () => {
    expect(extPaperSize("ANSIA")).toBe("ANSI A");
    expect(extPaperSize("ANSIE")).toBe("ANSI E");
    expect(extPaperSize("LETTER")).toBe("Letter");
    expect(extPaperSize("LEDGER")).toBe("Ledger");
    expect(extPaperSize("A4")).toBe("A4");
    expect(extPaperSize()).toBe("A4");
  });

  it("offers all 16 required sizes plus aliases", () => {
    const vals = ALL_SIZES.map((s) => s.value);
    for (const s of [
      "A0", "A1", "A2", "A3", "A4", "A5", "A6", "A7", "A8",
      "ANSIA", "ANSIB", "ANSIC", "ANSID", "ANSIE",
      "LETTER", "LEGAL", "TABLOID", "LEDGER",
    ]) {
      expect(vals).toContain(s);
    }
  });

  it("reads defaults.page with sensible defaults", () => {
    expect(readPage(undefined)).toMatchObject({
      size: "A4",
      orientation: "portrait",
      margins: {},
      header_html: "",
    });
    const p = readPage({
      page: { size: "A3", orientation: "landscape", margins: { top: 0.5 } },
      toggles: { promo: true },
    });
    expect(p.size).toBe("A3");
    expect(p.margins?.top).toBe(0.5);
  });

  it("payload drops blanks and zero margins", () => {
    const p: PageSetup = {
      size: "A3",
      orientation: "landscape",
      margins: { top: 0.8, right: 0, bottom: undefined, left: 1 },
      header_html: " <b>H</b> ",
      footer_html: "  ",
    };
    expect(pagePayload(p)).toEqual({
      size: "A3",
      orientation: "landscape",
      margins: { top: 0.8, left: 1 },
      header_html: " <b>H</b> ",
    });
    expect(
      pagePayload({ size: "A4", orientation: "portrait", margins: {} }),
    ).toEqual({ size: "A4", orientation: "portrait" });
  });

  it("reads and emits the print block (MPE-57)", () => {
    expect(readPage(undefined).print).toEqual({ enabled: false });
    expect(
      readPage({ print: { enabled: true } }).print,
    ).toEqual({ enabled: true });
    // Unknown truthy values don't enable it accidentally.
    expect(
      readPage({ print: { enabled: "yes" } }).print?.enabled,
    ).toBe(false);
    expect(printPayload({ enabled: true })).toEqual({ enabled: true });
    expect(printPayload({ enabled: false })).toEqual({});
    expect(printPayload(undefined)).toEqual({});
  });
});

describe("PageSetupPanel", () => {
  afterEach(cleanup);

  function setup(v?: Partial<PageSetup>) {
    const onChange = vi.fn();
    const value: PageSetup = {
      size: "A4",
      orientation: "portrait",
      margins: {},
      header_html: "",
      footer_html: "",
      ...v,
    };
    render(
      <PageSetupPanel value={value} readOnly={false} onChange={onChange} />,
    );
    return { onChange, value };
  }

  it("exposes size, orientation, and four margin inputs", () => {
    setup();
    const size = screen.getByLabelText("Paper size") as HTMLSelectElement;
    expect(size.value).toBe("A4");
    expect(size.querySelectorAll("option").length).toBe(18);
    for (const side of ["top", "right", "bottom", "left"]) {
      expect(screen.getByLabelText(`${side} margin`)).toBeTruthy();
    }
    expect(screen.getByRole("radio", { name: "landscape" })).toBeTruthy();
  });

  it("emits changes for size, orientation, and margins", () => {
    const { onChange } = setup();
    fireEvent.change(screen.getByLabelText("Paper size"), {
      target: { value: "ANSID" },
    });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ size: "ANSID" }),
    );
    fireEvent.click(screen.getByRole("radio", { name: "landscape" }));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ orientation: "landscape" }),
    );
    fireEvent.change(screen.getByLabelText("top margin"), {
      target: { value: "0.75" },
    });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ margins: { top: 0.75 } }),
    );
    fireEvent.change(screen.getByLabelText("left margin"), {
      target: { value: "" },
    });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ margins: { top: 0.75, left: undefined } }),
    );
  });

  it("token chips insert Chromium page-number spans", () => {
    const { onChange } = setup();
    const tokens = screen.getAllByTitle(/Insert <span class="(pageNumber|totalPages)"/);
    expect(tokens.length).toBe(4); // header + footer, two each
    fireEvent.click(tokens[2]); // footer [page #]
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        footer_html: '<span class="pageNumber"></span>',
      }),
    );
  });

  it("print-ready toggle emits print.enabled (MPE-57)", () => {
    const { onChange } = setup();
    const box = screen.getByRole("checkbox", { name: /print-ready/i });
    expect((box as HTMLInputElement).checked).toBe(false);
    fireEvent.click(box);
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ print: { enabled: true } }),
    );
    cleanup();
    // A stored enabled:true restores the toggle on reload.
    setup({ print: { enabled: true } });
    expect(
      (screen.getByRole("checkbox", { name: /print-ready/i }) as HTMLInputElement)
        .checked,
    ).toBe(true);
  });

  it("surfaces a validation error inline", () => {
    const onChange = vi.fn();
    render(
      <PageSetupPanel
        value={{ size: "A4", orientation: "portrait" }}
        readOnly={false}
        error="footer_html: template parse error"
        onChange={onChange}
      />,
    );
    expect(screen.getByRole("alert").textContent).toContain("footer_html");
  });
});

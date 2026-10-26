// Page-setup model (MPE-52) — the `defaults.page` block authored by the
// panel and merged into master defaults on save. Keys/units match the
// Go PageSetup wire shape exactly (inches, header_html/footer_html).

export interface PageMargins {
  top?: number;
  right?: number;
  bottom?: number;
  left?: number;
}

// PrintSetup is the post-processing block (MPE-57): a print-ready
// template's generated PDFs are piped through Ghostscript for CMYK
// conversion, font embedding, and a PDF/X-style OutputIntent. The
// draft preview stays RGB — the flag only applies to generated output.
export interface PrintSetup {
  enabled?: boolean;
}

export interface PageSetup {
  size?: string;
  orientation?: "portrait" | "landscape";
  margins?: PageMargins;
  header_html?: string;
  footer_html?: string;
  print?: PrintSetup;
}

export const MM_PER_INCH = 25.4;

// value = backend size key; ext = tiptap-extension-pagination paperSize.
export const SIZE_GROUPS: {
  label: string;
  sizes: { value: string; label: string; ext: string }[];
}[] = [
  {
    label: "ISO A",
    sizes: ["A0", "A1", "A2", "A3", "A4", "A5", "A6", "A7", "A8"].map(
      (v) => ({ value: v, label: v, ext: v }),
    ),
  },
  {
    label: "ANSI",
    sizes: [
      { value: "ANSIA", label: "ANSI A", ext: "ANSI A" },
      { value: "ANSIB", label: "ANSI B", ext: "ANSI B" },
      { value: "ANSIC", label: "ANSI C", ext: "ANSI C" },
      { value: "ANSID", label: "ANSI D", ext: "ANSI D" },
      { value: "ANSIE", label: "ANSI E", ext: "ANSI E" },
    ],
  },
  {
    label: "US",
    sizes: [
      { value: "LETTER", label: "Letter", ext: "Letter" },
      { value: "LEGAL", label: "Legal", ext: "Legal" },
      { value: "TABLOID", label: "Tabloid", ext: "Tabloid" },
      { value: "LEDGER", label: "Ledger", ext: "Ledger" },
    ],
  },
];

export const ALL_SIZES = SIZE_GROUPS.flatMap((g) => g.sizes);

export function extPaperSize(backendKey?: string): string {
  return (
    ALL_SIZES.find((s) => s.value === backendKey)?.ext ??
    (backendKey || "A4")
  );
}

/** Reads defaults.page out of a master's defaults blob. */
export function readPage(defaults: Record<string, unknown> | undefined): PageSetup {
  const p = (defaults?.page ?? {}) as PageSetup;
  return {
    size: p.size ?? "A4",
    orientation: p.orientation === "landscape" ? "landscape" : "portrait",
    margins: p.margins ?? {},
    header_html: p.header_html ?? "",
    footer_html: p.footer_html ?? "",
    print: readPrint(defaults),
  };
}

/** Reads defaults.print out of a master's defaults blob. */
export function readPrint(
  defaults: Record<string, unknown> | undefined,
): PrintSetup {
  const p = (defaults?.print ?? {}) as PrintSetup;
  return { enabled: p.enabled === true };
}

/**
 * Emits the defaults.print block for the wire: {enabled:true} when on,
 * {} when off — callers should delete the key rather than persist a
 * stale enabled:true under the toggle.
 */
export function printPayload(p: PrintSetup | undefined): Record<string, unknown> {
  return p?.enabled ? { enabled: true } : {};
}

/**
 * Strips unset fields for the wire: empty margins sides drop out (a zero
 * means "backend default", not "0"), blank header/footer drop out. An
 * all-empty block returns {} so callers can omit it cleanly.
 */
export function pagePayload(p: PageSetup): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  if (p.size) out.size = p.size;
  if (p.orientation) out.orientation = p.orientation;
  const m = Object.fromEntries(
    Object.entries(p.margins ?? {}).filter(
      ([, v]) => typeof v === "number" && v > 0,
    ),
  );
  if (Object.keys(m).length) out.margins = m;
  if (p.header_html?.trim()) out.header_html = p.header_html;
  if (p.footer_html?.trim()) out.footer_html = p.footer_html;
  return out;
}

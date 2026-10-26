// Template HTML ↔ editor serialization (MPE-51).
//
// The editor works on a "document surface" derived from the stored
// template HTML: inline `{{expr}}` bindings become merge-field chips,
// and the whole document is wrapped in the pagination extension's
// page scaffolding. Exporting reverses both steps so the HTML that
// reaches the API is clean Handlebars-flavoured markup.
//
// Constructs that cannot be represented visually — block helpers
// ({{#each}}, {{#if}}, …), partials, raw triple-stash, comments — are
// flagged by analyze() so the shell can pin source mode instead of
// silently dropping or mangling them.

/** Inline `{{expr}}` — everything a merge-field chip can hold. */
const INLINE_RE = /\{\{(?!#|\/|>|!|\{)([^{}]*?)\}\}/g;

/** Block-level Handlebars syntax the visual editor cannot represent. */
const ADVANCED_RE =
  /\{\{(?:#|\/|>|!|else\b|\{)[^{}]*\}\}?|\{\{\{[^{}]*\}\}\}/g;

/** Tokens surfaced to the user when a template pins source mode. */
export function advancedConstructs(html: string): string[] {
  const found = new Set<string>();
  for (const m of html.matchAll(ADVANCED_RE)) {
    found.add(m[0]);
  }
  return [...found];
}

/** Wraps inline {{expr}} bindings in chip spans the MergeField node parses. */
export function prepareForEditor(html: string, { wrap = true } = {}): string {
  const chipped = html.replace(INLINE_RE, (m, expr: string) => {
    const e = expr.trim();
    return e ? `<span class="merge-field-chip">{{${e}}}</span>` : m;
  });
  // Body content lives inside the pagination extension's page scaffold.
  return wrap
    ? `<div data-page="true"><div data-page-body="true">${chipped}</div></div>`
    : chipped;
}

const CHIP_RE =
  /<span class="merge-field-chip"[^>]*>([\s\S]*?)<\/span>/g;

function unescapeEntities(s: string): string {
  return s
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&amp;/g, "&");
}

/**
 * Serializes editor HTML back to template source: page scaffolding is
 * unwrapped (body regions concatenated, empty header/footer regions
 * dropped) and merge-field chips become plain `{{expr}}` text.
 */
export function exportHTML(html: string): string {
  let body = html;
  if (typeof DOMParser !== "undefined" && html.includes("data-page-body")) {
    const doc = new DOMParser().parseFromString(html, "text/html");
    const bodies = [...doc.querySelectorAll("[data-page-body]")];
    if (bodies.length) body = bodies.map((b) => b.innerHTML).join("");
  } else {
    // Fallback without a DOM — last resort regex unwrap.
    body = html
      .replace(/<div[^>]*data-page-body[^>]*>/g, "")
      .replace(/<div[^>]*data-page[^>]*>/g, "");
  }
  return body
    .replace(CHIP_RE, (_m, inner: string) => unescapeEntities(inner))
    .trim();
}

/** Sample-data JSON → flattened merge-field paths for the picker. */
export function fieldPaths(data: unknown, prefix = ""): string[] {
  if (Array.isArray(data)) {
    return data.length ? fieldPaths(data[0], `${prefix}[]`) : [];
  }
  if (data && typeof data === "object") {
    return Object.entries(data as Record<string, unknown>).flatMap(([k, v]) =>
      fieldPaths(v, prefix ? `${prefix}.${k}` : k),
    );
  }
  return prefix ? [prefix] : [];
}

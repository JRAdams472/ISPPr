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
  const chipped = liftConstructs(html).replace(INLINE_RE, (m, expr: string) => {
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
  return lowerConstructs(body)
    .replace(CHIP_RE, (_m, inner: string) => unescapeEntities(inner))
    .trim();
}

// ── MPE-53 print constructs ─────────────────────────────────────────
// Visual representations for the Handlebars constructs the canvas can
// faithfully round-trip: flat {{#if expr}} regions (any expression) and
// {{#each path}} bodies inside <tbody>. Everything else ({{else}},
// nested blocks, partials, triple-stash, each outside tbody) still pins
// source mode — residualAdvanced() runs after the supported transforms.

const IF_OPEN = /\{\{#if\s+([^{}]+?)\s*\}\}/g;
const EACH_TBODY =
  /<tbody>\s*\{\{#each\s+([^{}]+?)\s*\}\}([\s\S]*?)\{\{\/each\}\}\s*<\/tbody>/g;

/**
 * Rewrites supported Handlebars blocks into the visual node markup the
 * editor parses (cond-region div, data-each tbody). Runs inside
 * prepareForEditor before the page wrap.
 */
export function liftConstructs(html: string): string {
  return html
    .replace(EACH_TBODY, '<tbody data-each="$1">$2</tbody>')
    .replace(IF_OPEN, '<div class="cond-region" data-expr="$1">')
    .replace(/\{\{\/if\}\}/g, "</div>");
}

/**
 * Reverses liftConstructs on editor HTML — runs inside exportHTML after
 * the page unwrap, before chip unwrapping.
 */
export function lowerConstructs(html: string): string {
  return html
    .replace(
      /<div class="cond-region" data-expr="([^"]*)">([\s\S]*?)<\/div>/g,
      "{{#if $1}}$2{{/if}}",
    )
    .replace(
      /<tbody data-each="([^"]*)">([\s\S]*?)<\/tbody>/g,
      '<tbody>{{#each $1}}$2{{/each}}</tbody>',
    );
}

/**
 * Constructs the canvas still cannot represent AFTER the supported
 * transforms — anything left pins source mode.
 */
export function residualAdvanced(html: string): string[] {
  return advancedConstructs(liftConstructs(html));
}

/** `{{#if blocks.X}}` toggle names for defaults.toggles. */
export function togglesFromHtml(html: string): string[] {
  const names = new Set<string>();
  for (const m of html.matchAll(/\{\{#if\s+blocks\.(\w+)\s*\}\}/g)) {
    names.add(m[1]);
  }
  return [...names].sort();
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

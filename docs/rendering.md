# Rendering — Handlebars surface

`internal/render` compiles resolved templates (`templates.Resolved.HTML`)
with [`aymerick/raymond`](https://github.com/aymerick/raymond) and binds the
request payload via `Resolved.ContextWith(payload)` — resolved
`blocks`/`branding` merge under caller data, caller keys win.

```go
tpl, _ := render.Compile(resolved.HTML)      // once
html, _ := tpl.Exec(ctx)                     // per request (concurrent-safe)

html, err := render.Render(resolved, payload) // one-shot convenience
```

## Helper surface (frozen)

The supported set is fixed per release — there is no user-registered
helper mechanism.

**Built-ins (raymond):** `{{#if}}` / `{{#unless}}` / `{{#each}}` /
`{{#with}}` / `{{lookup}}` / `{{log}}` / `{{equal}}`, plus `{{@index}}`,
`{{@key}}`, `{{@first}}`, `{{@last}}` inside `each`.

**Comparisons** — numeric when both sides parse as numbers, string compare
for strings, `false < true` for bools: `eq` `ne` `lt` `le` `gt` `ge`.
Usable as sub-expressions: `{{#if (gt total 1000)}}…{{/if}}`.

**Logic:** `and` `or` `not` — Handlebars truthiness (false/nil/0/""/empty
collections are falsy).

**Strings:** `upper` `lower` `title`.

**Collections:** `join list "sep"` — stringifies each element.

**Formatting:**
- `{{date value "layout"}}` — value may be RFC3339, `YYYY-MM-DD`,
  `YYYY-MM-DD HH:MM:SS`, unix seconds, or `time.Time`; layout is a Go
  `time.Format` layout. Output is always **UTC** — deterministic, no host
  timezone dependence. Unparseable → empty string.
- `{{currency amount "CODE"}}` — grouped thousands, two decimals:
  `{{currency 1234.5 "USD"}}` → `1,234.50 USD`. Non-numeric → empty.

## Unsupported-helper behavior — fail closed

`Compile` walks the parsed AST and **rejects any call expression whose
callee is not a registered helper** (`unsupported helper "x" — line N`),
including nested sub-expression calls. A typo'd helper is a compile error,
never silent empty output.

Pure path lookups (`{{foo}}`, `{{a.b.c}}`) are *not* calls — they render
empty when the path is missing (standard Handlebars behavior).

**Field names must not shadow helper names.** A payload key named `title`,
`date`, `join`, etc. resolves to the *helper*, not the value — `{{title}}`
renders helper internals instead of `data.title`. Prefix collision-prone
fields (`doc_title`, `due_date`) when authoring templates.

## Injection safety

- `{{path}}` and helper results are HTML-escaped by raymond
  (`<` → `&lt;`, `'` → `&apos;`, etc.) — safe for untrusted payload values.
- `{{{path}}}` (triple-stache) and `raymond.SafeString` outputs bypass
  escaping — **authoring opt-out only**: never bind raw customer payload
  through triple-stache. The render pool treats every `{{…}}` value as
  untrusted by construction.
- Partials (`{{> name}}`) are not part of the surface — none are
  registered; they fail at render time.

## Page setup (defaults.page)

Masters (and variant overlays) may carry a `page` block steering the
print layout — forwarded to Gotenberg's Chromium fields:

```json
{"page":{
  "size":"A4",                      // A0-A8, ANSIA-ANSIE, LETTER, LEGAL, TABLOID, LEDGER
  "orientation":"landscape",        // default portrait; Chromium rotates
  "margins":{"top":0.8,"bottom":0.8,"left":1,"right":1},  // inches
  "header_html":"<p>{{branding.logo}}</p>",
  "footer_html":"<div style='font-size:9px;text-align:center;width:100%'>Page <span class='pageNumber'></span> of <span class='totalPages'></span></div>"
}}
```

- `header.html`/`footer.html` become separate Gotenberg file parts — the
  only mechanism that produces incrementing page numbers in Chromium
  (`pageNumber` / `totalPages` / `date` / `title` classes). Never stamp
  page numbers into the body.
- Strips bind the same context as the body (payload fields, branding) and
  must compile — the write path fails closed on bad source.
- Chromium print headers/footers ignore external CSS: inline styles only,
  and they need margin room (set `margins.top`/`bottom` to make space).
- `landscape` is passed as Chromium's flag with portrait paper dims — do
  NOT pre-swap width/height or the rotation cancels out.
- Variants override `page` field-wise through the overlay merge (a
  customer can reskin just the footer, say). A zero margin in an override
  means "unset" — explicit zeroing isn't expressible in v1.
- In-document layout: repeat table headers = `<thead>` (Chromium repeats
  natively); new page = `break-before: page`; new sheet (duplex) =
  `break-before: recto`.

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

## Injection safety

- `{{path}}` and helper results are HTML-escaped by raymond
  (`<` → `&lt;`, `'` → `&apos;`, etc.) — safe for untrusted payload values.
- `{{{path}}}` (triple-stache) and `raymond.SafeString` outputs bypass
  escaping — **authoring opt-out only**: never bind raw customer payload
  through triple-stache. The render pool treats every `{{…}}` value as
  untrusted by construction.
- Partials (`{{> name}}`) are not part of the surface — none are
  registered; they fail at render time.

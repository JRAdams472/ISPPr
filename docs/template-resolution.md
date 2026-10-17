# Template Resolution

How a generation request's `template` name becomes the effective HTML/CSS +
context handed to the renderer (MPE-33). Implemented in
`internal/templates` (`Resolve` / `ResolveFor`); persistence via sqlc in
`internal/store`.

## Layers

| Layer | Scope | Table / source | What it provides |
|---|---|---|---|
| Base | global | embedded `templates/base/` | fallback HTML skeleton + base CSS; used only when the client has **no** master for the name |
| Master | client (`client_id`) | `template_masters` | Handlebars `html`, `css`, `defaults` (toggles + branding), version history |
| Variant | customer (`customer_id`) | `template_variants` | `overlay` JSONB: `css_patch`, `toggles`, `branding` |

## Resolution order

```
customer variant ──overlays──▶ client master ──falls back to──▶ base
```

1. `GetLatestMaster(client_id, name)` — highest `version` wins. No rows →
   return the embedded base template; variants are not consulted.
2. `ResolveVariant(client_id, name, customer_id)` — the customer's variant
   attached to **any** version of that master name (newest first). A variant
   therefore survives master re-publish: it re-bases onto the latest version.
3. `Resolve(base, master, overlay)` merges per the semantics below.

`ResolveFor` is the single entry point generation code calls — never query
masters/variants directly in a render path.

## Merge semantics

**CSS — concatenate, never replace.** Order: `base.css` → `master.css` →
`overlay.css_patch`, newline-joined. Later rules win the cascade, so a
variant patch can override master styling without forking it. Empty layers
are skipped.

**HTML — replace.** The master's `html` replaces the base skeleton wholesale
(masters own the layout). Variants do not patch HTML — they hide regions or
restyle them.

**Toggles — key merge over master defaults.** Masters declare named regions
as `{{#if blocks.<name>}}…{{/if}}` and seed their on/off state in
`defaults.toggles`. The resolved map `blocks = master.toggles ∪
variant.toggles` (variant wins per key) lands in `Context.blocks` and is
merged under the caller's data at render time. A key absent from both maps
renders falsy → the region is **opt-in**: masters must declare a toggle to
show its block.

**Branding — key merge.** `branding = master.branding ∪ variant.branding` →
`Context.branding`; templates read `{{branding.logo}}` etc. for asset paths
(S3/MinIO keys resolved later by the storage layer, MPE-36).

**Unknown keys are ignored**, so overlays written by newer tooling don't
break older servers.

## Version history

`template_masters` rows are append-only per `(client_id, name, version)`
(unique constraint). Publishing = `NextMasterVersion` + insert, never
UPDATE — old versions stay addressable for audit/re-render. Resolution
always targets `max(version)`; there is no "pinned" variant relationship.

## Edge cases

- **No master** → base template (empty `Context`); request may still fail
  downstream if caller data is missing required `{{fields}}`.
- **Variant exists but master was renamed/deleted** → variant unreachable
  (join is by name); base or another master resolves instead.
- **Variant authored against v1, master now at v3** → `ResolveVariant`
  still returns it (any version of the name); the overlay applies to v3.
  If v3 renamed toggle/branding keys, stale keys are simply unused.
- **Cross-tenant** → impossible by construction: every query is scoped by
  `client_id` / `customer_id`; `ResolveVariant` joins through the master's
  `client_id`, so a customer under client A can never pull client B's
  master or variant.
- **Malformed overlay/defaults JSON** → `Resolve`/`ResolveFor` error
  (caller maps to 500, logged with the row id — never silently ignored).

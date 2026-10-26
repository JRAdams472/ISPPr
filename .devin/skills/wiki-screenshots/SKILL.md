---
name: wiki-screenshots
description: Capture real web-UI screenshots for docs/wiki/proof — run an isolated ISPPr stack, seed the demo tenant, and shoot pages with Playwright. Use when a feature adds/changes user-facing screens.
---

# Wiki / proof screenshots

Run an **isolated** compose project so the dev DB is never touched:

```bash
# Isolated project name — NEVER the default dev stack.
docker compose -p ispprshots up -d --build   # api seeds cli_demo via RUN_SEED=1
docker compose -p ispprshots down -v          # tear down + drop volumes when done
```

Then drive the live portal with the `playwright` MCP (or `npx playwright`
headless). `networkidle` never settles on Next.js pages — wait on rendered
text or a selector instead.

## Ports & seed data (compose defaults)

- Portal `http://localhost:3100` · API `http://localhost:8090` · MinIO `:9100`
- Seeded: `cli_demo` / `cust_demo` / key `isppr_dev_demo` · admin
  `admin@isppr.local` / `isppr-admin` · template `tmpl_invoice` (toggles
  `header`, `promo`)
- Generate a real request for monitoring shots: `POST :8090/api/v1/packages/generate`
  with `X-API-Key: isppr_dev_demo`, `customer_id: cust_demo`,
  `template_id: tmpl_invoice`, any `payload`.

## Capture conventions

- Viewport 1440×900; sign in via the real `/login` form (the demo creds are
  prefilled).
- The Playwright MCP screenshots can only save under the repo root — save to
  `C:/wip/ISPPr/*.png` then `mv` into `ISPPr.wiki/images/`.
- **Inspect every PNG** for spinners, empty states, or error toasts before
  attaching to a wiki page, ticket, or PR.
- Pages that are stubs (Templates, Usage) don't get a wiki image.
- Clean up PNGs left in the repo root after the move — never commit them.

## Variant-editor flow (the fiddly one)

`Clients → Demo Corp → Demo Customer → New override → pick tmpl_invoice`.
Two CodeMirror editors share the page — target them by language:
`.cm-content[data-language='css']` and `.cm-content[data-language='json']`.
Fill via `locator(...).fill(...)`, toggle `promo` by clicking its checkbox,
then *Render preview* and wait ~4 s for the iframe PDF.

The demo master's `{{title}}` is shadowed by the `title` raymond helper —
payload key `doc_title` is what the h1 actually binds. If the seed still
renders `&{0X…[] Map[]}`, patch the *shots* DB only (throwaway):
`UPDATE template_masters SET html = REPLACE(html,'<h1>{{title}}</h1>','<h1>{{doc_title}}</h1>')`.

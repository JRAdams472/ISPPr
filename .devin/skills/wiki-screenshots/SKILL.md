---
name: wiki-screenshots
description: Capture real web-UI screenshots for docs/wiki/proof — run an isolated ISPPr stack, seed the demo tenant, and shoot pages with Playwright. Use when a feature adds/changes user-facing screens.
---

# Wiki / proof screenshots

ISPPr doesn't have a dedicated shots stack yet — the e2e harness lands in
MPE-46. Until then, run an isolated compose project so the dev DB is never
touched:

```bash
# Isolated project name — NEVER the default dev stack.
docker compose -p ispprshots up -d --build   # api seeds cli_demo via RUN_SEED=1
# capture with playwright MCP or tools/wiki-shots/capture.mjs when it exists
docker compose -p ispprshots down -v
```

Key rules:

- Always use the separate `-p ispprshots` project (never the dev DB).
- `RUN_SEED=1` is on in compose already, so `cli_demo`/`cust_demo`/`key_demo`
  and the `admin@isppr.local` portal login exist automatically.
- `networkidle` never settles on Next dev/prod pages — wait on rendered
  text instead (`page.getByText(...)` / `wait_for_selector`).
- Inspect every PNG for loading spinners or error toasts before attaching
  to a ticket, wiki page, or PR.
- Admin portal runs on host `:3100` (container `:3000`); API on `:8090`.

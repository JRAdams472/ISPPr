# docs/

Design and architecture notes for ISPPr. The authoritative system spec is
[`projectplan.md`](../projectplan.md) at the repo root; per-phase design
notes land here as the megaplan (MPE-22) is executed.

## Index

- [architecture.md](architecture.md) — request path, components, invariants
- [tenant-api.md](tenant-api.md) — generate/status/download contract
- [management-api.md](management-api.md) — admin REST surface + roles
- [authz-matrix.md](authz-matrix.md) — route × scope × role matrix
- [template-resolution.md](template-resolution.md) — base→master→variant merge
- [rendering.md](rendering.md) — Handlebars surface + `defaults.page`
- [runbook.md](runbook.md) — ops: triage, credentials, limits, shutdown

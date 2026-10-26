# Architecture

ISPPr is a stateless document-generation engine: HTTP in, PDFs out, all
state in Postgres + S3.

## Request path

```text
POST /api/v1/packages/generate  (X-API-Key)
  │
  ▼
Echo server ── tenancy middleware (API key → client_id, scopes, 429)
  │
  ▼
orchestrator ── INSERT generation_requests (idempotency_key UNIQUE)
  │              │── sync path (within SYNC_BUDGET_MS) → inline render
  │              └── async path → worker pool (WORKER_COUNT)
  ▼
templates.Resolve ── base ← master ← variant overlay
  (html, css, defaults incl. page setup, toggles, branding)
  ▼
render.Bind ── raymond (Handlebars-compatible) exec with payload
  ▼
render.Pool ── multipart → Gotenberg /forms/chromium/convert/html
  (index.html + style.css + header/footer parts + page options)
  ▼
package ── ZIP per request (or single PDF when output_type=PDF)
  ▼
storage ── S3/MinIO put → presigned GET (15 min TTL) or local disk
  ▼
results.download_url in status response; webhook POST (HMAC-signed)
```

## Components

| Piece | Where | Notes |
|---|---|---|
| API server | `internal/server` | Echo, RFC 7807 errors, 413/429 limits, graceful drain |
| Tenant auth | `internal/tenancy` | `X-API-Key` → sha256 lookup → client_id on context |
| Orchestrator | `internal/jobs` | sync budget, worker pool, status polling, webhooks |
| Resolution | `internal/templates` | field-wise merge incl. `defaults.page` |
| Binding | `internal/render` | raymond helper surface (frozen set) |
| Gotenberg pool | `internal/render/pool.go` | fan-out, multipart page options |
| Storage | `internal/storage` | S3 presign + local dev backend |
| Persistence | `internal/store` | sqlc; migrations `migrations/NNNN_*.up.sql` |
| Portal | `clients/web` | Next.js admin UI (session auth, `/api/v1/admin/*`) |

## Data model (core)

- `clients` → `customers` (tenant boundary = `client_id`, always from the key)
- `template_masters` — append-only `(client_id, name, version)`; `defaults` JSONB
  carries `page`, `toggles`, `branding`
- `template_variants` — customer overlay `{css_patch, toggles, branding, page}`
- `generation_requests` — audit + idempotency (`client_id, idempotency_key`)
- `admin_users`/`admin_sessions`, `api_keys` (sha256 only)

## Invariants

- Stateless engine — no in-process request state; workers are restartable.
- Tenant isolation enforced at the store layer, never from payload fields.
- No binaries on the response stream — presigned URLs only.
- Append-only masters — edits publish a new version; old versions immutable.

See `docs/template-resolution.md` for merge semantics, `docs/rendering.md`
for the binding surface, `docs/tenant-api.md` for the wire contract.

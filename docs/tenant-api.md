# Tenant API — package generation

All tenant endpoints sit under `/api/v1/packages` behind `X-API-Key`
middleware (see `docs/authz-matrix.md`). Errors are RFC 7807
`application/problem+json`.

```http
POST /api/v1/packages/generate
GET  /api/v1/packages/{request_id}
GET  /api/v1/packages/{request_id}/download
```

## POST /generate

```jsonc
{
  "customer_id": "cust_demo",            // required, tenant-owned
  "package_name": "Q1_Invoices",         // required, <= 120 chars
  "client_id": "cli_demo",               // optional; if present MUST equal the
                                         // authenticated tenant (403 otherwise)
  "webhook_url": "https://…",            // optional, stored (delivery in MPE-37)
  "documents": [                         // required, non-empty, <= 50
    { "template_id": "invoice",          // template NAME (resolution is name-keyed)
      "payload": { "title": "…" } }
  ]
}
```

The tenant must own `customer_id` **and** every `template_id` — any mismatch
is a global 403.

### Responses

- `201` — finished inside the sync budget (`SYNC_BUDGET_MS`, default 8000):

  ```jsonc
  {
    "request_id": "req_…", "status": "COMPLETED", "created_at": "…",
    "metrics": { "total_documents": 1, "parsing_duration_ms": 1,
                 "rendering_duration_ms": 150, "compression_duration_ms": 0,
                 "total_duration_ms": 156 },
    "results": { "output_type": "ZIP", "filename": "Q1_Invoices_req_….zip",
                 "download_url": "/api/v1/packages/req_…/download" }
  }
  ```

- `202` — still queued/in-flight after the budget; body carries
  `request_id` + current `status` only. Poll `GET /{request_id}`.
- `400`/`403`/`401` — validation, ownership, auth failures (problem+json).

### Idempotency

Send `Idempotency-Key: <key>`. The pair `(client_id, idempotency_key)` is
unique; a replay returns the original request's current state **without
re-executing work** (same `request_id`).

### GET /{request_id}

Tenant-scoped — cross-tenant ids 404. `200` with the same response shape as
`generate` (status + metrics + results once completed). A `failed` request
includes `error`.

### Lifecycle

`QUEUED → RENDERING → COMPRESSING → COMPLETED | FAILED`

`generation_requests` is the durable audit/idempotency row;
`generation_jobs` is the work queue (`FOR UPDATE SKIP LOCKED` leases, ≤3
attempts, stale-claim requeue). The worker pipeline is
resolve → Handlebars → Gotenberg PDF → ZIP → storage.

### GET /{request_id}/download

Tenant-scoped `application/zip` of the package. Filename is
`{package_name}_{request_id}.zip`; documents are `NN_{template}.pdf` inside.
In dev the storage backend is local fs (`STORAGE_LOCAL_DIR`); S3/MinIO with
presigned URLs replaces it in MPE-36 behind the same route.

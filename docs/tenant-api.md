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
  "webhook_url": "https://…",            // ignored — callbacks go to the
                                         // customer's webhook_url (admin API)
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

Tenant-scoped download. With `STORAGE_BACKEND=s3` the route **302-redirects
to a fresh presigned URL** (minted against `S3_PUBLIC_ENDPOINT`, valid for
`S3_URL_TTL_MINUTES` — the spec's 15-minute TTL); the bucket also carries a
1-day lifecycle-expiry backstop. With `local` it streams the bytes directly
(`application/zip` or `application/pdf`).

The `results.download_url` in 201/status responses is the presigned URL
itself under s3 — re-minted on every read, never persisted — or the tenant
route path under local.

`output_type` is `ZIP` (`{name}_{request_id}.zip`, documents inside as
`NN_{template}.pdf`), or `PDF` (`{name}_{request_id}.pdf`) for
single-document packages when `SINGLE_DOC_SKIP_ZIP=1`.

## Webhooks (MPE-37)

When the customer has `webhook_url` set (admin API), a terminal request
(COMPLETED or FAILED) triggers a signed callback:

```http
POST {customers.webhook_url}
Content-Type: application/json
X-ISPPr-Signature: sha256=<hex>   # HMAC-SHA256(WEBHOOK_SECRET, raw body)
```

Body is the same egress shape as the status endpoint (`request_id`,
`status`, `metrics`, `results.download_url` — presigned URL re-minted per
attempt, `error` on failure). Receivers verify:

```
hmac_sha256(webhook_secret, raw_body) == signature[7:]
```

Retries: `WEBHOOK_MAX_ATTEMPTS` (default 5), exponential backoff
(1s→16s), 5s per-attempt timeout, any 2xx = delivered. The request row
records `webhook_status` (NONE/PENDING/DELIVERED/EXHAUSTED) and a
`webhook_log` of every attempt — visible via `GET /admin/requests/{id}`.

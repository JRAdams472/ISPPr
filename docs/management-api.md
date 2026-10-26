# Management API

Admin-facing REST surface for the portal — `/api/v1/admin/*`, session-gated.
Distinct from the tenant generation surface (`X-API-Key`, MPE-30). All errors
are RFC 7807 `application/problem+json`.

## Auth

```http
POST /api/v1/admin/login          {"email":"…","password":"…"}
→ 200 {"token":"ses_tok_…","expires_at":"…","user":{"id","email","name","role"}}
       + Set-Cookie: isppr_admin=<token>; HttpOnly; SameSite=Lax
→ 401 invalid credentials (same body for unknown email — no enumeration)

POST /api/v1/admin/logout         (authed) → 204, clears session + cookie
GET  /api/v1/admin/me             → {"id","email","name","role"}
```

Present the token as `Authorization: Bearer <token>` or the `isppr_admin`
cookie. Sessions live in `admin_sessions` (sha-256 hash only), 12h TTL.

**Roles.** Every authenticated session can read. Mutating routes
(POST/PUT/PATCH/DELETE) additionally require `role: admin`; other roles get
`403 admin role required`. `/login` and `/logout` are open to all sessions —
signing out is not a destructive action. The dev seed ships
`admin@isppr.local` (admin) and `viewer@isppr.local` (viewer), both
`isppr-admin`.

**Secret material is never re-readable.** API-key plaintext is returned
exactly once at create/rotate; password/key hashes never appear in responses.

## Resources

### Clients

```http
GET   /admin/clients                        → [client]
POST  /admin/clients   {"name","status"?}   → 201 client   (status=active default)
GET   /admin/clients/:id                    → client | 404
PATCH /admin/clients/:id {"name"?,"status"?}→ client
```

`client` = `{id:"cli_…", name, status:"active"|"disabled", created_at}`

### Customers

```http
GET   /admin/clients/:id/customers                    → [customer]
POST  /admin/clients/:id/customers {"name","external_ref"?,"webhook_url"?} → 201
GET   /admin/customers/:id                            → customer | 404
PATCH /admin/customers/:id {"name"?,"external_ref"?,"webhook_url"?} → customer
```

`customer` = `{id:"cust_…", client_id, external_ref?, name, webhook_url?, created_at}`

### API keys

```http
GET  /admin/clients/:id/api-keys                 → [key]      (no hashes)
POST /admin/clients/:id/api-keys {"label"?,"scopes"?} → 201 key + `"key"` plaintext
POST /admin/api-keys/:id/rotate                  → key + `"key"` plaintext, rotated_at set
POST /admin/api-keys/:id/revoke                  → key, revoked_at set
```

`key` = `{id:"key_…", client_id, label, scopes:[…], created_at, rotated_at?,
revoked_at?, last_used_at?, key?}`. Rotation kills the old credential
immediately (hash replaced); revoke sets `revoked_at`, auth ignores it.
Default scope `["generate"]`.

### Template masters (versioned)

```http
GET  /admin/clients/:id/template-masters                       → [master] (all versions)
POST /admin/clients/:id/template-masters {"name","html","css"?,"defaults"?} → 201 v1 | 409 if name exists
POST /admin/clients/:id/template-masters/:name/versions {"html","css"?,"defaults"?} → 201 next version | 404
POST /admin/template-masters/preview {"html","css"?,"defaults"?,"data"?} → PDF (draft, nothing persisted)
GET  /admin/template-masters/:id                               → master | 404
```

`master` = `{id:"tmpl_…", client_id, name, version, html, css, defaults:{…},
created_at, updated_at}`. Masters are append-only — publish creates a new
version row; merge semantics in `docs/template-resolution.md`.

`defaults` accepts `toggles`, `branding`, `page` (print layout: size,
orientation, margins, `header_html`/`footer_html`), and `print`
(`{"enabled":true}` — Ghostscript CMYK/PDF-X post-processing on
generated PDFs; see `docs/rendering.md`). An invalid `page` block
(unknown size, negative margin, non-compiling header/footer source) is
rejected with 400 on create/publish/preview.

### Template variants

```http
PUT /admin/template-variants {"master_id","customer_id","overlay"?} → variant (upsert)
GET /admin/template-variants?master_id=…&customer_id=…             → variant | 404
GET /admin/customers/:id/variants                                  → [variant]
POST /admin/template-variants/preview {"master_id","overlay"?,"data"?}
                                                                 → application/pdf | 400 | 404
```

`variant` = `{id:"var_…", master_id, customer_id, overlay:{"css_patch"?,
"toggles"?,"branding"?}, updated_at}`

`/preview` resolves the master plus the *draft* overlay in memory (same
base ← master ← overlay chain generation uses), renders `data` through the
Handlebars surface, and returns the Gotenberg PDF. Nothing is persisted —
safe for draft editing; available to any authenticated session (readers
included).

### Requests (ops view)

```http
GET /admin/requests?client_id&status&limit → [request, …]
GET /admin/requests/{id} →
{"id":"req_…","client_id","customer_id","package_name","status",
 "error"?,"metrics"?,"webhook_status":"NONE|PENDING|DELIVERED|EXHAUSTED",
 "webhook_log":[{"at":"…","attempt":1,"code":200}],"created_at",
 "results"?:{"output_type":"ZIP|PDF","filename":"….zip",
            "download_url":"<fresh presigned URL>"}}
GET /admin/requests/{id}/download → 302 | application/zip | application/pdf | 404 | 409
```

`GET /admin/requests` lists requests newest-first; `client_id` and
`status` are optional filters, `limit` caps the page (1–500, default all).
Cross-tenant ops view of a generation request, including webhook
delivery status and the per-attempt log (MPE-37).

The detail view carries `results` only for COMPLETED rows with a stored
artifact (MPE-43): `download_url` is a fresh presigned URL under S3/MinIO
(never persisted — every read signs anew within the TTL), or the
`/download` route under the local dev backend. `/download` streams the
artifact directly (or 302-redirects to a fresh presigned URL under S3)
and 409s while the request is not COMPLETED.

### Usage (dashboard)

```http
GET /admin/usage?days=30[&client_id=cli_…] →
{"days":30,
 "by_day":[{"day":"YYYY-MM-DD","client_id","customer_id","requests","completed","failed"}],
 "latency":{"p50_ms":0,"p95_ms":0,"samples":0}}
```

Aggregates `generation_requests`; `latency` percentiles interpolate over
`metrics.total_duration_ms` (0/0/0 until the render pipeline emits
metrics). `client_id` scopes both aggregates to a single client.

## Error shape

```json
{"type":"about:blank","title":"Unauthorized","status":401,
 "detail":"session required","instance":"/api/v1/admin/clients"}
```

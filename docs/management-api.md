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
GET  /admin/template-masters/:id                               → master | 404
```

`master` = `{id:"tmpl_…", client_id, name, version, html, css, defaults:{…},
created_at, updated_at}`. Masters are append-only — publish creates a new
version row; merge semantics in `docs/template-resolution.md`.

### Template variants

```http
PUT /admin/template-variants {"master_id","customer_id","overlay"?} → variant (upsert)
GET /admin/template-variants?master_id=…&customer_id=…             → variant | 404
GET /admin/customers/:id/variants                                  → [variant]
```

`variant` = `{id:"var_…", master_id, customer_id, overlay:{"css_patch"?,
"toggles"?,"branding"?}, updated_at}`

### Requests (ops view)

```http
GET /admin/requests/{id} →
{"id":"req_…","client_id","customer_id","package_name","status",
 "error"?,"metrics"?,"webhook_status":"NONE|PENDING|DELIVERED|EXHAUSTED",
 "webhook_log":[{"at":"…","attempt":1,"code":200}],"created_at"}
```

Cross-tenant ops view of a generation request, including webhook
delivery status and the per-attempt log (MPE-37).

### Usage (dashboard)

```http
GET /admin/usage?days=30 →
{"days":30,
 "by_day":[{"day":"YYYY-MM-DD","client_id","customer_id","requests","completed","failed"}],
 "latency":{"p50_ms":0,"p95_ms":0,"samples":0}}
```

Aggregates `generation_requests`; `latency` percentiles interpolate over
`metrics.latency_ms` (0/0/0 until the render pipeline emits metrics).

## Error shape

```json
{"type":"about:blank","title":"Unauthorized","status":401,
 "detail":"session required","instance":"/api/v1/admin/clients"}
```

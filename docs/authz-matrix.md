# Authorization Matrix — Tenant Surface

The tenant surface (`/api/v1/packages/*`) is gated by `X-API-Key`
(`internal/tenancy`). Authorization has two layers, both enforced in the
engine — never only at the gateway:

1. **Authentication** — `tenancy.Middleware` resolves the presented key to a
   client + key row and stores both in the Echo context.
2. **Ownership** — `tenancy.RequireOwnership` verifies the resolved client
   owns the payload's `customer_id` and every `template_id`. Any mismatch is
   a **global deny**; a request never partially resolves.

All failures return RFC 7807 `application/problem+json` via the central
`ProblemHandler`.

## Matrix

| # | Request state | Result | Covered by |
|---|---------------|--------|------------|
| 1 | No `X-API-Key` header | `401` | `TestTenantAuth_MissingKey` |
| 2 | Unknown / malformed key | `401` | `TestTenantAuth_InvalidKey` |
| 3 | Revoked key (`revoked_at` set) | `401` | `TestTenantAuth_RevokedKey` |
| 4 | Rotated key (old plaintext after `RotateAPIKey`) | `401` | `TestTenantAuth_RotatedKey` |
| 5 | Valid key, missing `generate` scope | `403` | `TestTenantAuth_MissingScope` |
| 6 | Valid key, tenant `status != active` | `403` | `TestTenantAuth_DisabledClient` |
| 7 | Valid key + scope, own resources | `200/204`, tenant context set | `TestTenantAuth_HappyPath`, `TestOwnership_OwnResources` |
| 8 | Payload `customer_id` owned by another client | `403` | `TestOwnership_CrossTenantCustomer` |
| 9 | Payload `template_id` owned by another client | `403` | `TestOwnership_CrossTenantTemplate` |
| 10 | Mixed ownership (own + foreign template) | `403` — whole request | `TestOwnership_GlobalDenyOnAnyMismatch` |
| 11 | Unknown `template_id` (exists for nobody) | `403` | `TestOwnership_MissingTemplate` |

## Notes on the key model

- Keys are stored as sha-256 hashes; lookup filters `revoked_at IS NULL`,
  then the fetched hash is re-verified with `subtle.ConstantTimeCompare`.
- There is **no `expires_at` column** — "expired" key material arises from
  **rotation** (the stored hash is replaced; old plaintext → row 4 above)
  or **revocation** (row 3). Both fail at the same lookup.
- "Key owning subset of resources" (row 10): a key is scoped to a whole
  client, so partial-ownership payloads are denied wholesale — a key that
  legitimately owns only some of the referenced resources gets `403`.
- Successful auth stamps `api_keys.last_used_at` (verified in
  `TestTenantAuth_HappyPath`).

## Where the checks live

| Layer | Code |
|-------|------|
| Key extraction + tenant resolution | `internal/tenancy/auth.go` → `Middleware` |
| Context accessors | `tenancy.Client`, `tenancy.APIKey` |
| In-engine ownership gate | `tenancy.RequireOwnership` |
| Tenant-scoped reads | `GetActiveAPIKeyByHash`, `GetCustomerForClient`, `GetMasterForClient` |
| Problem serialization | `internal/server/problem.go` |

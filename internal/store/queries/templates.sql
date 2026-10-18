-- name: CreateTemplateMaster :one
INSERT INTO template_masters (id, client_id, name, html, css, version, defaults)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: NextMasterVersion :one
SELECT coalesce(max(version), 0) + 1 AS next_version
FROM template_masters
WHERE client_id = $1 AND name = $2;

-- name: GetMaster :one
SELECT * FROM template_masters WHERE id = $1;

-- name: GetMasterForClient :one
-- Tenant-scoped read: a master is only visible to its owning client.
SELECT * FROM template_masters WHERE id = $1 AND client_id = $2;

-- name: GetLatestMaster :one
SELECT * FROM template_masters
WHERE client_id = $1 AND name = $2
ORDER BY version DESC
LIMIT 1;

-- name: ListMasters :many
SELECT * FROM template_masters
WHERE client_id = $1
ORDER BY name, version DESC;

-- name: UpsertVariant :one
INSERT INTO template_variants (id, master_id, customer_id, overlay)
VALUES ($1, $2, $3, $4)
ON CONFLICT (master_id, customer_id)
DO UPDATE SET overlay = $4, updated_at = now()
RETURNING *;

-- name: GetVariant :one
SELECT * FROM template_variants
WHERE master_id = $1 AND customer_id = $2;

-- Resolution lookup: the variant a customer set on ANY version of the named
-- master, preferring the one attached to the newest version.
-- name: ResolveVariant :one
SELECT v.* FROM template_variants v
JOIN template_masters m ON m.id = v.master_id
WHERE m.client_id = $1 AND m.name = $2 AND v.customer_id = $3
ORDER BY m.version DESC
LIMIT 1;

-- name: ListVariantsForCustomer :many
SELECT * FROM template_variants
WHERE customer_id = $1
ORDER BY created_at;

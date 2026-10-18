-- name: CreateAPIKey :one
INSERT INTO api_keys (id, client_id, key_hash, label, scopes)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetAPIKeyByHash :one
SELECT * FROM api_keys WHERE key_hash = $1;

-- name: GetAPIKey :one
SELECT * FROM api_keys WHERE id = $1;

-- name: ListAPIKeysForClient :many
SELECT * FROM api_keys WHERE client_id = $1 ORDER BY created_at;

-- name: TouchAPIKeyLastUsed :exec
UPDATE api_keys SET last_used_at = now() WHERE key_hash = $1;

-- name: RotateAPIKey :one
UPDATE api_keys SET key_hash = $2, rotated_at = now()
WHERE id = $1
RETURNING *;

-- name: RevokeAPIKey :one
UPDATE api_keys SET revoked_at = now()
WHERE id = $1
RETURNING *;

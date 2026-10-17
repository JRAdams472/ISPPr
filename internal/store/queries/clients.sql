-- name: GetClientByAPIKeyHash :one
SELECT c.* FROM clients c
JOIN api_keys k ON k.client_id = c.id
WHERE k.key_hash = $1 AND k.revoked_at IS NULL;

-- name: GetCustomerForClient :one
SELECT * FROM customers
WHERE id = $1 AND client_id = $2;

-- name: TouchAPIKeyLastUsed :exec
UPDATE api_keys SET last_used_at = now()
WHERE key_hash = $1;

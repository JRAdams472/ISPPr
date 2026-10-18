-- name: CreateClient :one
INSERT INTO clients (id, name, status)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetClient :one
SELECT * FROM clients WHERE id = $1;

-- name: GetClientByAPIKeyHash :one
SELECT c.* FROM clients c
JOIN api_keys k ON k.client_id = c.id
WHERE k.key_hash = $1 AND k.revoked_at IS NULL;

-- name: ListClients :many
SELECT * FROM clients ORDER BY created_at;

-- name: SetClientStatus :one
UPDATE clients SET status = $2 WHERE id = $1
RETURNING *;

-- name: UpdateClient :one
UPDATE clients
SET name = coalesce(sqlc.narg(name), name),
    status = coalesce(sqlc.narg(status), status)
WHERE id = sqlc.arg(id)
RETURNING *;

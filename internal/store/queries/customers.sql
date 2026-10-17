-- name: CreateCustomer :one
INSERT INTO customers (id, client_id, external_ref, name, webhook_url)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetCustomerForClient :one
-- Tenant-scoped read: a customer is only visible to its owning client.
SELECT * FROM customers WHERE id = $1 AND client_id = $2;

-- name: GetCustomerByExternalRef :one
SELECT * FROM customers WHERE client_id = $1 AND external_ref = $2;

-- name: ListCustomersForClient :many
SELECT * FROM customers WHERE client_id = $1 ORDER BY created_at;

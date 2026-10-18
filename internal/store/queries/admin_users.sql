-- name: CreateAdminUser :one
INSERT INTO admin_users (id, email, name, password_hash, role)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetAdminUserByEmail :one
SELECT * FROM admin_users WHERE email = $1;

-- name: GetAdminUser :one
SELECT * FROM admin_users WHERE id = $1;

-- name: CreateAdminSession :one
INSERT INTO admin_sessions (id, admin_user_id, token_hash, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetSessionByTokenHash :one
SELECT * FROM admin_sessions
WHERE token_hash = $1 AND expires_at > now();

-- name: DeleteSessionByTokenHash :exec
DELETE FROM admin_sessions WHERE token_hash = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM admin_sessions WHERE expires_at <= now();

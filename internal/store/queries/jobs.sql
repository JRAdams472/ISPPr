-- name: CreateGenerationRequest :one
INSERT INTO generation_requests (id, client_id, customer_id, idempotency_key, package_name, status)
VALUES ($1, $2, $3, $4, $5, 'QUEUED')
RETURNING *;

-- name: GetRequestByIdempotency :one
SELECT * FROM generation_requests
WHERE client_id = $1 AND idempotency_key = $2;

-- name: GetRequestForClient :one
SELECT * FROM generation_requests WHERE id = $1 AND client_id = $2;

-- name: GetRequest :one
SELECT * FROM generation_requests WHERE id = $1;

-- name: SetRequestStatus :exec
UPDATE generation_requests SET status = $2, error = $3 WHERE id = $1;

-- name: CompleteRequest :exec
UPDATE generation_requests SET status = 'COMPLETED', metrics = $2, output_key = $3 WHERE id = $1;

-- name: CreateJob :one
INSERT INTO generation_jobs (id, request_id, client_id, payload)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ClaimJob :one
-- Atomic claim: pending first, then expired claims (worker-crash
-- recovery). FOR UPDATE SKIP LOCKED lets workers race safely.
UPDATE generation_jobs
SET status = 'CLAIMED', claimed_at = now(),
    lease_until = now() + interval '120 seconds', attempts = attempts + 1
WHERE id = (
    SELECT id FROM generation_jobs
    WHERE status = 'QUEUED' OR (status = 'CLAIMED' AND lease_until < now())
    ORDER BY created_at
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: FinishJob :exec
UPDATE generation_jobs SET status = $2, finished_at = now(), error = $3 WHERE id = $1;

-- name: RequeueJob :exec
UPDATE generation_jobs SET status = 'QUEUED', error = $2 WHERE id = $1;

-- name: GetJobForRequest :one
SELECT * FROM generation_jobs WHERE request_id = $1;

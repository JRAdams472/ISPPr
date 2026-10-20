-- Dashboard aggregates over the generation-request audit log.
-- name: UsageByDay :many
SELECT date_trunc('day', created_at)::date AS day,
       client_id,
       customer_id,
       count(*) AS requests,
       count(*) FILTER (WHERE status = 'COMPLETED') AS completed,
       count(*) FILTER (WHERE status = 'FAILED') AS failed
FROM generation_requests
WHERE created_at >= now() - make_interval(days => $1)
GROUP BY 1, 2, 3
ORDER BY 1 DESC, 2, 3;

-- name: UsageByDayForClient :many
SELECT date_trunc('day', created_at)::date AS day,
       client_id,
       customer_id,
       count(*) AS requests,
       count(*) FILTER (WHERE status = 'COMPLETED') AS completed,
       count(*) FILTER (WHERE status = 'FAILED') AS failed
FROM generation_requests
WHERE created_at >= now() - make_interval(days => $1)
  AND client_id = $2
GROUP BY 1, 2, 3
ORDER BY 1 DESC, 2, 3;

-- name: UsageLatency :one
SELECT coalesce(percentile_cont(0.5) WITHIN GROUP (ORDER BY (metrics->>'total_duration_ms')::float8), 0)::float8 AS p50_ms,
       coalesce(percentile_cont(0.95) WITHIN GROUP (ORDER BY (metrics->>'total_duration_ms')::float8), 0)::float8 AS p95_ms,
       count(*) AS samples
FROM generation_requests
WHERE created_at >= now() - make_interval(days => $1)
  AND metrics ? 'total_duration_ms';

-- name: UsageLatencyForClient :one
SELECT coalesce(percentile_cont(0.5) WITHIN GROUP (ORDER BY (metrics->>'total_duration_ms')::float8), 0)::float8 AS p50_ms,
       coalesce(percentile_cont(0.95) WITHIN GROUP (ORDER BY (metrics->>'total_duration_ms')::float8), 0)::float8 AS p95_ms,
       count(*) AS samples
FROM generation_requests
WHERE created_at >= now() - make_interval(days => $1)
  AND client_id = $2
  AND metrics ? 'total_duration_ms';

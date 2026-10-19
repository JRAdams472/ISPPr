-- MPE-35: Postgres-backed job queue for package generation.
-- generation_requests (0001) is the durable audit/idempotency record;
-- generation_jobs carries scheduling state (claim, lease, attempts).
CREATE TABLE generation_jobs (
    id          TEXT PRIMARY KEY,             -- job_*
    request_id  TEXT NOT NULL REFERENCES generation_requests (id),
    client_id   TEXT NOT NULL REFERENCES clients (id),
    payload     JSONB NOT NULL,               -- validated documents[] snapshot
    status      TEXT NOT NULL DEFAULT 'QUEUED', -- QUEUED | CLAIMED | DONE | FAILED
    attempts    INT  NOT NULL DEFAULT 0,
    claimed_at  TIMESTAMPTZ,
    lease_until TIMESTAMPTZ,
    error       TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ
);

-- Claim scan: pending rows, plus claimed rows whose lease expired
-- (worker crash recovery).
CREATE INDEX generation_jobs_claim ON generation_jobs (status, lease_until)
    WHERE status IN ('QUEUED', 'CLAIMED');

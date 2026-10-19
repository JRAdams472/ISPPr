-- MPE-37: webhook delivery bookkeeping on the request row.
-- webhook_status: NONE (no target), PENDING (delivering), DELIVERED,
-- EXHAUSTED (retries spent). webhook_log: JSONB array of attempt entries
-- {at, attempt, code|error}.
ALTER TABLE generation_requests
    ADD COLUMN webhook_status TEXT NOT NULL DEFAULT 'NONE',
    ADD COLUMN webhook_log    JSONB NOT NULL DEFAULT '[]';

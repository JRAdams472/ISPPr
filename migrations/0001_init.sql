-- 0001_init.sql — multi-tenant DGE schema: clients, customers, API keys,
-- template masters/variants, and the generation-request audit/idempotency log.

CREATE TABLE clients (
    id         TEXT PRIMARY KEY,             -- cli_*
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE api_keys (
    id           TEXT PRIMARY KEY,           -- key_*
    client_id    TEXT NOT NULL REFERENCES clients (id),
    key_hash     TEXT NOT NULL UNIQUE,       -- sha-256 of the presented key
    label        TEXT NOT NULL DEFAULT '',
    revoked_at   TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE customers (
    id         TEXT PRIMARY KEY,             -- cust_*
    client_id  TEXT NOT NULL REFERENCES clients (id),
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE template_masters (
    id         TEXT PRIMARY KEY,             -- tmpl_*
    client_id  TEXT NOT NULL REFERENCES clients (id),
    name       TEXT NOT NULL,
    html       TEXT NOT NULL,                -- Handlebars source
    css        TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- A variant is a per-customer overlay on a master: CSS patches, toggle blocks,
-- branding paths. Resolution order: variant overlay -> client master -> base.
CREATE TABLE template_variants (
    id          TEXT PRIMARY KEY,            -- var_*
    master_id   TEXT NOT NULL REFERENCES template_masters (id),
    customer_id TEXT NOT NULL REFERENCES customers (id),
    overlay     JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (master_id, customer_id)
);

-- Audit row per POST /api/v1/packages/generate. The idempotency key makes
-- retried submissions safe: one row per (client, key); NULLs never collide.
CREATE TABLE generation_requests (
    id              TEXT PRIMARY KEY,        -- guid
    client_id       TEXT NOT NULL REFERENCES clients (id),
    customer_id     TEXT NOT NULL REFERENCES customers (id),
    idempotency_key TEXT,
    package_name    TEXT NOT NULL,
    status          TEXT NOT NULL,           -- COMPLETED | FAILED
    metrics         JSONB NOT NULL DEFAULT '{}',
    output_key      TEXT,                    -- S3 object key of the archive
    error           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (client_id, idempotency_key)
);

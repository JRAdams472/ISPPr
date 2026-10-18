-- Portal sessions for the management API. Opaque tokens — only the sha-256
-- hash is stored, so a DB read never hands out a usable credential.
CREATE TABLE admin_sessions (
    id            TEXT PRIMARY KEY,           -- ses_*
    admin_user_id TEXT NOT NULL REFERENCES admin_users (id),
    token_hash    TEXT NOT NULL UNIQUE,       -- sha-256 of the bearer token
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX admin_sessions_expiry ON admin_sessions (expires_at);

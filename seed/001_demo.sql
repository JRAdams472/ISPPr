-- Demo tenant for local development. Applied when the API boots with
-- RUN_SEED=1. Produces one usable client -> customer -> api key chain plus a
-- portal admin login.
--
--   client     cli_demo  "Demo Corp"
--   customer   cust_demo (external_ref demo-cust-1)
--   api key    isppr_dev_demo   (sha-256 stored; dev-only credential)
--   admin      admin@isppr.local / isppr-admin

INSERT INTO clients (id, name, status)
VALUES ('cli_demo', 'Demo Corp', 'active')
ON CONFLICT (id) DO NOTHING;

INSERT INTO customers (id, client_id, external_ref, name)
VALUES ('cust_demo', 'cli_demo', 'demo-cust-1', 'Demo Customer')
ON CONFLICT (id) DO NOTHING;

INSERT INTO api_keys (id, client_id, key_hash, label, scopes)
VALUES (
    'key_demo',
    'cli_demo',
    'a081342f8ddce4cdd9474456259dac7596ed05d924a641cf581bce06196c6829',
    'demo key',
    '{generate}'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO admin_users (id, email, name, password_hash, role)
VALUES (
    'adm_demo',
    'admin@isppr.local',
    'Demo Admin',
    '$2a$10$FGtHHFYFlHn.Fyy4nqt/W.GvADoT3kwdXsVEnTNiq6U6lXoQ8dfPW',
    'admin'
)
ON CONFLICT (id) DO NOTHING;

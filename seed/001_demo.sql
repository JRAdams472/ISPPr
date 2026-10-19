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

-- Read-only account for exercising the role gate; same dev password.
INSERT INTO admin_users (id, email, name, password_hash, role)
VALUES (
    'adm_viewer',
    'viewer@isppr.local',
    'Demo Viewer',
    '$2a$10$FGtHHFYFlHn.Fyy4nqt/W.GvADoT3kwdXsVEnTNiq6U6lXoQ8dfPW',
    'viewer'
)
ON CONFLICT (id) DO NOTHING;

-- A usable template so the demo tenant can POST /packages/generate out of
-- the box. Name matches the spec's style (tmpl_*); payload fields: title,
-- total, due, items[].
INSERT INTO template_masters (id, client_id, name, html, css, version, defaults)
VALUES (
    'tm_demo_invoice',
    'cli_demo',
    'tmpl_invoice',
    '<html><head></head><body>{{#if blocks.header}}<header class="hdr">INVOICE</header>{{/if}}<h1>{{title}}</h1><table>{{#each items}}<tr><td>{{title desc}}</td><td>{{currency price "USD"}}</td></tr>{{/each}}</table><p class="total">Total: {{currency total "USD"}}</p><p>Due: {{due}}</p>{{#if blocks.promo}}<footer class="promo">Thanks for your business!</footer>{{/if}}</body></html>',
    '.hdr{font-size:24px;font-weight:bold;color:#1a56db}table{width:100%;border-collapse:collapse}td{border-bottom:1px solid #ddd;padding:6px}.total{font-size:18px;font-weight:bold}.promo{color:#666;font-style:italic}',
    1,
    '{"toggles":{"header":true,"promo":false},"branding":{"logo":"brand/demo.png"}}'
)
ON CONFLICT (id) DO NOTHING;

-- Template version history + master-owned merge defaults.
--
-- Masters are append-only per (client_id, name, version): publishing a new
-- layout revision INSERTs a row with the next version, never UPDATEs an old
-- one (resolution always picks the highest version). `defaults` carries the
-- master's own toggle/branding map that customer variants overlay onto.
ALTER TABLE template_masters
    ADD COLUMN version  INT   NOT NULL DEFAULT 1,
    ADD COLUMN defaults JSONB NOT NULL DEFAULT '{}';

ALTER TABLE template_masters
    ADD CONSTRAINT template_masters_client_name_version_key
    UNIQUE (client_id, name, version);

-- Spike T-B008 (supuesto 2 de research.md): does sqlc parse a schema that uses CREATE POLICY and
-- column-level GRANT? This file is read by sqlc AFTER db/migrations (see sqlc.yaml) and is NOT a
-- migration: nothing here is ever applied to a database. It mimics what Phase 1 migrations will
-- contain (data-model.md §3.2, §3.4). Delete it together with this package once the real tables
-- and queries of Phase 1 exercise the same syntax.

-- Stand-in for migration 00002 (app.current_tenant_id).
CREATE FUNCTION app.current_tenant_id() RETURNS uuid
LANGUAGE sql STABLE PARALLEL SAFE AS $$
  SELECT CASE
    WHEN current_user ~ '^crm_t_[0-9a-f]{32}$'
    THEN substr(current_user, 7)::uuid
  END
$$;

CREATE TABLE app.spike_items (
    id          uuid        NOT NULL DEFAULT uuidv7(),
    tenant_id   uuid        NOT NULL,
    email       text        NOT NULL,
    secret_hash bytea,
    ip          inet,
    parent_id   uuid,
    created_at  timestamptz NOT NULL,
    PRIMARY KEY (id),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, parent_id) REFERENCES app.spike_items (tenant_id, id)
);

ALTER TABLE app.spike_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.spike_items FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON app.spike_items
    FOR ALL TO PUBLIC
    USING (tenant_id = (SELECT app.current_tenant_id()))
    WITH CHECK (tenant_id = (SELECT app.current_tenant_id()));

CREATE POLICY auth_lookup ON app.spike_items FOR SELECT TO crm_auth USING (true);

GRANT SELECT, INSERT ON app.spike_items TO crm_tenant;
GRANT UPDATE (email, ip, parent_id) ON app.spike_items TO crm_tenant;
REVOKE UPDATE (ip) ON app.spike_items FROM crm_tenant;
GRANT SELECT (id, tenant_id, email) ON app.spike_items TO crm_auth;
GRANT SELECT (id, created_at), UPDATE (created_at) ON app.spike_items TO crm_worker;

CREATE INDEX spike_items_email_idx ON app.spike_items (email);

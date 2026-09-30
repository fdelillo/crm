-- +goose Up
-- Audit trail (data-model.md §2.6). Append-only by privileges: the company role gets SELECT and INSERT
-- from the default privileges and nothing else (INV-15).
CREATE TABLE app.audit_log (
    id            uuid        NOT NULL DEFAULT uuidv7(),
    tenant_id     uuid        NOT NULL,
    occurred_at   timestamptz NOT NULL DEFAULT now(),
    actor_user_id uuid,
    action        text        NOT NULL,
    target_type   text,
    target_id     uuid,      -- polymorphic: no foreign key
    data          jsonb       NOT NULL DEFAULT '{}',
    ip            inet,
    user_agent    text,
    request_id    text,
    CONSTRAINT audit_log_pkey PRIMARY KEY (id),
    CONSTRAINT audit_log_tenant_fk FOREIGN KEY (tenant_id) REFERENCES app.tenants (id),
    CONSTRAINT audit_log_actor_fk FOREIGN KEY (tenant_id, actor_user_id) REFERENCES app.users (tenant_id, id),
    CONSTRAINT audit_log_action_chk CHECK (action ~ '^[a-z_]+\.[a-z_]+$'),
    CONSTRAINT audit_log_target_type_chk CHECK (char_length(target_type) <= 40),
    CONSTRAINT audit_log_user_agent_chk CHECK (char_length(user_agent) <= 512),
    CONSTRAINT audit_log_request_id_chk CHECK (char_length(request_id) <= 64)
);

CREATE INDEX audit_log_tenant_time_idx ON app.audit_log (tenant_id, occurred_at DESC);

ALTER TABLE app.audit_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.audit_log FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON app.audit_log
    FOR ALL TO PUBLIC
    USING (tenant_id = (SELECT app.current_tenant_id()))
    WITH CHECK (tenant_id = (SELECT app.current_tenant_id()));

-- +goose Down
DROP TABLE app.audit_log;

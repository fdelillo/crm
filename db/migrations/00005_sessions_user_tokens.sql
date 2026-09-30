-- +goose Up
-- Sessions (data-model.md §2.3).
CREATE TABLE app.sessions (
    id             uuid        NOT NULL DEFAULT uuidv7(),
    tenant_id      uuid        NOT NULL,
    user_id        uuid        NOT NULL,
    token_hash     bytea       NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    last_seen_at   timestamptz NOT NULL DEFAULT now(),
    expires_at     timestamptz NOT NULL,
    revoked_at     timestamptz,
    revoked_reason text,
    ip             inet,
    user_agent     text,
    CONSTRAINT sessions_pkey PRIMARY KEY (id),
    CONSTRAINT sessions_tenant_fk FOREIGN KEY (tenant_id) REFERENCES app.tenants (id),
    CONSTRAINT sessions_user_fk FOREIGN KEY (tenant_id, user_id) REFERENCES app.users (tenant_id, id),
    CONSTRAINT sessions_token_hash_key UNIQUE (token_hash),
    CONSTRAINT sessions_token_hash_chk CHECK (octet_length(token_hash) = 32),
    CONSTRAINT sessions_expires_chk CHECK (expires_at > created_at),
    CONSTRAINT sessions_revoked_reason_chk CHECK (revoked_reason IN ('logout', 'password_reset', 'user_disabled')),
    CONSTRAINT sessions_revoked_pair_chk CHECK ((revoked_at IS NULL) = (revoked_reason IS NULL)),
    CONSTRAINT sessions_user_agent_chk CHECK (char_length(user_agent) <= 512)
);

CREATE INDEX sessions_user_open_idx ON app.sessions (tenant_id, user_id) WHERE revoked_at IS NULL;
CREATE INDEX sessions_expires_idx ON app.sessions (expires_at);

ALTER TABLE app.sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.sessions FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON app.sessions
    FOR ALL TO PUBLIC
    USING (tenant_id = (SELECT app.current_tenant_id()))
    WITH CHECK (tenant_id = (SELECT app.current_tenant_id()));

GRANT UPDATE ON app.sessions TO crm_tenant;

-- Resolving the session of every request: find the company from the token hash.
GRANT SELECT (id, tenant_id, token_hash) ON app.sessions TO crm_auth;
CREATE POLICY auth_lookup ON app.sessions FOR SELECT TO crm_auth USING (true);

-- Periodic cleanup: only expired or long-revoked rows can be deleted, whatever the code does.
GRANT SELECT (id, expires_at, revoked_at), DELETE ON app.sessions TO crm_worker;
CREATE POLICY worker_read ON app.sessions FOR SELECT TO crm_worker USING (true);
CREATE POLICY worker_cleanup ON app.sessions FOR DELETE TO crm_worker
    USING (expires_at < now() - interval '30 days' OR revoked_at < now() - interval '30 days');

-- One-time tokens (data-model.md §2.4): email verification, password reset, invitation.
CREATE TABLE app.user_tokens (
    id                 uuid        NOT NULL DEFAULT uuidv7(),
    tenant_id          uuid        NOT NULL,
    user_id            uuid        NOT NULL,
    purpose            text        NOT NULL,
    token_hash         bytea       NOT NULL,
    created_by_user_id uuid,
    created_at         timestamptz NOT NULL DEFAULT now(),
    expires_at         timestamptz NOT NULL,
    used_at            timestamptz,
    revoked_at         timestamptz,
    CONSTRAINT user_tokens_pkey PRIMARY KEY (id),
    CONSTRAINT user_tokens_tenant_fk FOREIGN KEY (tenant_id) REFERENCES app.tenants (id),
    CONSTRAINT user_tokens_user_fk FOREIGN KEY (tenant_id, user_id) REFERENCES app.users (tenant_id, id),
    CONSTRAINT user_tokens_created_by_fk FOREIGN KEY (tenant_id, created_by_user_id) REFERENCES app.users (tenant_id, id),
    CONSTRAINT user_tokens_purpose_chk CHECK (purpose IN ('email_verification', 'password_reset', 'invitation')),
    CONSTRAINT user_tokens_token_hash_key UNIQUE (token_hash),
    CONSTRAINT user_tokens_token_hash_chk CHECK (octet_length(token_hash) = 32),
    CONSTRAINT user_tokens_expires_chk CHECK (expires_at > created_at),
    CONSTRAINT user_tokens_used_or_revoked_chk CHECK (NOT (used_at IS NOT NULL AND revoked_at IS NOT NULL))
);

CREATE INDEX user_tokens_open_idx ON app.user_tokens (tenant_id, user_id, purpose)
    WHERE used_at IS NULL AND revoked_at IS NULL;
CREATE INDEX user_tokens_expires_idx ON app.user_tokens (expires_at);

ALTER TABLE app.user_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.user_tokens FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON app.user_tokens
    FOR ALL TO PUBLIC
    USING (tenant_id = (SELECT app.current_tenant_id()))
    WITH CHECK (tenant_id = (SELECT app.current_tenant_id()));

GRANT UPDATE ON app.user_tokens TO crm_tenant;

GRANT SELECT (id, tenant_id, token_hash, purpose) ON app.user_tokens TO crm_auth;
CREATE POLICY auth_lookup ON app.user_tokens FOR SELECT TO crm_auth USING (true);

GRANT SELECT (id, purpose, expires_at, used_at, revoked_at), DELETE ON app.user_tokens TO crm_worker;
CREATE POLICY worker_read ON app.user_tokens FOR SELECT TO crm_worker USING (true);
-- The open invitation of an invited user is never cleaned up, so the UI can show "Invitación vencida"
-- (DD-25): only used or revoked invitations, and the other purposes, can be deleted once expired.
CREATE POLICY worker_cleanup ON app.user_tokens FOR DELETE TO crm_worker
    USING (expires_at < now() - interval '30 days'
           AND (purpose <> 'invitation' OR used_at IS NOT NULL OR revoked_at IS NOT NULL));

-- +goose Down
DROP TABLE app.user_tokens;
DROP TABLE app.sessions;

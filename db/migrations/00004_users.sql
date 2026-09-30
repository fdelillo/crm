-- +goose Up
-- Users (data-model.md §2.2). One user belongs to one company; the email is unique across all of them (DD-2).
CREATE TABLE app.users (
    id                uuid        NOT NULL DEFAULT uuidv7(),
    tenant_id         uuid        NOT NULL,
    email             text        NOT NULL,
    name              text,
    password_hash     text,
    role              text        NOT NULL,
    status            text        NOT NULL,
    email_verified_at timestamptz,
    status_changed_at timestamptz NOT NULL DEFAULT now(),
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_pkey PRIMARY KEY (id),
    CONSTRAINT users_tenant_fk FOREIGN KEY (tenant_id) REFERENCES app.tenants (id),
    CONSTRAINT users_email_key UNIQUE (email),
    -- Target of the composite foreign keys of the other company tables (INV-07).
    CONSTRAINT users_tenant_id_id_key UNIQUE (tenant_id, id),
    CONSTRAINT users_email_chk CHECK (email = lower(btrim(email)) AND char_length(email) <= 254),
    CONSTRAINT users_name_chk CHECK (char_length(name) <= 120),
    CONSTRAINT users_password_hash_chk CHECK (char_length(password_hash) <= 255),
    CONSTRAINT users_role_chk CHECK (role IN ('admin', 'operator')),
    CONSTRAINT users_status_chk CHECK (status IN ('invited', 'active', 'disabled')),
    -- A disabled user without a password cannot become active again: the database forces "invited" (P-3).
    CONSTRAINT users_active_complete_chk CHECK (status <> 'active' OR (name IS NOT NULL AND password_hash IS NOT NULL))
);

CREATE INDEX users_tenant_created_idx ON app.users (tenant_id, created_at);

ALTER TABLE app.users ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.users FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON app.users
    FOR ALL TO PUBLIC
    USING (tenant_id = (SELECT app.current_tenant_id()))
    WITH CHECK (tenant_id = (SELECT app.current_tenant_id()));

GRANT UPDATE ON app.users TO crm_tenant;

-- Login and password reset find the company of an email with routing columns only (plan §4.4).
GRANT SELECT (id, tenant_id, email) ON app.users TO crm_auth;
CREATE POLICY auth_lookup ON app.users FOR SELECT TO crm_auth USING (true);

-- +goose Down
DROP TABLE app.users;

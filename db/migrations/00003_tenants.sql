-- +goose Up
-- The company (data-model.md §2.1). It is the one company table without tenant_id: its isolation is by id.
CREATE TABLE app.tenants (
    id                        uuid        NOT NULL,  -- UUIDv7 generated in Go: the role is created before the INSERT (ADR-008)
    name                      text        NOT NULL,
    legal_name                text,
    tax_id                    text,
    address                   text,
    phone                     text,
    email                     text,
    logo_object_key           text,
    logo_content_type         text,
    base_currency             text        NOT NULL,
    timezone                  text        NOT NULL DEFAULT 'America/Argentina/Buenos_Aires',
    industry_template_code    text        NOT NULL,
    industry_template_version integer     NOT NULL,
    created_at                timestamptz NOT NULL DEFAULT now(),
    updated_at                timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT tenants_pkey PRIMARY KEY (id),
    CONSTRAINT tenants_name_chk CHECK (char_length(name) BETWEEN 1 AND 120),
    CONSTRAINT tenants_legal_name_chk CHECK (char_length(legal_name) <= 200),
    CONSTRAINT tenants_tax_id_chk CHECK (tax_id ~ '^[0-9]{11}$'), -- the check digit is validated in Go (DD-16)
    CONSTRAINT tenants_address_chk CHECK (char_length(address) <= 300),
    CONSTRAINT tenants_phone_chk CHECK (char_length(phone) <= 50),
    CONSTRAINT tenants_email_chk CHECK (char_length(email) <= 254),
    CONSTRAINT tenants_logo_object_key_chk CHECK (char_length(logo_object_key) <= 300),
    CONSTRAINT tenants_logo_content_type_chk CHECK (logo_content_type IN ('image/png', 'image/jpeg')),
    CONSTRAINT tenants_logo_pair_chk CHECK ((logo_object_key IS NULL) = (logo_content_type IS NULL)),
    CONSTRAINT tenants_base_currency_chk CHECK (base_currency IN ('ARS', 'USD')),
    CONSTRAINT tenants_timezone_chk CHECK (char_length(timezone) <= 64),
    CONSTRAINT tenants_template_code_chk CHECK (char_length(industry_template_code) <= 64),
    CONSTRAINT tenants_template_version_chk CHECK (industry_template_version > 0)
);

ALTER TABLE app.tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.tenants FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON app.tenants
    FOR ALL TO PUBLIC
    USING (id = (SELECT app.current_tenant_id()))
    WITH CHECK (id = (SELECT app.current_tenant_id()));

-- SELECT and INSERT for crm_tenant come from the default privileges of 00001.
GRANT UPDATE ON app.tenants TO crm_tenant;

-- The worker reads only the id, to know which companies exist (reprovisioning roles, plan §4.4).
GRANT SELECT (id) ON app.tenants TO crm_worker;
CREATE POLICY worker_read ON app.tenants FOR SELECT TO crm_worker USING (true);

-- +goose Down
DROP TABLE app.tenants;

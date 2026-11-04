-- +goose Up
-- Company configuration (specs/002-configuracion/data-model.md §2 to §4, ADR-028, ADR-029): the labels and
-- quote defaults, the stages of the pipeline, the categories and the catalog of products and services.
-- Nothing is ever deleted physically (INV-3): crm_tenant gets no DELETE, and UPDATE only per column.

CREATE TABLE app.tenant_settings (
    id                      uuid        NOT NULL DEFAULT uuidv7(),
    tenant_id               uuid        NOT NULL,
    project_label_singular  text        NOT NULL,
    project_label_plural    text        NOT NULL,
    project_label_gender    text        NOT NULL,
    customer_label_singular text        NOT NULL,
    customer_label_plural   text        NOT NULL,
    customer_label_gender   text        NOT NULL,
    quote_label_singular    text        NOT NULL,
    quote_label_plural      text        NOT NULL,
    quote_label_gender      text        NOT NULL,
    quote_validity_days     integer     NOT NULL,
    quote_terms             text,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT tenant_settings_pkey PRIMARY KEY (id),
    CONSTRAINT tenant_settings_tenant_fk FOREIGN KEY (tenant_id) REFERENCES app.tenants (id),
    -- One row per company (INV-1).
    CONSTRAINT tenant_settings_tenant_id_key UNIQUE (tenant_id),
    CONSTRAINT tenant_settings_project_label_singular_chk CHECK (char_length(project_label_singular) BETWEEN 1 AND 40),
    CONSTRAINT tenant_settings_project_label_plural_chk CHECK (char_length(project_label_plural) BETWEEN 1 AND 40),
    CONSTRAINT tenant_settings_project_label_gender_chk CHECK (project_label_gender IN ('masculine', 'feminine')),
    CONSTRAINT tenant_settings_customer_label_singular_chk CHECK (char_length(customer_label_singular) BETWEEN 1 AND 40),
    CONSTRAINT tenant_settings_customer_label_plural_chk CHECK (char_length(customer_label_plural) BETWEEN 1 AND 40),
    CONSTRAINT tenant_settings_customer_label_gender_chk CHECK (customer_label_gender IN ('masculine', 'feminine')),
    CONSTRAINT tenant_settings_quote_label_singular_chk CHECK (char_length(quote_label_singular) BETWEEN 1 AND 40),
    CONSTRAINT tenant_settings_quote_label_plural_chk CHECK (char_length(quote_label_plural) BETWEEN 1 AND 40),
    CONSTRAINT tenant_settings_quote_label_gender_chk CHECK (quote_label_gender IN ('masculine', 'feminine')),
    CONSTRAINT tenant_settings_quote_validity_days_chk CHECK (quote_validity_days BETWEEN 1 AND 365),
    -- NULL means "no terms"; the empty string is never stored.
    CONSTRAINT tenant_settings_quote_terms_chk CHECK (char_length(quote_terms) BETWEEN 1 AND 2000)
);

CREATE TABLE app.stages (
    id         uuid        NOT NULL DEFAULT uuidv7(),
    tenant_id  uuid        NOT NULL,
    name       text        NOT NULL,
    type       text        NOT NULL,
    color      text        NOT NULL,
    position   integer     NOT NULL,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT stages_pkey PRIMARY KEY (id),
    CONSTRAINT stages_tenant_fk FOREIGN KEY (tenant_id) REFERENCES app.tenants (id),
    -- Target of the composite foreign keys of later specs (INV-07 of 001).
    CONSTRAINT stages_tenant_id_id_key UNIQUE (tenant_id, id),
    CONSTRAINT stages_name_chk CHECK (char_length(name) BETWEEN 1 AND 60),
    CONSTRAINT stages_type_chk CHECK (type IN ('open', 'won', 'lost')),
    CONSTRAINT stages_color_chk CHECK (color IN ('gray', 'blue', 'teal', 'green', 'yellow', 'orange', 'red', 'purple')),
    CONSTRAINT stages_position_chk CHECK (position >= 1)
);
-- INV-6. No unique index on position on purpose: PostgreSQL checks a non-deferrable unique index row by row,
-- so rewriting positions would fail halfway (DD-5).
CREATE UNIQUE INDEX stages_active_name_key ON app.stages (tenant_id, lower(name)) WHERE deleted_at IS NULL;

CREATE TABLE app.categories (
    id          uuid        NOT NULL DEFAULT uuidv7(),
    tenant_id   uuid        NOT NULL,
    name        text        NOT NULL,
    type        text        NOT NULL,
    system_key  text,
    archived_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT categories_pkey PRIMARY KEY (id),
    CONSTRAINT categories_tenant_fk FOREIGN KEY (tenant_id) REFERENCES app.tenants (id),
    CONSTRAINT categories_tenant_id_id_key UNIQUE (tenant_id, id),
    CONSTRAINT categories_name_chk CHECK (char_length(name) BETWEEN 1 AND 60),
    CONSTRAINT categories_type_chk CHECK (type IN ('income', 'expense')),
    -- Later specs add their keys by widening this CHECK and categories_system_type_chk (ADR-028).
    CONSTRAINT categories_system_key_chk CHECK (system_key IN
        ('customer_receipts', 'partner_contributions', 'supplier_payments', 'employee_payments', 'partner_withdrawals')),
    CONSTRAINT categories_system_type_chk CHECK (
        system_key IS NULL
        OR (system_key IN ('customer_receipts', 'partner_contributions') AND type = 'income')
        OR (system_key IN ('supplier_payments', 'employee_payments', 'partner_withdrawals') AND type = 'expense')),
    -- A system category is never archived, even if the service gets it wrong (INV-4).
    CONSTRAINT categories_system_not_archived_chk CHECK (system_key IS NULL OR archived_at IS NULL)
);
CREATE UNIQUE INDEX categories_system_key_key ON app.categories (tenant_id, system_key) WHERE system_key IS NOT NULL;
CREATE UNIQUE INDEX categories_active_name_key ON app.categories (tenant_id, type, lower(name)) WHERE archived_at IS NULL;

CREATE TABLE app.catalog_items (
    id                       uuid        NOT NULL DEFAULT uuidv7(),
    tenant_id                uuid        NOT NULL,
    name                     text        NOT NULL,
    description              text,
    unit                     text        NOT NULL,
    unit_label               text,
    reference_price_cents    bigint,
    reference_price_currency text,
    archived_at              timestamptz,
    created_at               timestamptz NOT NULL DEFAULT now(),
    updated_at               timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT catalog_items_pkey PRIMARY KEY (id),
    CONSTRAINT catalog_items_tenant_fk FOREIGN KEY (tenant_id) REFERENCES app.tenants (id),
    CONSTRAINT catalog_items_tenant_id_id_key UNIQUE (tenant_id, id),
    CONSTRAINT catalog_items_name_chk CHECK (char_length(name) BETWEEN 1 AND 120),
    CONSTRAINT catalog_items_description_chk CHECK (char_length(description) BETWEEN 1 AND 1000),
    CONSTRAINT catalog_items_unit_chk CHECK (unit IN ('unit', 'square_meter', 'linear_meter', 'hour', 'other')),
    CONSTRAINT catalog_items_unit_label_length_chk CHECK (char_length(unit_label) BETWEEN 1 AND 20),
    -- The name of the unit is there exactly when the unit is "other".
    CONSTRAINT catalog_items_unit_label_chk CHECK ((unit = 'other') = (unit_label IS NOT NULL)),
    -- Money (ADR-029, INV-7): amount and currency together or neither; 2^53 - 1 is the largest integer a JSON number carries.
    CONSTRAINT catalog_items_reference_price_cents_chk CHECK (reference_price_cents BETWEEN 0 AND 9007199254740991),
    CONSTRAINT catalog_items_reference_price_currency_chk CHECK (reference_price_currency IN ('ARS', 'USD')),
    CONSTRAINT catalog_items_reference_price_pair_chk CHECK ((reference_price_cents IS NULL) = (reference_price_currency IS NULL))
);

-- Row level security, exactly like the other company tables (data-model.md §3.1).
ALTER TABLE app.tenant_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.tenant_settings FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON app.tenant_settings
    FOR ALL TO PUBLIC
    USING (tenant_id = (SELECT app.current_tenant_id()))
    WITH CHECK (tenant_id = (SELECT app.current_tenant_id()));

ALTER TABLE app.stages ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.stages FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON app.stages
    FOR ALL TO PUBLIC
    USING (tenant_id = (SELECT app.current_tenant_id()))
    WITH CHECK (tenant_id = (SELECT app.current_tenant_id()));

ALTER TABLE app.categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.categories FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON app.categories
    FOR ALL TO PUBLIC
    USING (tenant_id = (SELECT app.current_tenant_id()))
    WITH CHECK (tenant_id = (SELECT app.current_tenant_id()));

ALTER TABLE app.catalog_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.catalog_items FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON app.catalog_items
    FOR ALL TO PUBLIC
    USING (tenant_id = (SELECT app.current_tenant_id()))
    WITH CHECK (tenant_id = (SELECT app.current_tenant_id()));

-- SELECT and INSERT for crm_tenant come from the default privileges of 00001. UPDATE is per column (INV-5):
-- never id, tenant_id or created_at, and never categories.type or categories.system_key. DELETE is never granted (INV-3).
GRANT UPDATE (project_label_singular, project_label_plural, project_label_gender,
              customer_label_singular, customer_label_plural, customer_label_gender,
              quote_label_singular, quote_label_plural, quote_label_gender,
              quote_validity_days, quote_terms, updated_at)
    ON app.tenant_settings TO crm_tenant;
GRANT UPDATE (name, type, color, position, deleted_at, updated_at) ON app.stages TO crm_tenant;
GRANT UPDATE (name, archived_at, updated_at) ON app.categories TO crm_tenant;
GRANT UPDATE (name, description, unit, unit_label, reference_price_cents, reference_price_currency, archived_at, updated_at)
    ON app.catalog_items TO crm_tenant;

-- Backfill of the companies that already exist (DD-4, data-model.md §4.2): the minimum configuration, so that
-- INV-1, INV-2 and INV-4 hold for them. crm_owner owns the tables, and FORCE would subject it to the policies,
-- under which it sees no row (current_tenant_id() is NULL for it): FORCE is lifted here, inside this transaction,
-- and restored below (ADR-004 §6). The migration is atomic, so it never commits without the restore.
ALTER TABLE app.tenants NO FORCE ROW LEVEL SECURITY;
ALTER TABLE app.tenant_settings NO FORCE ROW LEVEL SECURITY;
ALTER TABLE app.stages NO FORCE ROW LEVEL SECURITY;
ALTER TABLE app.categories NO FORCE ROW LEVEL SECURITY;
ALTER TABLE app.catalog_items NO FORCE ROW LEVEL SECURITY;

-- Stages and categories go first: the companies to backfill are those without a tenant_settings row yet.
INSERT INTO app.stages (tenant_id, name, type, color, position)
SELECT t.id, s.name, s.type, s.color, s.position
FROM app.tenants t
CROSS JOIN (VALUES
    ('Presupuesto', 'open', 'blue', 1),
    ('Aprobado', 'open', 'teal', 2),
    ('En curso', 'open', 'yellow', 3),
    ('Finalizado', 'won', 'green', 4),
    ('Perdido', 'lost', 'gray', 5)
) AS s (name, type, color, position)
WHERE NOT EXISTS (SELECT 1 FROM app.tenant_settings ts WHERE ts.tenant_id = t.id);

INSERT INTO app.categories (tenant_id, name, type, system_key)
SELECT t.id, c.name, c.type, c.system_key
FROM app.tenants t
CROSS JOIN (VALUES
    ('Cobros de clientes', 'income', 'customer_receipts'),
    ('Aportes de socios', 'income', 'partner_contributions'),
    ('Pagos a proveedores', 'expense', 'supplier_payments'),
    ('Pagos a empleados', 'expense', 'employee_payments'),
    ('Retiros de socios', 'expense', 'partner_withdrawals')
) AS c (name, type, system_key)
WHERE NOT EXISTS (SELECT 1 FROM app.tenant_settings ts WHERE ts.tenant_id = t.id);

INSERT INTO app.tenant_settings (tenant_id,
    project_label_singular, project_label_plural, project_label_gender,
    customer_label_singular, customer_label_plural, customer_label_gender,
    quote_label_singular, quote_label_plural, quote_label_gender, quote_validity_days)
SELECT t.id, 'Proyecto', 'Proyectos', 'masculine', 'Cliente', 'Clientes', 'masculine',
       'Presupuesto', 'Presupuestos', 'masculine', 15
FROM app.tenants t
WHERE NOT EXISTS (SELECT 1 FROM app.tenant_settings ts WHERE ts.tenant_id = t.id);

-- T-B107 of 001 fails if any table of app is left without FORCE.
ALTER TABLE app.tenants FORCE ROW LEVEL SECURITY;
ALTER TABLE app.tenant_settings FORCE ROW LEVEL SECURITY;
ALTER TABLE app.stages FORCE ROW LEVEL SECURITY;
ALTER TABLE app.categories FORCE ROW LEVEL SECURITY;
ALTER TABLE app.catalog_items FORCE ROW LEVEL SECURITY;

-- +goose Down
-- The configuration is lost: that is what reverting this spec means (data-model.md §4.2).
DROP TABLE app.catalog_items;
DROP TABLE app.categories;
DROP TABLE app.stages;
DROP TABLE app.tenant_settings;

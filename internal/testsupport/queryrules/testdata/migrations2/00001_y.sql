-- +goose Up
CREATE TABLE IF NOT EXISTS app.plain (
    id uuid NOT NULL
);

CREATE TABLE IF NOT EXISTS app.with_tenant (
    id        uuid NOT NULL,
    tenant_id uuid NOT NULL
);

CREATE TABLE app.late (
    id uuid NOT NULL
);
ALTER TABLE app.late ADD COLUMN tenant_id uuid NOT NULL;

CREATE TABLE app.late_if (id uuid NOT NULL);
ALTER TABLE IF EXISTS ONLY app.late_if ADD COLUMN IF NOT EXISTS tenant_id uuid, ADD COLUMN note text;

CREATE TABLE app.late_bare (id uuid NOT NULL);
ALTER TABLE app.late_bare ADD tenant_id uuid;

CREATE TABLE app.other_alter (id uuid NOT NULL);
ALTER TABLE app.other_alter ADD COLUMN tenant_note text;

-- +goose Down
DROP TABLE app.plain;

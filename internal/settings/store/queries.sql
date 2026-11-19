-- Locks of the configuration (plan section 11). Rows of configuration tables are only ever locked FOR NO KEY UPDATE
-- (002/INV-8): a foreign key check from a later spec takes FOR KEY SHARE and must not wait behind these.

-- name: LockTenantSettings :one
-- First statement of every write of stages, names and conditions (002/DD-5, DD-10): the writes of one company run one
-- after the other. No row: the company lost its tenant_settings row (002/INV-1); lockTenantSettings turns it into ErrTenantSettingsMissing.
SELECT id FROM app.tenant_settings WHERE tenant_id = @tenant_id FOR NO KEY UPDATE;

-- name: LockCategory :one
-- First statement of rename, archive and restore (002/DD-8, INV-8). Other company or no row: pgx.ErrNoRows, which the
-- use case turns into ErrNotFound. It does not lock tenant_settings.
SELECT * FROM app.categories WHERE tenant_id = @tenant_id AND id = @id FOR NO KEY UPDATE;

-- name: LockCatalogItem :one
-- First statement of update, archive and restore of a catalog item (002/DD-9, INV-8). Other company or no row:
-- pgx.ErrNoRows, which the use case turns into ErrNotFound. It does not lock tenant_settings.
SELECT * FROM app.catalog_items WHERE tenant_id = @tenant_id AND id = @id FOR NO KEY UPDATE;

-- Locks of the configuration (plan section 11). Rows of configuration tables are only ever locked FOR NO KEY UPDATE
-- (002/INV-8): a foreign key check from a later spec takes FOR KEY SHARE and must not wait behind these.

-- name: LockTenantSettings :one
-- First statement of every write of stages, names and conditions (002/DD-5, DD-10): the writes of one company run one
-- after the other. No row: the company lost its tenant_settings row (002/INV-1); lockTenantSettings turns it into ErrTenantSettingsMissing.
SELECT id FROM app.tenant_settings WHERE tenant_id = @tenant_id FOR NO KEY UPDATE;

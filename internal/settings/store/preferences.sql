-- Queries of the labels and quote defaults of the company (002/DD-10). Every one filters by the company (INV-04 of 001).

-- name: GetTenantSettings :one
-- No row: pgx.ErrNoRows, which readTenantSettings turns into ErrTenantSettingsMissing (002/DD-19).
SELECT * FROM app.tenant_settings WHERE tenant_id = @tenant_id;

-- name: UpdateLabels :exec
-- Runs after LockTenantSettings (002/DD-10): the writes of one company run one after the other.
UPDATE app.tenant_settings SET
    project_label_singular = @project_singular, project_label_plural = @project_plural, project_label_gender = @project_gender,
    customer_label_singular = @customer_singular, customer_label_plural = @customer_plural, customer_label_gender = @customer_gender,
    quote_label_singular = @quote_singular, quote_label_plural = @quote_plural, quote_label_gender = @quote_gender,
    updated_at = @now
WHERE tenant_id = @tenant_id;

-- name: UpdateQuoteDefaults :exec
-- Runs after LockTenantSettings (002/DD-10). A NULL terms means "no terms" (never an empty string).
UPDATE app.tenant_settings SET quote_validity_days = @validity_days, quote_terms = @terms, updated_at = @now
WHERE tenant_id = @tenant_id;

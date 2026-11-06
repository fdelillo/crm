-- The four statements of the registration seeding (002/DD-3, INV-10), one per table, each by sets: the values
-- travel as parallel arrays and the rows come out of unnest, never one statement per row. An empty array inserts
-- no rows. sqlc does not know the multi-argument unnest(a, b, c), so ROWS FROM (unnest(a), unnest(b), ...) says the
-- same thing in a form its analyzer accepts (S-3).
-- These queries run as the role of the new company, so row level security applies to them as to any other.

-- name: SeedTenantSettings :exec
INSERT INTO app.tenant_settings (tenant_id,
    project_label_singular, project_label_plural, project_label_gender,
    customer_label_singular, customer_label_plural, customer_label_gender,
    quote_label_singular, quote_label_plural, quote_label_gender,
    quote_validity_days, quote_terms, created_at, updated_at)
VALUES (@tenant_id,
    @project_label_singular, @project_label_plural, @project_label_gender,
    @customer_label_singular, @customer_label_plural, @customer_label_gender,
    @quote_label_singular, @quote_label_plural, @quote_label_gender,
    @quote_validity_days, sqlc.narg(quote_terms), @now, @now);

-- name: SeedStages :exec
-- The position of a stage is its place in the arrays, from 1.
INSERT INTO app.stages (tenant_id, name, type, color, position, created_at, updated_at)
SELECT @tenant_id::uuid, s.name, s.type, s.color, s.position::integer, @now::timestamptz, @now::timestamptz
FROM ROWS FROM (unnest(@names::text[]), unnest(@types::text[]), unnest(@colors::text[])) WITH ORDINALITY AS s (name, type, color, position);

-- name: SeedCategories :exec
-- An empty system key means "not a system category".
INSERT INTO app.categories (tenant_id, name, type, system_key, created_at, updated_at)
SELECT @tenant_id::uuid, c.name, c.type, NULLIF(c.system_key, ''), @now::timestamptz, @now::timestamptz
FROM ROWS FROM (unnest(@names::text[]), unnest(@types::text[]), unnest(@system_keys::text[])) AS c (name, type, system_key);

-- name: SeedCatalogItems :exec
-- An empty description or unit label means "none". Sample items never carry a reference price.
INSERT INTO app.catalog_items (tenant_id, name, description, unit, unit_label, created_at, updated_at)
SELECT @tenant_id::uuid, i.name, NULLIF(i.description, ''), i.unit, NULLIF(i.unit_label, ''), @now::timestamptz, @now::timestamptz
FROM ROWS FROM (unnest(@names::text[]), unnest(@descriptions::text[]), unnest(@units::text[]), unnest(@unit_labels::text[])) AS i (name, description, unit, unit_label);

-- Queries of the catalog of products and services (002/DD-9). Every one filters by the company (INV-04 of 001).

-- name: ListCatalogItems :many
-- By name ignoring case, then id (002/DD-16). Archived ones only with include_archived.
SELECT * FROM app.catalog_items
WHERE tenant_id = @tenant_id AND (@include_archived::boolean OR archived_at IS NULL)
ORDER BY lower(name), id;

-- name: GetCatalogItem :one
-- Other company or no row: pgx.ErrNoRows, which the use case turns into ErrNotFound. Archived items are returned.
SELECT * FROM app.catalog_items WHERE tenant_id = @tenant_id AND id = @id;

-- name: InsertCatalogItem :exec
INSERT INTO app.catalog_items (id, tenant_id, name, description, unit, unit_label, reference_price_cents, reference_price_currency, created_at, updated_at)
VALUES (@id, @tenant_id, @name, sqlc.narg(description), @unit, sqlc.narg(unit_label), sqlc.narg(reference_price_cents), sqlc.narg(reference_price_currency), @now, @now);

-- name: UpdateCatalogItemFields :exec
-- Writes only the groups whose set_* flag is true; the rest keeps its value. Unit and label go together because
-- the table ties them (catalog_items_unit_label_chk), and so do the amount and the currency.
UPDATE app.catalog_items SET
    name                     = CASE WHEN @set_name::boolean THEN @name::text ELSE name END,
    description              = CASE WHEN @set_description::boolean THEN sqlc.narg(description)::text ELSE description END,
    unit                     = CASE WHEN @set_unit::boolean THEN @unit::text ELSE unit END,
    unit_label               = CASE WHEN @set_unit_label::boolean THEN sqlc.narg(unit_label)::text ELSE unit_label END,
    reference_price_cents    = CASE WHEN @set_reference_price::boolean THEN sqlc.narg(reference_price_cents)::bigint ELSE reference_price_cents END,
    reference_price_currency = CASE WHEN @set_reference_price::boolean THEN sqlc.narg(reference_price_currency)::text ELSE reference_price_currency END,
    updated_at               = @now::timestamptz
WHERE tenant_id = @tenant_id AND id = @id;

-- name: SetCatalogItemArchived :exec
-- A NULL archived_at restores the item.
UPDATE app.catalog_items SET archived_at = sqlc.narg(archived_at)::timestamptz, updated_at = @now::timestamptz
WHERE tenant_id = @tenant_id AND id = @id;

-- name: InsertTenant :exec
INSERT INTO app.tenants (id, name, base_currency, timezone, industry_template_code, industry_template_version)
VALUES (@tenant_id, @name, @base_currency, @timezone, @industry_template_code, @industry_template_version);

-- name: GetTenantSummary :one
SELECT id, name, base_currency, timezone, logo_object_key
FROM app.tenants WHERE id = @tenant_id;

-- name: GetTenant :one
SELECT * FROM app.tenants WHERE id = @tenant_id;

-- name: LockTenant :one
-- DD-40: serialize partial edits and logo reference replacement without blocking FK checks.
SELECT * FROM app.tenants WHERE id = @tenant_id FOR NO KEY UPDATE;

-- name: UpdateTenantDetails :one
UPDATE app.tenants SET name = @name, legal_name = sqlc.narg(legal_name),
    tax_id = sqlc.narg(tax_id), address = sqlc.narg(address), phone = sqlc.narg(phone),
    email = sqlc.narg(email), timezone = @timezone, updated_at = @updated_at
WHERE id = @tenant_id RETURNING *;

-- name: UpdateTenantLogo :one
UPDATE app.tenants SET logo_object_key = sqlc.narg(logo_object_key),
    logo_content_type = sqlc.narg(logo_content_type), updated_at = @updated_at
WHERE id = @tenant_id RETURNING *;

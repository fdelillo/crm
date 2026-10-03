-- name: InsertTenant :exec
INSERT INTO app.tenants (id, name, base_currency, timezone, industry_template_code, industry_template_version)
VALUES (@tenant_id, @name, @base_currency, @timezone, @industry_template_code, @industry_template_version);

-- name: GetTenantSummary :one
SELECT id, name, base_currency, timezone, logo_object_key
FROM app.tenants WHERE id = @tenant_id;

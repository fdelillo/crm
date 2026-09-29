-- name: GetSpikeItem :one
SELECT id, tenant_id, email, ip, parent_id, created_at
FROM app.spike_items
WHERE tenant_id = @tenant_id AND id = @id;

-- name: InsertSpikeItem :one
INSERT INTO app.spike_items (tenant_id, email, secret_hash, ip, parent_id, created_at)
VALUES (@tenant_id, @email, @secret_hash, @ip, sqlc.narg('parent_id'), @created_at)
RETURNING id, tenant_id, email, ip, parent_id, created_at;

-- name: LookupSpikeItemByEmail :one
-- Phase 1 style lookup as crm_auth: routing columns only.
SELECT id, tenant_id FROM app.spike_items WHERE email = @email;

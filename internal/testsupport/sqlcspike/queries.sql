-- name: GetTenantByID :one
SELECT id, name, base_currency, timezone, logo_object_key, created_at, updated_at
FROM app.tenants
WHERE id = @tenant_id;

-- name: LookupUserByEmail :one
-- Phase 1 style routing lookup as crm_auth: routing columns only (plan §4.4).
SELECT id, tenant_id FROM app.users WHERE email = @email;

-- name: InsertAuditLog :one
INSERT INTO app.audit_log (tenant_id, actor_user_id, action, target_type, target_id, ip, user_agent, request_id)
VALUES (@tenant_id, sqlc.narg('actor_user_id'), @action, sqlc.narg('target_type'), sqlc.narg('target_id'),
        sqlc.narg('ip'), sqlc.narg('user_agent'), sqlc.narg('request_id'))
RETURNING id, occurred_at, ip;

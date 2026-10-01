-- name: InsertAudit :exec
INSERT INTO app.audit_log (
    tenant_id, actor_user_id, action, target_type, target_id, data, ip, user_agent, request_id
) VALUES (
    @tenant_id, sqlc.narg(actor_user_id), @action, sqlc.narg(target_type),
    sqlc.narg(target_id), @data, sqlc.narg(ip), sqlc.narg(user_agent), sqlc.narg(request_id)
);

-- name: OrTrue :many
SELECT id FROM app.users WHERE tenant_id = @tenant_id OR true;

-- name: OrOtherColumn :many
SELECT id FROM app.users WHERE id = @id OR tenant_id = @tenant_id;

-- name: OrInsideGroup :many
SELECT id FROM app.users WHERE (tenant_id = @tenant_id OR archived) AND id = @id;

-- name: CommaFrom :many
SELECT u.id FROM app.tenants t, app.users u WHERE t.id = @tenant_id;

-- name: InsertSelect :exec
INSERT INTO app.audit_log (tenant_id, action)
SELECT @tenant_id, 'user.copied' FROM app.users;

-- name: SubqueryWithoutFilter :one
SELECT id FROM app.tenants WHERE id = @tenant_id AND EXISTS (SELECT 1 FROM app.users WHERE email = @email);

-- name: UnionSecondBranch :many
SELECT id FROM app.users WHERE tenant_id = @tenant_id
UNION ALL
SELECT id FROM app.sessions;

-- name: CteWithoutFilter :many
WITH recent AS (SELECT id FROM app.sessions WHERE created_at > @since)
SELECT id FROM recent;

-- name: PredicateOnlyInSelectList :many
SELECT @tenant_id AS t, id FROM app.users;

-- name: OrInsideAnExtraGroup :many
SELECT id FROM app.users WHERE tenant_id = @tenant_id AND (status = 'active' OR status = 'invited');

-- name: WrappedPredicate :one
SELECT id FROM app.users WHERE (tenant_id = @tenant_id AND id = @id);

-- name: ReversedPredicate :one
SELECT id FROM app.users WHERE @tenant_id = tenant_id AND id = @id;

-- name: SqlcArg :one
SELECT id FROM app.users WHERE tenant_id = sqlc.arg(tenant_id) AND id = @id;

-- name: InsertSelectFiltered :exec
INSERT INTO app.audit_log (tenant_id, action)
SELECT @tenant_id, 'user.copied' FROM app.users WHERE tenant_id = @tenant_id AND id = @id;

-- name: JoinFiltered :many
SELECT u.id FROM app.users u JOIN app.sessions s ON s.user_id = u.id WHERE u.tenant_id = @tenant_id;

-- name: SubqueryFiltered :one
SELECT id FROM app.tenants WHERE id = @tenant_id AND EXISTS (SELECT 1 FROM app.users WHERE tenant_id = @tenant_id AND email = @email);

-- name: CteFiltered :many
WITH recent AS (SELECT id FROM app.sessions WHERE tenant_id = @tenant_id)
SELECT id FROM recent;

-- name: UnionFiltered :many
SELECT id FROM app.users WHERE tenant_id = @tenant_id
UNION ALL
SELECT id FROM app.sessions WHERE tenant_id = @tenant_id;

-- name: CommaFromFiltered :many
SELECT u.id FROM app.tenants t, app.users u WHERE u.tenant_id = @tenant_id AND t.id = u.tenant_id;

-- name: NonCompanyOr :many
SELECT 1 FROM app.login_throttles WHERE email_hmac = @a OR email_hmac = @b;

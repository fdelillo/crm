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

-- name: OrBeforeAnd :many
SELECT id FROM app.users WHERE email = @email OR status = 'active' AND tenant_id = @tenant_id;

-- name: DeleteOrTrueBeforeAnd :exec
DELETE FROM app.users WHERE id = @id OR true AND tenant_id = @tenant_id;

-- name: UpdateTrailingOr :exec
UPDATE app.users SET status = @status WHERE tenant_id = @tenant_id AND id = @id OR email = @email;

-- name: QuotedTable :many
SELECT id FROM "users" WHERE email = @email;

-- name: QuotedSchemaAndTable :many
SELECT id FROM "app"."users" WHERE email = @email;

-- name: QuotedJoin :many
SELECT t.id FROM app.tenants t JOIN app."sessions" s ON s.tenant_id = t.id WHERE t.id = @tenant_id;

-- name: QuotedCommaFrom :many
SELECT u.id FROM app.tenants t, app."users" u WHERE t.id = @tenant_id;

-- name: QuotedInsert :exec
INSERT INTO app."users" (email) VALUES (@email);

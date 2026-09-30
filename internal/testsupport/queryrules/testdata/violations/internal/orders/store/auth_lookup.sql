-- Not excepted when the exception list is empty.
-- name: UserByEmail :one
SELECT id, tenant_id FROM app.users WHERE email = @email;

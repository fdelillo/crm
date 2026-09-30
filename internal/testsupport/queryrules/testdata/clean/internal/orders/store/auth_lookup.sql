-- name: UserByEmail :one
SELECT id, tenant_id FROM app.users WHERE email = @email;

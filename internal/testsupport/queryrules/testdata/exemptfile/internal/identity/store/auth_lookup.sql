-- An exempt file: only the queries listed by name in the exception skip the company filter.

-- name: UserByEmail :one
SELECT id, tenant_id FROM app.users WHERE email = @email;

-- name: ListAllUsers :many
SELECT id, email FROM app.users;

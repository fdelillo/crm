-- name: Filtered :one
SELECT id FROM app.users WHERE tenant_id = @tenant_id AND id = @id;

-- name: LookupSessionByHash :one
SELECT id, tenant_id, token_hash FROM app.sessions WHERE token_hash = @token_hash;

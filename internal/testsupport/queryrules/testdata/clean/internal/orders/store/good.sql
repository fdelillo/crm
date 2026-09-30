-- name: GetUser :one
SELECT id, email FROM app.users WHERE tenant_id = @tenant_id AND id = @id;

-- name: ListSessions :many
SELECT s.id FROM app.sessions s WHERE s.tenant_id = @tenant_id ORDER BY s.created_at;

-- name: GetTenant :one
SELECT id, name FROM app.tenants WHERE id = @tenant_id;

-- name: EnqueueEmail :exec
INSERT INTO app.outbox_messages (tenant_id, kind, template, recipient) VALUES (@tenant_id, @kind, @template, @recipient);

-- name: ClosePeriod :exec
-- A comment mentioning FROM app.users must not count as a reference.
UPDATE app.user_tokens SET used_at = @now WHERE tenant_id = @tenant_id AND id = @id;

-- name: NoCompanyTable :one
SELECT 1 FROM app.login_throttles WHERE email_hmac = @email_hmac;

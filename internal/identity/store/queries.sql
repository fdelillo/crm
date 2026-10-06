-- name: InsertFirstAdmin :one
INSERT INTO app.users (tenant_id, email, name, password_hash, role, status)
VALUES (@tenant_id, @email, @name, @password_hash, 'admin', 'active')
RETURNING id;

-- name: InsertSession :one
INSERT INTO app.sessions (tenant_id, user_id, token_hash, created_at, last_seen_at, expires_at, ip, user_agent)
VALUES (@tenant_id, @user_id, @token_hash, @created_at, @last_seen_at, @expires_at, @ip, @user_agent)
RETURNING id;

-- name: InsertVerificationToken :exec
INSERT INTO app.user_tokens (tenant_id, user_id, purpose, token_hash, created_at, expires_at)
VALUES (@tenant_id, @user_id, 'email_verification', @token_hash, @created_at, @expires_at);

-- name: GetSessionUser :one
SELECT s.id AS session_id, s.user_id, s.revoked_at, s.created_at, s.last_seen_at, s.expires_at,
       u.email, u.name, u.role, u.status, u.email_verified_at
FROM app.sessions AS s
JOIN app.users AS u ON u.tenant_id = s.tenant_id AND u.id = s.user_id
WHERE s.tenant_id = @tenant_id AND s.id = @session_id;

-- name: TouchSession :exec
UPDATE app.sessions SET last_seen_at = @last_seen_at
WHERE tenant_id = @tenant_id AND id = @session_id;

-- name: GetUserEmail :one
SELECT email FROM app.users WHERE tenant_id = @tenant_id AND id = @user_id;

-- name: GetLoginUser :one
SELECT password_hash, status, role FROM app.users
WHERE tenant_id = @tenant_id AND id = @user_id;

-- name: UpdatePasswordHash :exec
UPDATE app.users SET password_hash = @password_hash, updated_at = @now
WHERE tenant_id = @tenant_id AND id = @user_id;

-- name: RevokeSession :one
UPDATE app.sessions SET revoked_at = @now, revoked_reason = 'logout'
WHERE tenant_id = @tenant_id AND id = @session_id AND revoked_at IS NULL AND expires_at > @now AND last_seen_at > @idle_cutoff
RETURNING user_id;

-- name: GetTokenFlowUser :one
SELECT email, status, role, email_verified_at FROM app.users
WHERE tenant_id = @tenant_id AND id = @user_id FOR NO KEY UPDATE;

-- name: GetTokenForUpdate :one
SELECT id, user_id, purpose, expires_at, used_at, revoked_at FROM app.user_tokens
WHERE tenant_id = @tenant_id AND id = @token_id FOR UPDATE;

-- name: GetTokenUser :one
SELECT user_id FROM app.user_tokens WHERE tenant_id = @tenant_id AND id = @token_id;

-- name: RevokeOpenUserTokens :exec
UPDATE app.user_tokens SET revoked_at = @now
WHERE tenant_id = @tenant_id AND user_id = @user_id AND purpose = @purpose
  AND used_at IS NULL AND revoked_at IS NULL;

-- name: InsertUserToken :exec
INSERT INTO app.user_tokens (tenant_id, user_id, purpose, token_hash, created_at, expires_at, created_by_user_id)
VALUES (@tenant_id, @user_id, @purpose, @token_hash, @created_at, @expires_at, sqlc.narg(created_by_user_id));

-- name: UseUserToken :execrows
UPDATE app.user_tokens SET used_at = @now
WHERE tenant_id = @tenant_id AND id = @token_id AND purpose = @purpose
  AND used_at IS NULL AND revoked_at IS NULL AND expires_at > @now;

-- name: SetResetPassword :exec
UPDATE app.users SET password_hash = @password_hash, updated_at = @now
WHERE tenant_id = @tenant_id AND id = @user_id;

-- name: RevokeUserSessions :execrows
UPDATE app.sessions SET revoked_at = @now, revoked_reason = 'password_reset'
WHERE tenant_id = @tenant_id AND user_id = @user_id AND revoked_at IS NULL;

-- name: MarkEmailVerified :execrows
UPDATE app.users SET email_verified_at = @now, updated_at = @now
WHERE tenant_id = @tenant_id AND id = @user_id AND email_verified_at IS NULL;

-- name: GetTokenFlowTenantName :one
SELECT name FROM app.tenants WHERE id = @tenant_id;

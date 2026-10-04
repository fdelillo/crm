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

-- name: LockUsersTenant :one
SELECT id FROM app.tenants WHERE id = @tenant_id FOR UPDATE;

-- name: GetManagedUserForUpdate :one
SELECT id, email, name, role, status, password_hash, email_verified_at, created_at
FROM app.users WHERE tenant_id = @tenant_id AND id = @user_id FOR UPDATE;

-- name: FindInvitedEmail :one
SELECT id, status FROM app.users WHERE tenant_id = @tenant_id AND email = @email;

-- name: InsertInvitedUser :one
INSERT INTO app.users (tenant_id, email, role, status, created_at, updated_at, status_changed_at)
VALUES (@tenant_id, @email, @role, 'invited', @now, @now, @now) RETURNING id;

-- name: GetManagedUser :one
SELECT u.id, u.email, u.name, u.role, u.status, u.email_verified_at, u.created_at,
       tok.expires_at AS invitation_expires_at
FROM app.users AS u
LEFT JOIN app.user_tokens AS tok ON tok.tenant_id = u.tenant_id AND tok.user_id = u.id
  AND u.status = 'invited' AND tok.purpose = 'invitation' AND tok.used_at IS NULL AND tok.revoked_at IS NULL
WHERE u.tenant_id = @tenant_id AND u.id = @user_id;

-- name: ListManagedUsers :many
SELECT u.id, u.email, u.name, u.role, u.status, u.email_verified_at, u.created_at,
       tok.expires_at AS invitation_expires_at
FROM app.users AS u
LEFT JOIN app.user_tokens AS tok ON tok.tenant_id = u.tenant_id AND tok.user_id = u.id
  AND u.status = 'invited' AND tok.purpose = 'invitation' AND tok.used_at IS NULL AND tok.revoked_at IS NULL
WHERE u.tenant_id = @tenant_id ORDER BY u.created_at, u.id;

-- name: CountActiveAdmins :one
SELECT count(*) FROM app.users WHERE tenant_id = @tenant_id AND status = 'active' AND role = 'admin';

-- name: SetManagedUserRole :exec
UPDATE app.users SET updated_at = @now, role = @role WHERE tenant_id = @tenant_id AND id = @user_id;

-- name: SetManagedUserStatus :exec
UPDATE app.users SET status = @status, updated_at = @now, status_changed_at = @now
WHERE tenant_id = @tenant_id AND id = @user_id;

-- name: ActivateInvitedUser :exec
UPDATE app.users SET status = 'active', name = @name, password_hash = @password_hash,
  email_verified_at = @now, status_changed_at = @now, updated_at = @now
WHERE tenant_id = @tenant_id AND id = @user_id;

-- name: RevokeDisabledUserSessions :execrows
UPDATE app.sessions SET revoked_at = @now, revoked_reason = 'user_disabled'
WHERE tenant_id = @tenant_id AND user_id = @user_id AND revoked_at IS NULL;

-- name: RevokeDisabledUserTokens :exec
UPDATE app.user_tokens SET revoked_at = @now
WHERE tenant_id = @tenant_id AND user_id = @user_id AND used_at IS NULL AND revoked_at IS NULL;

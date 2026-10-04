-- name: LookupSessionByHash :one
SELECT id, tenant_id, token_hash FROM app.sessions WHERE token_hash = @token_hash;

-- name: EnsureLoginThrottle :exec
INSERT INTO app.login_throttles (email_hmac, failed_count, first_failed_at, last_failed_at)
VALUES (@email_hmac, 0, @now, @now)
ON CONFLICT (email_hmac) DO NOTHING;

-- name: LockLoginThrottle :one
SELECT failed_count, first_failed_at, last_failed_at, locked_until
FROM app.login_throttles WHERE email_hmac = @email_hmac FOR UPDATE;

-- name: SaveLoginThrottle :exec
UPDATE app.login_throttles
SET failed_count = @failed_count, first_failed_at = @first_failed_at,
    last_failed_at = @last_failed_at, locked_until = @locked_until
WHERE email_hmac = @email_hmac;

-- name: DeleteLoginThrottle :exec
DELETE FROM app.login_throttles WHERE email_hmac = @email_hmac;

-- name: LookupUserByEmail :one
SELECT id, tenant_id FROM app.users WHERE email = @email;

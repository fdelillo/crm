-- name: DeleteExpiredSessions :execrows
DELETE FROM app.sessions
WHERE expires_at < now() - interval '30 days'
   OR revoked_at < now() - interval '30 days';

-- name: DeleteExpiredUserTokens :execrows
DELETE FROM app.user_tokens
WHERE expires_at < now() - interval '30 days'
  AND (purpose <> 'invitation' OR used_at IS NOT NULL OR revoked_at IS NOT NULL);

-- name: DeleteOldLoginThrottles :execrows
DELETE FROM app.login_throttles
WHERE last_failed_at < now() - interval '1 day'
  AND (locked_until IS NULL OR locked_until < now());

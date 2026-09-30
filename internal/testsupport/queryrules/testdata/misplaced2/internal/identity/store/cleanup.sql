-- name: DeleteExpired :exec
DELETE FROM app.sessions WHERE expires_at < @cutoff;

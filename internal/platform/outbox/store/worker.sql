-- name: LockDueMessage :one
SELECT id, tenant_id FROM app.outbox_messages
WHERE status = 'pending' AND next_attempt_at <= @now
ORDER BY next_attempt_at, id
FOR UPDATE SKIP LOCKED
LIMIT 1;

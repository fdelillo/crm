-- name: LockDueMessage :one
-- next_attempt_at and created_at are returned too: the Dispatcher needs them to defer the message
-- (ADR-024 §6) without a second read, and crm_worker already has SELECT on both columns.
SELECT id, tenant_id, next_attempt_at, created_at FROM app.outbox_messages
WHERE status = 'pending' AND next_attempt_at <= @now
ORDER BY next_attempt_at, id
FOR UPDATE SKIP LOCKED
LIMIT 1;

-- name: DeferMessage :execrows
-- Isolates a per-message failure of the Dispatcher (AsTenant, GetMessage or the mark) from the
-- rest of the batch (ADR-024 §6, INV-31): it moves only next_attempt_at, as crm_worker, with the
-- UPDATE (next_attempt_at) privilege and the worker_lock policy that already exist for
-- LockDueMessage (no new privilege). The condition on next_attempt_at is the optimistic check
-- against another worker having already claimed and resolved the same message: 0 rows affected
-- means that happened, and it is not an error. attempts and last_error are untouched: no delivery
-- was attempted.
UPDATE app.outbox_messages
SET next_attempt_at = @next_attempt_at
WHERE id = @id AND status = 'pending' AND next_attempt_at = @expected_next_attempt_at;

-- name: InsertMessage :exec
-- created_at and next_attempt_at are set explicitly by Enqueue (clock.Clock), not by the column
-- defaults: deferDelay (ADR-024 §6) needs a created_at that matches the caller's clock in tests,
-- and the two columns must agree with each other (M6).
INSERT INTO app.outbox_messages (tenant_id, kind, template, recipient, payload, created_at, next_attempt_at)
VALUES (@tenant_id, @kind, @template, @recipient, @payload, @created_at, @next_attempt_at);

-- name: GetMessage :one
SELECT id, tenant_id, kind, template, recipient, payload, attempts
FROM app.outbox_messages
WHERE tenant_id = @tenant_id AND id = @id AND status = 'pending';

-- name: MarkSent :exec
UPDATE app.outbox_messages
SET status = 'sent', payload = NULL, sent_at = @sent_at, last_error = NULL
WHERE tenant_id = @tenant_id AND id = @id AND status = 'pending';

-- name: MarkRecoverable :exec
UPDATE app.outbox_messages
SET attempts = attempts + 1, next_attempt_at = @next_attempt_at, last_error = @last_error
WHERE tenant_id = @tenant_id AND id = @id AND status = 'pending';

-- name: MarkFailed :exec
UPDATE app.outbox_messages
SET status = 'failed', payload = NULL, failed_at = @failed_at, attempts = attempts + 1, last_error = @last_error
WHERE tenant_id = @tenant_id AND id = @id AND status = 'pending';

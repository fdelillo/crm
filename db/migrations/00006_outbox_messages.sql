-- +goose Up
-- Outgoing emails, written in the transaction of the operation that caused them (data-model.md §2.5, ADR-010).
CREATE TABLE app.outbox_messages (
    id              uuid        NOT NULL DEFAULT uuidv7(),
    tenant_id       uuid        NOT NULL,
    kind            text        NOT NULL,
    template        text        NOT NULL,
    recipient       text        NOT NULL,
    payload         jsonb,
    status          text        NOT NULL DEFAULT 'pending',
    attempts        integer     NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    last_error      text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    sent_at         timestamptz,
    failed_at       timestamptz,
    CONSTRAINT outbox_messages_pkey PRIMARY KEY (id),
    CONSTRAINT outbox_messages_tenant_fk FOREIGN KEY (tenant_id) REFERENCES app.tenants (id),
    CONSTRAINT outbox_kind_chk CHECK (kind IN ('email')),
    CONSTRAINT outbox_template_chk CHECK (template IN ('email_verification', 'password_reset', 'invitation')),
    CONSTRAINT outbox_recipient_chk CHECK (char_length(recipient) <= 254),
    CONSTRAINT outbox_status_chk CHECK (status IN ('pending', 'sent', 'failed')),
    CONSTRAINT outbox_attempts_chk CHECK (attempts >= 0),
    CONSTRAINT outbox_last_error_chk CHECK (char_length(last_error) <= 1000),
    -- The token travels in the payload in clear text: it exists only while the message is pending (INV-09).
    CONSTRAINT outbox_scrub_chk CHECK ((status = 'pending') = (payload IS NOT NULL)),
    CONSTRAINT outbox_terminal_chk CHECK ((status = 'sent') = (sent_at IS NOT NULL) AND (status = 'failed') = (failed_at IS NOT NULL))
);

CREATE INDEX outbox_pending_idx ON app.outbox_messages (next_attempt_at) WHERE status = 'pending';
CREATE INDEX outbox_created_idx ON app.outbox_messages (created_at);

ALTER TABLE app.outbox_messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.outbox_messages FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON app.outbox_messages
    FOR ALL TO PUBLIC
    USING (tenant_id = (SELECT app.current_tenant_id()))
    WITH CHECK (tenant_id = (SELECT app.current_tenant_id()));

GRANT UPDATE ON app.outbox_messages TO crm_tenant;

-- The worker sees only queue columns. SELECT ... FOR UPDATE SKIP LOCKED needs an UPDATE privilege on at
-- least one column, and the FOR UPDATE policy; it does not update anything as crm_worker: it marks the
-- message as sent already as the company role (data-model.md §3.4).
GRANT SELECT (id, tenant_id, status, next_attempt_at, created_at), UPDATE (next_attempt_at), DELETE
    ON app.outbox_messages TO crm_worker;
CREATE POLICY worker_read ON app.outbox_messages FOR SELECT TO crm_worker USING (true);
CREATE POLICY worker_lock ON app.outbox_messages FOR UPDATE TO crm_worker
    USING (status = 'pending') WITH CHECK (status = 'pending');
CREATE POLICY worker_cleanup ON app.outbox_messages FOR DELETE TO crm_worker
    USING (status <> 'pending' AND created_at < now() - interval '30 days');

-- +goose Down
DROP TABLE app.outbox_messages;

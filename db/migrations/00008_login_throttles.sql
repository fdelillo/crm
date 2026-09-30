-- +goose Up
-- Failed-login counters by HMAC of the email, whether or not the account exists (data-model.md §2.7, DD-7).
-- It has no tenant_id on purpose (Constitution Check, principle III): it is not company data. Only
-- crm_auth uses it; the company roles cannot touch it.
CREATE TABLE app.login_throttles (
    email_hmac      bytea       NOT NULL,
    failed_count    integer     NOT NULL,
    first_failed_at timestamptz NOT NULL,
    last_failed_at  timestamptz NOT NULL,
    locked_until    timestamptz,
    CONSTRAINT login_throttles_pkey PRIMARY KEY (email_hmac),
    CONSTRAINT login_throttles_hmac_chk CHECK (octet_length(email_hmac) = 32),
    CONSTRAINT login_throttles_count_chk CHECK (failed_count >= 0)
);

ALTER TABLE app.login_throttles ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.login_throttles FORCE ROW LEVEL SECURITY;

-- The default privileges of 00001 gave SELECT and INSERT to crm_tenant; this table is not theirs.
REVOKE ALL ON app.login_throttles FROM crm_tenant;

GRANT SELECT, INSERT, UPDATE, DELETE ON app.login_throttles TO crm_auth;
CREATE POLICY auth_all ON app.login_throttles FOR ALL TO crm_auth USING (true) WITH CHECK (true);

GRANT SELECT (email_hmac, last_failed_at, locked_until), DELETE ON app.login_throttles TO crm_worker;
CREATE POLICY worker_read ON app.login_throttles FOR SELECT TO crm_worker USING (true);
CREATE POLICY worker_cleanup ON app.login_throttles FOR DELETE TO crm_worker
    USING (last_failed_at < now() - interval '1 day' AND (locked_until IS NULL OR locked_until < now()));

-- +goose Down
DROP TABLE app.login_throttles;

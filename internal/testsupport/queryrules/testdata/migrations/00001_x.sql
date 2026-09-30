-- +goose Up
CREATE TABLE app.tenants (
    id uuid NOT NULL,
    name text NOT NULL
);

CREATE TABLE app.users (
    id        uuid NOT NULL,
    tenant_id uuid NOT NULL,
    email     text NOT NULL
);

CREATE TABLE app.sessions (
    id        uuid NOT NULL,
    tenant_id uuid NOT NULL
);

CREATE TABLE app.user_tokens (
    id        uuid NOT NULL,
    tenant_id uuid NOT NULL,
    used_at   timestamptz
);

CREATE TABLE app.outbox_messages (
    id        uuid NOT NULL,
    tenant_id uuid NOT NULL
);

CREATE TABLE app.login_throttles (
    email_hmac bytea NOT NULL
);

-- +goose Down
DROP TABLE app.login_throttles;
CREATE TABLE app.not_in_up (
    tenant_id uuid NOT NULL
);

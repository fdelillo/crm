-- +goose Up
-- crm_owner owns the database and therefore public through pg_database_owner (PostgreSQL 18).
-- Readiness needs only the migration version, never the whole version table (INV-08).
GRANT USAGE ON SCHEMA public TO crm_auth;
GRANT SELECT (version_id) ON public.goose_db_version TO crm_auth;

-- +goose Down
REVOKE SELECT (version_id) ON public.goose_db_version FROM crm_auth;
REVOKE USAGE ON SCHEMA public FROM crm_auth;

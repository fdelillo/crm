-- +goose Up
-- Schemas, default privileges and schema access (data-model.md §3.4, §3.5, ADR-004, ADR-005).
-- Runs as crm_owner. The roles and the database come from db/bootstrap/.

CREATE SCHEMA app;
-- crm_owner is a member of crm_provisioner (SET TRUE) precisely so it can create this schema for it.
CREATE SCHEMA provisioning AUTHORIZATION crm_provisioner;

-- The goose version table lives in public; no runtime role gets anything there.
REVOKE ALL ON SCHEMA public FROM PUBLIC;

-- Every table crm_owner creates in app is readable and insertable by the company roles (through
-- the crm_tenant group). UPDATE and DELETE are granted explicitly by each table's migration, so an
-- immutable table never receives them by accident.
ALTER DEFAULT PRIVILEGES FOR ROLE crm_owner IN SCHEMA app GRANT SELECT, INSERT ON TABLES TO crm_tenant;
-- Functions are not executable by PUBLIC unless a migration says so.
ALTER DEFAULT PRIVILEGES FOR ROLE crm_owner REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC;

GRANT USAGE ON SCHEMA app TO crm_tenant, crm_auth, crm_worker;

-- crm_owner can SET ROLE to crm_provisioner but does not inherit its privileges (INHERIT FALSE),
-- so it cannot grant on, or drop, a schema that crm_provisioner owns without switching to it.
SET LOCAL ROLE crm_provisioner;
GRANT USAGE ON SCHEMA provisioning TO crm_signup;
RESET ROLE;

-- +goose Down
SET LOCAL ROLE crm_provisioner;
REVOKE USAGE ON SCHEMA provisioning FROM crm_signup;
DROP SCHEMA provisioning;
RESET ROLE;
REVOKE USAGE ON SCHEMA app FROM crm_tenant, crm_auth, crm_worker;
ALTER DEFAULT PRIVILEGES FOR ROLE crm_owner GRANT EXECUTE ON FUNCTIONS TO PUBLIC;
ALTER DEFAULT PRIVILEGES FOR ROLE crm_owner IN SCHEMA app REVOKE SELECT, INSERT ON TABLES FROM crm_tenant;
GRANT USAGE ON SCHEMA public TO PUBLIC;
DROP SCHEMA app;

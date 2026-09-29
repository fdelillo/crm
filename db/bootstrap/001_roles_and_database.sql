-- Cluster bootstrap for the CRM (ADR-004, ADR-005, data-model.md §3.1).
--
-- Roles are global to the PostgreSQL cluster, and creating them needs CREATEROLE, so this script
-- lives outside goose. Run it ONCE PER CLUSTER as a superuser (or as a role with CREATEROLE and
-- CREATEDB that can also administer the roles below), connected to the `postgres` database:
--
--   psql -v ON_ERROR_STOP=1 -U postgres -d postgres -f db/bootstrap/001_roles_and_database.sql
--
-- It is idempotent: running it again converges the roles, memberships and settings to the state
-- described here. It needs psql (it uses \getenv and \gexec), PostgreSQL >= 16 (per-grant
-- INHERIT/SET options) and psql >= 15.
--
-- Passwords are NOT part of this file. To set them, export CRM_OWNER_PASSWORD and/or
-- CRM_APP_PASSWORD before running it; otherwise set them by hand with ALTER ROLE.
-- One cluster per environment (plan §12.4): the crm_t_<uuid> roles are cluster-wide.

\set ON_ERROR_STOP on

-- ---------------------------------------------------------------------------------------------
-- Roles (created if missing; attributes always re-applied)
-- ---------------------------------------------------------------------------------------------
SELECT format('CREATE ROLE %I', r) AS stmt
FROM unnest(ARRAY['crm_provisioner', 'crm_tenant', 'crm_auth', 'crm_worker', 'crm_signup',
                  'crm_owner', 'crm_app']) AS r
WHERE NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = r)
\gexec

-- Group and system roles: no login.
ALTER ROLE crm_tenant      NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
ALTER ROLE crm_auth        NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
ALTER ROLE crm_worker      NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
ALTER ROLE crm_signup      NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
-- The only role with CREATEROLE; it owns the SECURITY DEFINER provisioning function (INV-08, §10.4).
ALTER ROLE crm_provisioner NOLOGIN NOSUPERUSER NOCREATEDB CREATEROLE NOREPLICATION NOBYPASSRLS;

-- Login roles. crm_owner runs migrations only; crm_app is the runtime pool role.
ALTER ROLE crm_owner LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
ALTER ROLE crm_app   LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;

ALTER ROLE crm_app SET statement_timeout = '5s';
ALTER ROLE crm_app SET idle_in_transaction_session_timeout = '30s';

-- Optional passwords, taken from the environment (never stored in this repository).
\getenv crm_owner_password CRM_OWNER_PASSWORD
\if :{?crm_owner_password}
  ALTER ROLE crm_owner PASSWORD :'crm_owner_password';
\endif
\getenv crm_app_password CRM_APP_PASSWORD
\if :{?crm_app_password}
  ALTER ROLE crm_app PASSWORD :'crm_app_password';
\endif

-- ---------------------------------------------------------------------------------------------
-- Memberships (PostgreSQL >= 16: options per grant)
-- ---------------------------------------------------------------------------------------------
-- crm_owner can become crm_provisioner so a migration creates the provisioning function with it,
-- but it does not inherit CREATEROLE.
GRANT crm_provisioner TO crm_owner WITH INHERIT FALSE, SET TRUE;
-- crm_app can become a system role or a company role with SET ROLE, but INHERIT FALSE leaves it
-- with no privileges of its own (INV-02: a query outside a transaction helper fails closed).
GRANT crm_auth   TO crm_app WITH INHERIT FALSE, SET TRUE;
GRANT crm_worker TO crm_app WITH INHERIT FALSE, SET TRUE;
GRANT crm_signup TO crm_app WITH INHERIT FALSE, SET TRUE;
-- crm_provisioner administers crm_tenant so the function can make each new crm_t_<hex> a member
-- of it (INHERIT TRUE, SET FALSE) without being able to use its privileges.
GRANT crm_tenant TO crm_provisioner WITH ADMIN TRUE, INHERIT FALSE, SET FALSE;

-- ---------------------------------------------------------------------------------------------
-- Database
-- ---------------------------------------------------------------------------------------------
SELECT 'CREATE DATABASE crm OWNER crm_owner'
WHERE NOT EXISTS (SELECT 1 FROM pg_catalog.pg_database WHERE datname = 'crm')
\gexec

ALTER DATABASE crm OWNER TO crm_owner;
REVOKE ALL ON DATABASE crm FROM PUBLIC;
GRANT CONNECT ON DATABASE crm TO crm_app;

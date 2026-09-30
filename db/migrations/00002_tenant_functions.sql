-- +goose Up
-- The two functions the isolation design rests on (data-model.md §3.2, §3.3, ADR-005).

-- app.current_tenant_id(): the company encoded in the current role, or NULL. Every RLS policy compares
-- tenant_id with it, so it is the ONLY place that knows how a role maps to a company: changing the
-- isolation strategy (ADR-005, alternative B) means changing this function and TxRunner, nothing else.
-- SECURITY INVOKER: it reads current_user, which is the role of the statement being checked. It has no
-- SET search_path clause (that would stop PostgreSQL from inlining it into the policies), so the body
-- names pg_catalog explicitly for every operator, function and type: the caller's search_path cannot
-- change what it answers.
-- +goose StatementBegin
CREATE FUNCTION app.current_tenant_id() RETURNS uuid
LANGUAGE sql STABLE PARALLEL SAFE AS $$
  SELECT CASE
    WHEN current_user::pg_catalog.text OPERATOR(pg_catalog.~) '^crm_t_[0-9a-f]{32}$'
    THEN pg_catalog.substr(current_user::pg_catalog.text, 7)::pg_catalog.uuid
  END
$$;
-- +goose StatementEnd

-- Default privileges revoke EXECUTE from PUBLIC (migration 00001); this one is meant for everybody:
-- the function reveals nothing and every role's policies evaluate it.
GRANT EXECUTE ON FUNCTION app.current_tenant_id() TO PUBLIC;

-- provisioning.provision_tenant_role(uuid): creates the role of a company. It is created while
-- acting as crm_provisioner (the only role with CREATEROLE), so that role owns it and, being
-- SECURITY DEFINER, is the one that runs it. crm_app cannot create roles by itself (plan §10.4).
SET LOCAL ROLE crm_provisioner;

-- +goose StatementBegin
CREATE FUNCTION provisioning.provision_tenant_role(p_tenant_id uuid) RETURNS text
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE
  v_role text := 'crm_t_' || replace(p_tenant_id::text, '-', '');
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = v_role) THEN
    EXECUTE format('CREATE ROLE %I NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS', v_role);
  END IF;
  -- The company role gets the group's privileges (INHERIT) but cannot become the group (no SET).
  EXECUTE format('GRANT crm_tenant TO %I WITH INHERIT TRUE, SET FALSE', v_role);
  -- The runtime role can become the company role (SET) without inheriting anything from it (INV-02).
  EXECUTE format('GRANT %I TO crm_app WITH INHERIT FALSE, SET TRUE', v_role);
  RETURN v_role;
END
$$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION provisioning.provision_tenant_role(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION provisioning.provision_tenant_role(uuid) TO crm_signup;
RESET ROLE;

-- +goose Down
SET LOCAL ROLE crm_provisioner;
DROP FUNCTION provisioning.provision_tenant_role(uuid);
RESET ROLE;
DROP FUNCTION app.current_tenant_id();

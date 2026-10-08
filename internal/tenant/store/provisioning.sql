-- name: SetSignupLockTimeout :one
SELECT set_config('lock_timeout', @timeout::text, true);

-- name: ProvisionTenantRole :one
SELECT provisioning.provision_tenant_role(@tenant_id::uuid);

-- name: CountTenants :one
SELECT count(*) FROM app.tenants;

-- name: CountTenantsWithoutRole :one
SELECT count(*) FROM app.tenants t
WHERE NOT EXISTS (
    SELECT 1 FROM pg_catalog.pg_roles r
    WHERE r.rolname = 'crm_t_' || replace(t.id::text, '-', '')
);

-- name: CountTenantRoles :one
SELECT count(*) FROM pg_catalog.pg_roles
WHERE rolname ~ '^crm_t_[0-9a-f]{32}$';

-- name: SetSignupLockTimeout :one
SELECT set_config('lock_timeout', @timeout::text, true);

-- name: ProvisionTenantRole :one
SELECT provisioning.provision_tenant_role(@tenant_id::uuid);

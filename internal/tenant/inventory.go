package tenant

import (
	"context"
	"expvar"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/tenant/store"
	"sync/atomic"
	"time"
)

type roleSample struct{ tenants, roles, missing int64 }

// RoleInventory samples table and cluster catalog counts in one worker transaction.
type RoleInventory struct {
	runner db.TxRunner
	sample atomic.Pointer[roleSample]
}

var publishedInventory atomic.Pointer[RoleInventory]

func init() {
	expvar.Publish("signup_email_exists_total", expvar.Func(func() any { return SignupEmailExistsCount() }))
	expvar.Publish("signup_lock_timeout_total", expvar.Func(func() any { return SignupLockTimeoutCount() }))
	for _, name := range []string{"tenants_total", "tenant_roles_total", "tenant_roles_missing"} {
		expvar.Publish(name, expvar.Func(func() any {
			inventory := publishedInventory.Load()
			if inventory == nil {
				return int64(-1)
			}
			sample := inventory.sample.Load()
			if sample == nil {
				return int64(-1)
			}
			switch name {
			case "tenants_total":
				return sample.tenants
			case "tenant_roles_total":
				return sample.roles
			default:
				return sample.missing
			}
		}))
	}
}
func NewRoleInventory(runner db.TxRunner) *RoleInventory {
	task := &RoleInventory{runner: runner}
	publishedInventory.Store(task)
	return task
}
func (*RoleInventory) Name() string         { return "tenant_role_inventory" }
func (*RoleInventory) Every() time.Duration { return time.Minute }
func (i *RoleInventory) Run(ctx context.Context) error {
	var sample roleSample
	err := i.runner.InSystemTx(ctx, db.RoleWorker, func(ctx context.Context, tx db.Tx) error {
		q := store.New(tx)
		var err error
		if sample.tenants, err = q.CountTenants(ctx); err != nil {
			return db.MapError(err)
		}
		if sample.roles, err = q.CountTenantRoles(ctx); err != nil {
			return db.MapError(err)
		}
		sample.missing, err = q.CountTenantsWithoutRole(ctx)
		return db.MapError(err)
	})
	if err != nil {
		i.sample.Store(nil)
		return err
	}
	i.sample.Store(&sample)
	return nil
}

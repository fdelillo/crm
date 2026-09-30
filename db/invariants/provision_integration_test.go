//go:build integration

package invariants_test

import (
	"context"
	"strings"
	"testing"

	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// T-B101: app.current_tenant_id() maps the current role to a company (data-model.md §3.2).
func TestProvision_CurrentTenantID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	app := pgtest.AppPool(t)
	id := uuid.New()
	role := fixture.ProvisionRole(t, app, id)

	// Simple protocol on purpose: with the extended protocol pgx caches the prepared statement per
	// connection, and a cached plan is not re-checked for USAGE on the schema, so whether a role
	// without that USAGE got an error would depend on which connection ran the test before.
	current := func(t *testing.T, role string) (*uuid.UUID, error) {
		t.Helper()
		var got *uuid.UUID
		var err error
		asRole(t, app, role, func(ctx context.Context, tx pgx.Tx) {
			err = tx.QueryRow(ctx, `SELECT app.current_tenant_id()`, pgx.QueryExecModeSimpleProtocol).Scan(&got)
		})
		return got, err
	}

	t.Run("company role", func(t *testing.T) {
		if got, err := current(t, role); err != nil || got == nil || *got != id {
			t.Errorf("current_tenant_id() as %s = %v (err %v), want %v", role, got, err, id)
		}
	})
	for _, sys := range []string{"crm_auth", "crm_worker"} {
		t.Run(sys, func(t *testing.T) {
			if got, err := current(t, sys); err != nil || got != nil {
				t.Errorf("current_tenant_id() as %s = %v (err %v), want NULL", sys, got, err)
			}
		})
	}
	// crm_signup has no USAGE on schema app (data-model.md §3.5): it can only call the provisioning function.
	t.Run("crm_signup has no access to schema app", func(t *testing.T) {
		if _, err := current(t, "crm_signup"); sqlState(err) != "42501" {
			t.Errorf("current_tenant_id() as crm_signup: err = %v, want SQLSTATE 42501", err)
		}
	})
	t.Run("crm_owner", func(t *testing.T) {
		var got *uuid.UUID
		if err := pgtest.OwnerPool(t).QueryRow(ctx, `SELECT app.current_tenant_id()`).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Errorf("current_tenant_id() as crm_owner = %v, want NULL", got)
		}
	})

	// Roles that merely look like a company role never map to one.
	hex := strings.ReplaceAll(uuid.NewString(), "-", "")
	lookalikes := map[string]string{
		"not hex":         "crm_t_xyz",
		"31 hex digits":   "crm_t_" + hex[:31],
		"33 hex digits":   "crm_t_" + hex + "0",
		"uppercase hex":   "crm_t_" + strings.ToUpper(hex),
		"other prefix":    "crm_x_" + hex,
		"prefix and hex+": "crm_t_" + hex + "_extra",
	}
	super := pgtest.SuperuserPool(t)
	for name, lookalike := range lookalikes {
		t.Run("lookalike "+name, func(t *testing.T) {
			quoted := pgx.Identifier{lookalike}.Sanitize()
			if _, err := super.Exec(ctx, `CREATE ROLE `+quoted+` NOLOGIN`); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _, _ = super.Exec(ctx, `DROP ROLE `+quoted) })
			// A member of the company group, like a real company role, so it can reach schema app.
			for _, grant := range []string{
				`GRANT crm_tenant TO ` + quoted + ` WITH INHERIT TRUE, SET FALSE`,
				`GRANT ` + quoted + ` TO crm_app WITH INHERIT FALSE, SET TRUE`,
			} {
				if _, err := super.Exec(ctx, grant); err != nil {
					t.Fatal(err)
				}
			}
			if got, err := current(t, lookalike); err != nil || got != nil {
				t.Errorf("current_tenant_id() as %q = %v (err %v), want NULL", lookalike, got, err)
			}
		})
	}
}

// T-B101: provisioning.provision_tenant_role (data-model.md §3.3, plan §10.4).
func TestProvision_CreatesTheCompanyRole(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	app := pgtest.AppPool(t)
	super := pgtest.SuperuserPool(t)
	id := uuid.New()
	want := db.TenantRoleName(id)

	if got := fixture.ProvisionRole(t, app, id); got != want {
		t.Fatalf("provision_tenant_role returned %q, want %q", got, want)
	}

	var login, superuser, createDB, createRole, replication, bypass bool
	err := super.QueryRow(ctx, `
		SELECT rolcanlogin, rolsuper, rolcreatedb, rolcreaterole, rolreplication, rolbypassrls
		FROM pg_roles WHERE rolname = $1`, want).Scan(&login, &superuser, &createDB, &createRole, &replication, &bypass)
	if err != nil {
		t.Fatalf("the role does not exist: %v", err)
	}
	if login || superuser || createDB || createRole || replication || bypass {
		t.Errorf("attributes login=%v super=%v createdb=%v createrole=%v replication=%v bypassrls=%v, want all false",
			login, superuser, createDB, createRole, replication, bypass)
	}

	var inherit, set bool
	err = super.QueryRow(ctx, `SELECT inherit_option, set_option FROM pg_auth_members
		WHERE roleid = 'crm_tenant'::regrole AND member = $1::regrole`, pgx.Identifier{want}.Sanitize()).Scan(&inherit, &set)
	if err != nil {
		t.Fatalf("membership in crm_tenant: %v", err)
	}
	if !inherit || set {
		t.Errorf("in crm_tenant: inherit = %v set = %v, want true/false", inherit, set)
	}
	err = super.QueryRow(ctx, `SELECT inherit_option, set_option FROM pg_auth_members
		WHERE roleid = $1::regrole AND member = 'crm_app'::regrole`, pgx.Identifier{want}.Sanitize()).Scan(&inherit, &set)
	if err != nil {
		t.Fatalf("crm_app membership: %v", err)
	}
	if inherit || !set {
		t.Errorf("crm_app in the company role: inherit = %v set = %v, want false/true", inherit, set)
	}
}

func TestProvision_IsIdempotent(t *testing.T) {
	t.Parallel()
	app := pgtest.AppPool(t)
	id := uuid.New()
	first := fixture.ProvisionRole(t, app, id)
	second := fixture.ProvisionRole(t, app, id)
	if first != second {
		t.Errorf("second call returned %q, want %q", second, first)
	}
	var n int
	err := pgtest.SuperuserPool(t).QueryRow(context.Background(),
		`SELECT count(*) FROM pg_auth_members WHERE roleid = $1::regrole AND member = 'crm_app'::regrole`,
		pgx.Identifier{first}.Sanitize()).Scan(&n)
	if err != nil || n != 1 {
		t.Errorf("crm_app memberships in the role = %d (err %v), want exactly 1", n, err)
	}
}

// CREATE ROLE is transactional: a registration that rolls back leaves no role behind (INV-14).
func TestProvision_RollbackLeavesNoRole(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	app := pgtest.AppPool(t)
	id := uuid.New()

	tx, err := app.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE crm_signup`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT provisioning.provision_tenant_role($1)`, id); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var exists bool
	err = pgtest.SuperuserPool(t).QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)`, db.TenantRoleName(id)).Scan(&exists)
	if err != nil || exists {
		t.Errorf("role exists after ROLLBACK = %v (err %v), want false", exists, err)
	}
}

// Only crm_signup can execute the function, and nobody in the runtime can create roles directly.
func TestProvision_IsNotCallableByOthers(t *testing.T) {
	t.Parallel()
	app := pgtest.AppPool(t)
	call := `SELECT provisioning.provision_tenant_role('00000000-0000-7000-8000-000000000001')`
	tests := []struct {
		name, role, sql string
	}{
		{"crm_app without SET ROLE", "", call},
		{"crm_auth", "crm_auth", call},
		{"crm_worker", "crm_worker", call},
		{"CREATE ROLE as crm_app", "", `CREATE ROLE crm_intruder`},
		{"CREATE ROLE as crm_signup", "crm_signup", `CREATE ROLE crm_intruder`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			tx, err := app.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if tt.role != "" {
				if _, err := tx.Exec(ctx, `SET LOCAL ROLE `+tt.role); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := tx.Exec(ctx, tt.sql); sqlState(err) != "42501" {
				t.Errorf("err = %v, want SQLSTATE 42501", err)
			}
		})
	}
}

// current_tenant_id() runs with the search_path of whoever calls it (it is SECURITY INVOKER without a
// SET clause, so it can be inlined into the policies). Its body must therefore name pg_catalog for every
// operator, function and type it uses: a caller who puts a schema of their own first in the path
// must not be able to make it answer with a company.
func TestProvision_CurrentTenantIDIgnoresTheCallersSearchPath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	super := pgtest.SuperuserPool(t)
	schema := "evil_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	for _, stmt := range []string{
		`CREATE SCHEMA ` + schema,
		`GRANT USAGE ON SCHEMA ` + schema + ` TO PUBLIC`,
		`CREATE FUNCTION ` + schema + `.always_true(text, text) RETURNS boolean LANGUAGE sql AS 'SELECT true'`,
		`CREATE OPERATOR ` + schema + `.~ (LEFTARG = text, RIGHTARG = text, FUNCTION = ` + schema + `.always_true)`,
		`CREATE FUNCTION ` + schema + `.substr(text, integer) RETURNS text LANGUAGE sql AS $$ SELECT '11111111111111111111111111111111' $$`,
		`GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA ` + schema + ` TO PUBLIC`,
	} {
		if _, err := super.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	t.Cleanup(func() { _, _ = super.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`) })

	asRole(t, pgtest.AppPool(t), "crm_auth", func(ctx context.Context, tx pgx.Tx) {
		if _, err := tx.Exec(ctx, `SET LOCAL search_path = `+schema+`, pg_catalog`); err != nil {
			t.Fatal(err)
		}
		var got *uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT app.current_tenant_id()`, pgx.QueryExecModeSimpleProtocol).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Errorf("current_tenant_id() as crm_auth with a hostile search_path = %v, want NULL", got)
		}
	})
}

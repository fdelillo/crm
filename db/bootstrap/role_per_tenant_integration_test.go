//go:build integration

// This spike checks, against a real PostgreSQL 18, the assumption ADR-005 rests on: a role created
// and granted to crm_app inside a transaction can be used with SET LOCAL ROLE in that same
// transaction, with no intermediate COMMIT, and vanishes on ROLLBACK (INV-14). It uses a helper
// function shaped like provisioning.provision_tenant_role (data-model.md §3.3, Phase 1) because
// the real one does not exist yet.
package bootstrap_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

const spikeSetup = `
CREATE SCHEMA spike_provisioning AUTHORIZATION crm_provisioner;

CREATE FUNCTION spike_provisioning.create_role(p_name text) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $fn$
BEGIN
  EXECUTE format('CREATE ROLE %I NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS', p_name);
  EXECUTE format('GRANT crm_tenant TO %I WITH INHERIT TRUE, SET FALSE', p_name);
  EXECUTE format('GRANT %I TO crm_app WITH INHERIT FALSE, SET TRUE', p_name);
END
$fn$;
ALTER FUNCTION spike_provisioning.create_role(text) OWNER TO crm_provisioner;

-- Same shape, but asking for BYPASSRLS: crm_provisioner has CREATEROLE and not BYPASSRLS, so
-- PostgreSQL >= 16 must refuse it (plan §10.4).
CREATE FUNCTION spike_provisioning.create_bypass_role(p_name text) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $fn$
BEGIN
  EXECUTE format('CREATE ROLE %I NOLOGIN BYPASSRLS', p_name);
END
$fn$;
ALTER FUNCTION spike_provisioning.create_bypass_role(text) OWNER TO crm_provisioner;

REVOKE ALL ON FUNCTION spike_provisioning.create_role(text), spike_provisioning.create_bypass_role(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION spike_provisioning.create_role(text), spike_provisioning.create_bypass_role(text) TO crm_signup;
GRANT USAGE ON SCHEMA spike_provisioning TO crm_signup;
`

var setupOnce sync.Once

func setup(t *testing.T) {
	t.Helper()
	pool := pgtest.SuperuserPool(t) // fixture only: it needs to create objects owned by crm_provisioner
	var err error
	setupOnce.Do(func() { _, err = pool.Exec(context.Background(), spikeSetup) })
	if err != nil {
		t.Fatalf("spike setup: %v", err)
	}
}

func newRoleName() string { return "crm_t_" + strings.ReplaceAll(uuid.NewString(), "-", "") }

func roleExists(t *testing.T, name string) bool {
	t.Helper()
	var exists bool
	err := pgtest.SuperuserPool(t).QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)`, name).Scan(&exists)
	if err != nil {
		t.Fatal(err)
	}
	return exists
}

func sqlState(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// T-B012, case 1: create, grant and SET LOCAL ROLE in one transaction, without a COMMIT in between.
func TestSpike_CreateGrantAndSetRoleInOneTransaction(t *testing.T) {
	t.Parallel()
	setup(t)
	ctx := context.Background()
	role := newRoleName()

	tx, err := pgtest.AppPool(t).Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	for _, stmt := range []string{`SET LOCAL ROLE crm_signup`} {
		if _, err := tx.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if _, err := tx.Exec(ctx, `SELECT spike_provisioning.create_role($1)`, role); err != nil {
		t.Fatalf("creating the company role as crm_signup: %v", err)
	}
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE `+pgx.Identifier{role}.Sanitize()); err != nil {
		t.Fatalf("SET LOCAL ROLE %s in the same transaction: %v", role, err)
	}
	var current, session string
	if err := tx.QueryRow(ctx, `SELECT current_user, session_user`).Scan(&current, &session); err != nil {
		t.Fatal(err)
	}
	if current != role || session != "crm_app" {
		t.Errorf("current_user = %q, session_user = %q; want %q and crm_app", current, session, role)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if !roleExists(t, role) {
		t.Error("the role does not exist after COMMIT")
	}
	// SET LOCAL is undone at COMMIT: the connection is back to crm_app with no privileges (INV-02).
	var after string
	if err := pgtest.AppPool(t).QueryRow(ctx, `SELECT current_user`).Scan(&after); err != nil || after != "crm_app" {
		t.Errorf("after COMMIT current_user = %q (err %v), want crm_app", after, err)
	}
}

// T-B012, case 2: the same transaction with ROLLBACK leaves no role behind (CREATE ROLE is transactional).
func TestSpike_RollbackLeavesNoRole(t *testing.T) {
	t.Parallel()
	setup(t)
	ctx := context.Background()
	role := newRoleName()

	tx, err := pgtest.AppPool(t).Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`SET LOCAL ROLE crm_signup`, nil},
		{`SELECT spike_provisioning.create_role($1)`, []any{role}},
		{`SET LOCAL ROLE ` + pgx.Identifier{role}.Sanitize(), nil},
	} {
		if _, err := tx.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatalf("%s: %v", q.sql, err)
		}
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if roleExists(t, role) {
		t.Error("the role still exists after ROLLBACK")
	}
	// And crm_app is not a member of anything named like it.
	var n int
	err = pgtest.SuperuserPool(t).QueryRow(ctx,
		`SELECT count(*) FROM pg_auth_members m JOIN pg_roles r ON r.oid = m.roleid WHERE r.rolname = $1`, role).Scan(&n)
	if err != nil || n != 0 {
		t.Errorf("memberships of the rolled-back role = %d (err %v), want 0", n, err)
	}
}

// The role is what the design says: no login, no privileges to bypass RLS or administer, a member
// of crm_tenant that inherits from it, and granted to crm_app with SET but not INHERIT (data-model.md §3.1).
func TestSpike_CreatedRoleAttributesAndMemberships(t *testing.T) {
	t.Parallel()
	setup(t)
	ctx := context.Background()
	role := newRoleName()

	tx, err := pgtest.AppPool(t).Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE crm_signup`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT spike_provisioning.create_role($1)`, role); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	pool := pgtest.SuperuserPool(t)
	var login, super, createDB, createRole, replication, bypass bool
	err = pool.QueryRow(ctx, `
		SELECT rolcanlogin, rolsuper, rolcreatedb, rolcreaterole, rolreplication, rolbypassrls
		FROM pg_roles WHERE rolname = $1`, role).Scan(&login, &super, &createDB, &createRole, &replication, &bypass)
	if err != nil {
		t.Fatal(err)
	}
	if login || super || createDB || createRole || replication || bypass {
		t.Errorf("attributes login=%v super=%v createdb=%v createrole=%v replication=%v bypassrls=%v, want all false",
			login, super, createDB, createRole, replication, bypass)
	}

	var inherit, set bool
	err = pool.QueryRow(ctx, `
		SELECT inherit_option, set_option FROM pg_auth_members
		WHERE roleid = 'crm_tenant'::regrole AND member = $1::regrole`, pgx.Identifier{role}.Sanitize()).Scan(&inherit, &set)
	if err != nil {
		t.Fatalf("membership in crm_tenant: %v", err)
	}
	if !inherit || set {
		t.Errorf("membership in crm_tenant: inherit = %v set = %v, want true/false", inherit, set)
	}
	err = pool.QueryRow(ctx, `
		SELECT inherit_option, set_option FROM pg_auth_members
		WHERE roleid = $1::regrole AND member = 'crm_app'::regrole`, pgx.Identifier{role}.Sanitize()).Scan(&inherit, &set)
	if err != nil {
		t.Fatalf("crm_app membership: %v", err)
	}
	if inherit || !set {
		t.Errorf("crm_app in the company role: inherit = %v set = %v, want false/true", inherit, set)
	}
}

// Control cases: crm_app has no way to create roles or to become a role it was not granted (§10.4).
func TestSpike_CrmAppCannotProvisionOrSwitchToArbitraryRoles(t *testing.T) {
	t.Parallel()
	setup(t)
	ctx := context.Background()
	app := pgtest.AppPool(t)

	tests := []struct {
		name string
		sql  string
	}{
		{"execute the function without SET ROLE crm_signup", `SELECT spike_provisioning.create_role('crm_t_00000000000000000000000000000001')`},
		{"CREATE ROLE as crm_app", `CREATE ROLE crm_intruder`},
		{"become crm_owner", `SET LOCAL ROLE crm_owner`},
		{"become crm_provisioner", `SET LOCAL ROLE crm_provisioner`},
		{"become crm_tenant", `SET LOCAL ROLE crm_tenant`},
		{"become the superuser", `SET LOCAL ROLE postgres`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx, err := app.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if _, err := tx.Exec(ctx, tt.sql); sqlState(err) != "42501" {
				t.Errorf("%s: err = %v, want SQLSTATE 42501", tt.name, err)
			}
		})
	}

	t.Run("CREATE ROLE as crm_signup", func(t *testing.T) {
		tx, err := app.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err := tx.Exec(ctx, `SET LOCAL ROLE crm_signup`); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `CREATE ROLE crm_intruder`); sqlState(err) != "42501" {
			t.Errorf("err = %v, want SQLSTATE 42501", err)
		}
	})
}

// Plan §10.4: even the provisioning function cannot create a role with BYPASSRLS, because
// crm_provisioner does not have that attribute.
func TestSpike_ProvisionerCannotCreateBypassRLSRole(t *testing.T) {
	t.Parallel()
	setup(t)
	ctx := context.Background()
	tx, err := pgtest.AppPool(t).Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE crm_signup`); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, `SELECT spike_provisioning.create_bypass_role($1)`, newRoleName())
	if sqlState(err) != "42501" {
		t.Errorf("creating a BYPASSRLS role through crm_provisioner: err = %v, want SQLSTATE 42501", err)
	}
}

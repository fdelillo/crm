//go:build integration

package pgtest_test

import (
	"context"
	"strings"
	"testing"

	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
)

// Roles and memberships of data-model.md §3.1, as created by db/bootstrap/.
func TestBootstrap_RolesAndMemberships(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := pgtest.SuperuserPool(t)

	roles := []struct {
		name              string
		login, createRole bool
		settings          int // number of role-level settings (crm_app: the two timeouts)
	}{
		{"crm_owner", true, false, 0},
		{"crm_app", true, false, 2},
		{"crm_tenant", false, false, 0},
		{"crm_auth", false, false, 0},
		{"crm_worker", false, false, 0},
		{"crm_signup", false, false, 0},
		{"crm_provisioner", false, true, 0},
	}
	for _, r := range roles {
		t.Run("role "+r.name, func(t *testing.T) {
			var login, createRole, super, createDB, bypass, replication bool
			var nSettings int
			err := pool.QueryRow(ctx, `
				SELECT rolcanlogin, rolcreaterole, rolsuper, rolcreatedb, rolbypassrls, rolreplication,
				       coalesce(array_length(rolconfig, 1), 0)
				FROM pg_roles WHERE rolname = $1`, r.name).
				Scan(&login, &createRole, &super, &createDB, &bypass, &replication, &nSettings)
			if err != nil {
				t.Fatalf("role %s: %v", r.name, err)
			}
			if login != r.login || createRole != r.createRole {
				t.Errorf("login = %v createrole = %v, want %v/%v", login, createRole, r.login, r.createRole)
			}
			if super || createDB || bypass || replication {
				t.Errorf("super=%v createdb=%v bypassrls=%v replication=%v, want all false", super, createDB, bypass, replication)
			}
			if nSettings != r.settings {
				t.Errorf("role settings = %d, want %d", nSettings, r.settings)
			}
		})
	}

	var timeout, idle string
	if err := pool.QueryRow(ctx, `
		SELECT
		  (SELECT split_part(c, '=', 2) FROM unnest((SELECT rolconfig FROM pg_roles WHERE rolname = 'crm_app')) c WHERE c LIKE 'statement_timeout=%'),
		  (SELECT split_part(c, '=', 2) FROM unnest((SELECT rolconfig FROM pg_roles WHERE rolname = 'crm_app')) c WHERE c LIKE 'idle_in_transaction_session_timeout=%')`).
		Scan(&timeout, &idle); err != nil {
		t.Fatal(err)
	}
	if timeout != "5s" || idle != "30s" {
		t.Errorf("crm_app statement_timeout = %q, idle_in_transaction_session_timeout = %q, want 5s and 30s", timeout, idle)
	}

	memberships := []struct {
		role, member        string
		admin, inherit, set bool
	}{
		{"crm_provisioner", "crm_owner", false, false, true},
		{"crm_auth", "crm_app", false, false, true},
		{"crm_worker", "crm_app", false, false, true},
		{"crm_signup", "crm_app", false, false, true},
		{"crm_tenant", "crm_provisioner", true, false, false},
	}
	for _, m := range memberships {
		var admin, inherit, set bool
		err := pool.QueryRow(ctx, `
			SELECT admin_option, inherit_option, set_option
			FROM pg_auth_members
			WHERE roleid = $1::regrole AND member = $2::regrole`, m.role, m.member).Scan(&admin, &inherit, &set)
		if err != nil {
			t.Errorf("membership %s in %s: %v", m.member, m.role, err)
			continue
		}
		if admin != m.admin || inherit != m.inherit || set != m.set {
			t.Errorf("%s in %s: admin=%v inherit=%v set=%v, want %v/%v/%v",
				m.member, m.role, admin, inherit, set, m.admin, m.inherit, m.set)
		}
	}

	// crm_app is a member of nothing else but company roles (no crm_tenant, crm_owner, crm_provisioner).
	var n int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM pg_auth_members
		WHERE member = 'crm_app'::regrole
		  AND roleid NOT IN ('crm_auth'::regrole, 'crm_worker'::regrole, 'crm_signup'::regrole)
		  AND pg_get_userbyid(roleid) !~ '^crm_t_[0-9a-f]{32}$'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("crm_app has %d unexpected memberships", n)
	}
}

func TestBootstrap_Database(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := pgtest.SuperuserPool(t)

	var owner string
	if err := pool.QueryRow(ctx, `SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname = 'crm'`).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	if owner != "crm_owner" {
		t.Errorf("database crm owner = %q, want crm_owner", owner)
	}

	var appConnect, ownerConnect, publicConnect, publicTemp bool
	err := pool.QueryRow(ctx, `
		SELECT has_database_privilege('crm_app', 'crm', 'CONNECT'),
		       has_database_privilege('crm_owner', 'crm', 'CONNECT'),
		       EXISTS (SELECT 1 FROM pg_database d, aclexplode(d.datacl) a
		               WHERE d.datname = 'crm' AND a.grantee = 0 AND a.privilege_type = 'CONNECT'),
		       EXISTS (SELECT 1 FROM pg_database d, aclexplode(d.datacl) a
		               WHERE d.datname = 'crm' AND a.grantee = 0 AND a.privilege_type = 'TEMPORARY')`).
		Scan(&appConnect, &ownerConnect, &publicConnect, &publicTemp)
	if err != nil {
		t.Fatal(err)
	}
	if !appConnect || !ownerConnect {
		t.Errorf("CONNECT: crm_app = %v, crm_owner = %v, want both true", appConnect, ownerConnect)
	}
	if publicConnect || publicTemp {
		t.Errorf("PUBLIC has CONNECT = %v / TEMPORARY = %v on crm, want both revoked", publicConnect, publicTemp)
	}
}

// Running the bootstrap again converges without errors and keeps the same state (ADR-004).
// It is sequential on purpose: parallel tests only resume after the sequential ones finish.
func TestBootstrap_IsIdempotent(t *testing.T) {
	ctx := context.Background()
	pgtest.ApplyBootstrap(t)
	pgtest.ApplyBootstrap(t)

	var login bool
	var timeout string
	err := pgtest.SuperuserPool(t).QueryRow(ctx, `
		SELECT rolcanlogin,
		       (SELECT split_part(c, '=', 2) FROM unnest(rolconfig) c WHERE c LIKE 'statement_timeout=%')
		FROM pg_roles WHERE rolname = 'crm_app'`).Scan(&login, &timeout)
	if err != nil {
		t.Fatal(err)
	}
	if !login || timeout != "5s" {
		t.Errorf("after re-running the bootstrap: crm_app login = %v, statement_timeout = %q", login, timeout)
	}
	// The pools created with the initial passwords still work (the passwords were re-applied from the same environment).
	if err := pgtest.AppPool(t).Ping(ctx); err != nil {
		t.Errorf("app pool after re-bootstrap: %v", err)
	}
}

// Re-running the bootstrap does not only add: it brings the cluster back to the described state,
// revoking memberships and database privileges that drifted (ADR-004, ADR-005 INV-02). Company
// roles crm_t_<hex> are legitimate memberships of crm_app and must survive. Sequential on purpose:
// it changes cluster state that the parallel catalog tests read afterwards.
func TestBootstrap_RevertsDrift(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.SuperuserPool(t)
	companyRole := "crm_t_" + strings.ReplaceAll(uuid.NewString(), "-", "")

	mustExec := func(sql string) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	mustExec(`CREATE ROLE ` + companyRole + ` NOLOGIN`)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DROP ROLE IF EXISTS `+companyRole) })
	mustExec(`GRANT ` + companyRole + ` TO crm_app WITH INHERIT FALSE, SET TRUE`)

	// The drift: a group role that lets crm_app inherit table privileges, a role that gives crm_owner
	// more than crm_provisioner, weaker options on a legitimate grant, and extra database privileges.
	mustExec(`GRANT crm_tenant TO crm_app WITH INHERIT TRUE, SET TRUE`)
	mustExec(`GRANT crm_worker TO crm_owner`)
	mustExec(`GRANT crm_auth TO crm_app WITH INHERIT TRUE, SET TRUE`)
	mustExec(`GRANT CREATE, TEMPORARY ON DATABASE crm TO crm_app`)

	pgtest.ApplyBootstrap(t)

	member := func(role, member string) (exists, inherit bool) {
		t.Helper()
		err := pool.QueryRow(ctx, `
			SELECT count(*) > 0, coalesce(bool_or(inherit_option), false) FROM pg_auth_members
			WHERE roleid = $1::regrole AND member = $2::regrole`, role, member).Scan(&exists, &inherit)
		if err != nil {
			t.Fatal(err)
		}
		return exists, inherit
	}
	if ok, _ := member("crm_tenant", "crm_app"); ok {
		t.Error("crm_app is still a member of crm_tenant")
	}
	if ok, _ := member("crm_worker", "crm_owner"); ok {
		t.Error("crm_owner is still a member of crm_worker")
	}
	if ok, inherit := member("crm_auth", "crm_app"); !ok || inherit {
		t.Errorf("crm_auth in crm_app: exists = %v inherit = %v, want a grant without INHERIT", ok, inherit)
	}
	if ok, _ := member("crm_provisioner", "crm_owner"); !ok {
		t.Error("crm_owner lost its legitimate membership in crm_provisioner")
	}
	if ok, _ := member(companyRole, "crm_app"); !ok {
		t.Errorf("crm_app lost its membership in the company role %s", companyRole)
	}

	var create, temp, connect bool
	err := pool.QueryRow(ctx, `
		SELECT has_database_privilege('crm_app', 'crm', 'CREATE'),
		       has_database_privilege('crm_app', 'crm', 'TEMPORARY'),
		       has_database_privilege('crm_app', 'crm', 'CONNECT')`).Scan(&create, &temp, &connect)
	if err != nil {
		t.Fatal(err)
	}
	if create || temp || !connect {
		t.Errorf("crm_app on database crm: CREATE = %v TEMPORARY = %v CONNECT = %v, want false/false/true", create, temp, connect)
	}
}

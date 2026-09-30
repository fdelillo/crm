//go:build integration

package invariants_test

import (
	"context"
	"slices"
	"sort"
	"testing"

	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// expectedTables is the list of tables of schema app. Every spec that adds a table adds it here (and
// to fixture.CompanyInserters): a table nobody declared makes this test fail (data-model.md §6).
var expectedTables = []string{"audit_log", "login_throttles", "outbox_messages", "sessions", "tenants", "user_tokens", "users"}

func queryStrings(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), sql, args...)
	if err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

// appTables reads the tables of schema app from the catalog.
func appTables(t *testing.T) []string {
	t.Helper()
	return queryStrings(t, pgtest.OwnerPool(t), `
		SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'app' AND c.relkind IN ('r', 'p')`)
}

// T-B107, INV-01: the set of tables is exactly the declared one.
func TestCatalog_ExpectedTables(t *testing.T) {
	t.Parallel()
	if got := appTables(t); !slices.Equal(got, expectedTables) {
		t.Errorf("tables in app = %v, want %v (declare new tables here and in fixture.CompanyInserters)", got, expectedTables)
	}
	for _, name := range expectedTables {
		if name == "login_throttles" {
			continue // no tenant_id by design (plan §2, Constitution Check)
		}
		if _, ok := fixture.CompanyInserters[name]; !ok {
			t.Errorf("fixture.CompanyInserters has no entry for %s", name)
		}
	}
}

// T-B107, INV-01: RLS enabled and forced on every table; every table with tenant_id has it NOT NULL
// and a tenant_isolation policy for ALL commands, USING and WITH CHECK, on app.current_tenant_id().
func TestCatalog_RowLevelSecurityOnEveryTable(t *testing.T) {
	t.Parallel()
	pool := pgtest.OwnerPool(t)

	if bad := queryStrings(t, pool, `
		SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'app' AND c.relkind IN ('r', 'p') AND NOT (c.relrowsecurity AND c.relforcerowsecurity)`); len(bad) > 0 {
		t.Errorf("tables without ENABLE + FORCE ROW LEVEL SECURITY: %v", bad)
	}

	if bad := queryStrings(t, pool, `
		SELECT a.attrelid::regclass::text FROM pg_attribute a JOIN pg_class c ON c.oid = a.attrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'app' AND c.relkind IN ('r', 'p') AND a.attname = 'tenant_id' AND NOT a.attisdropped AND NOT a.attnotnull`); len(bad) > 0 {
		t.Errorf("tenant_id is nullable in: %v", bad)
	}

	// Tables that must carry the standard policy: those with a tenant_id, plus tenants (on id).
	withPolicy := queryStrings(t, pool, `
		SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'app' AND c.relkind IN ('r', 'p') AND (c.relname = 'tenants' OR EXISTS (
		  SELECT 1 FROM pg_attribute a WHERE a.attrelid = c.oid AND a.attname = 'tenant_id' AND NOT a.attisdropped))`)
	if len(withPolicy) == 0 {
		t.Fatal("no company tables found")
	}
	for _, table := range withPolicy {
		var cmd, roles, qual, check string
		err := pool.QueryRow(context.Background(), `
			SELECT cmd, roles::text, coalesce(qual, ''), coalesce(with_check, '')
			FROM pg_policies WHERE schemaname = 'app' AND tablename = $1 AND policyname = 'tenant_isolation'`, table).
			Scan(&cmd, &roles, &qual, &check)
		if err != nil {
			t.Errorf("%s: no policy tenant_isolation: %v", table, err)
			continue
		}
		col := "tenant_id"
		if table == "tenants" {
			col = "id"
		}
		if cmd != "ALL" || roles != "{public}" {
			t.Errorf("%s: tenant_isolation applies to %s for %s, want ALL for PUBLIC", table, cmd, roles)
		}
		// Exact equality with the form PostgreSQL 18 prints (a substring test would pass with
		// "... OR (SELECT app.current_tenant_id()) IS NULL", which lets crm_worker see every row).
		canonical := "(" + col + " = ( SELECT app.current_tenant_id() AS current_tenant_id))"
		if qual != canonical || check != canonical {
			t.Errorf("%s: tenant_isolation USING = %q, WITH CHECK = %q, want both %q", table, qual, check, canonical)
		}
	}
}

// T-B107, INV-06: every table and function of app belongs to crm_owner.
func TestCatalog_OwnershipOfSchemaObjects(t *testing.T) {
	t.Parallel()
	pool := pgtest.OwnerPool(t)
	if bad := queryStrings(t, pool, `
		SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'app' AND pg_get_userbyid(c.relowner) <> 'crm_owner'`); len(bad) > 0 {
		t.Errorf("relations in app not owned by crm_owner: %v", bad)
	}
	if bad := queryStrings(t, pool, `
		SELECT p.proname FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = 'app' AND pg_get_userbyid(p.proowner) <> 'crm_owner'`); len(bad) > 0 {
		t.Errorf("functions in app not owned by crm_owner: %v", bad)
	}
}

// T-B107, INV-06: no runtime role can bypass RLS, is a superuser, or owns anything - including the
// roles of the companies created so far.
func TestCatalog_RuntimeRolesAreHarmless(t *testing.T) {
	t.Parallel()
	pool := pgtest.SuperuserPool(t)
	fixture.ProvisionRole(t, pgtest.AppPool(t), uuid.New()) // at least one company role to inspect

	roles := queryStrings(t, pool, `
		SELECT rolname FROM pg_roles
		WHERE rolname IN ('crm_app', 'crm_tenant', 'crm_auth', 'crm_worker', 'crm_signup') OR rolname ~ '^crm_t_[0-9a-f]{32}$'`)
	if !slices.Contains(roles, "crm_app") || len(roles) < 6 {
		t.Fatalf("roles under inspection = %v, want the fixed ones and at least one company role", roles)
	}
	if bad := queryStrings(t, pool, `
		SELECT rolname FROM pg_roles WHERE rolname = ANY($1) AND (rolbypassrls OR rolsuper OR rolcreaterole OR rolcreatedb OR rolreplication)`, roles); len(bad) > 0 {
		t.Errorf("roles with dangerous attributes: %v", bad)
	}
	if bad := queryStrings(t, pool, `
		SELECT DISTINCT r.rolname FROM pg_shdepend d JOIN pg_roles r ON r.oid = d.refobjid
		WHERE d.deptype = 'o' AND r.rolname = ANY($1)`, roles); len(bad) > 0 {
		t.Errorf("roles that own objects: %v", bad)
	}
}

// T-B107, INV-02: crm_app has no privilege on any table or column, direct or inherited.
func TestCatalog_RuntimeRoleHasNoTablePrivileges(t *testing.T) {
	t.Parallel()
	pool := pgtest.SuperuserPool(t)
	if bad := queryStrings(t, pool, `
		SELECT c.oid::regclass::text FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname IN ('app', 'provisioning', 'public') AND c.relkind IN ('r', 'p', 'v', 'm', 'S')
		  AND (has_table_privilege('crm_app', c.oid, 'SELECT, INSERT, UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER')
		       OR (c.relkind IN ('r', 'p', 'v', 'm') AND has_any_column_privilege('crm_app', c.oid, 'SELECT, INSERT, UPDATE, REFERENCES')))`); len(bad) > 0 {
		t.Errorf("crm_app has privileges on: %v", bad)
	}
}

// T-B107, INV-08: SECURITY DEFINER only in provisioning, with search_path = pg_catalog, pg_temp and no PUBLIC EXECUTE.
func TestCatalog_SecurityDefinerFunctions(t *testing.T) {
	t.Parallel()
	pool := pgtest.OwnerPool(t)
	defs := queryStrings(t, pool, `
		SELECT n.nspname || '.' || p.proname FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE p.prosecdef AND n.nspname NOT IN ('pg_catalog', 'information_schema')`)
	if !slices.Equal(defs, []string{"provisioning.provision_tenant_role"}) {
		t.Errorf("SECURITY DEFINER functions = %v, want only provisioning.provision_tenant_role", defs)
	}
	if bad := queryStrings(t, pool, `
		SELECT n.nspname || '.' || p.proname FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE p.prosecdef AND n.nspname NOT IN ('pg_catalog', 'information_schema')
		  AND (n.nspname <> 'provisioning'
		       OR NOT ('search_path=pg_catalog, pg_temp' = ANY(coalesce(p.proconfig, '{}')))
		       OR EXISTS (SELECT 1 FROM aclexplode(coalesce(p.proacl, acldefault('f', p.proowner))) a WHERE a.grantee = 0 AND a.privilege_type = 'EXECUTE'))`); len(bad) > 0 {
		t.Errorf("SECURITY DEFINER functions outside provisioning, without a fixed search_path, or executable by PUBLIC: %v", bad)
	}
}

// T-B107, INV-08: views are security_invoker (there are none in 001).
func TestCatalog_ViewsAreSecurityInvoker(t *testing.T) {
	t.Parallel()
	if bad := queryStrings(t, pgtest.OwnerPool(t), `
		SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'app' AND c.relkind IN ('v', 'm')
		  AND NOT ('security_invoker=true' = ANY(coalesce(c.reloptions, '{}')))`); len(bad) > 0 {
		t.Errorf("views without security_invoker = true: %v", bad)
	}
}

// T-B107, INV-15: audit_log is append-only for the company roles; and no runtime role can delete users
// or companies.
func TestCatalog_ImmutableAndUndeletableTables(t *testing.T) {
	t.Parallel()
	pool := pgtest.SuperuserPool(t)
	company := fixture.ProvisionRole(t, pgtest.AppPool(t), uuid.New())

	for _, role := range []string{"crm_tenant", company} {
		for _, priv := range []string{"UPDATE", "DELETE", "TRUNCATE"} {
			var has bool
			if err := pool.QueryRow(context.Background(),
				`SELECT has_table_privilege($1, 'app.audit_log', $2)`, role, priv).Scan(&has); err != nil {
				t.Fatal(err)
			}
			if has {
				t.Errorf("%s has %s on audit_log", role, priv)
			}
		}
		for _, priv := range []string{"SELECT", "INSERT"} {
			var has bool
			if err := pool.QueryRow(context.Background(),
				`SELECT has_table_privilege($1, 'app.audit_log', $2)`, role, priv).Scan(&has); err != nil || !has {
				t.Errorf("%s lacks %s on audit_log (err %v)", role, priv, err)
			}
		}
	}
	for _, role := range []string{"crm_tenant", "crm_auth", "crm_worker", "crm_signup", "crm_app", company} {
		for _, table := range []string{"app.users", "app.tenants", "app.audit_log"} {
			var has bool
			if err := pool.QueryRow(context.Background(),
				`SELECT has_table_privilege($1, $2, 'DELETE')`, role, table).Scan(&has); err != nil {
				t.Fatal(err)
			}
			if has {
				t.Errorf("%s can DELETE from %s", role, table)
			}
		}
	}
}

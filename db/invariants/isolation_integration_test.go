//go:build integration

package invariants_test

import (
	"context"
	"slices"
	"testing"

	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/isolation"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/fdelillo/crm/internal/testsupport/queryrules"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// companyTables are the tables the isolation test sweeps, taken from the catalog: every table of app
// with a tenant_id, plus tenants (isolated by id). A table without a fixture inserter fails the test.
func companyTables(t *testing.T) []string {
	t.Helper()
	return queryStrings(t, pgtest.OwnerPool(t), `
		SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'app' AND c.relkind IN ('r', 'p') AND (c.relname = 'tenants' OR EXISTS (
		  SELECT 1 FROM pg_attribute a WHERE a.attrelid = c.oid AND a.attname = 'tenant_id' AND NOT a.attisdropped))`)
}

// Tables where the company role has no UPDATE at all (not even on a column) and none where it can DELETE.
// Declared so that a table skipped by the sweep is a visible decision, not a silent one.
var (
	tablesWithoutCompanyUpdate = []string{"audit_log"}
	tablesWithCompanyDelete    []string
)

// T-B110, SC-002 (database level), principle III: with two companies that have a row in every table,
// the role of A sees nothing of B, cannot change it, cannot delete it, cannot write rows for B and cannot
// move its own rows to B.
func TestIsolation_CompanyRoleNeverReachesAnotherCompany(t *testing.T) {
	t.Parallel()
	app := pgtest.AppPool(t)
	a := fixture.NewCompany(t, app)
	b := fixture.NewCompany(t, app)

	tables := companyTables(t)
	if len(tables) == 0 {
		t.Fatal("no company tables found in the catalog")
	}
	var noUpdate, withDelete []string
	for _, table := range tables {
		if _, ok := fixture.CompanyInserters[table]; !ok {
			t.Fatalf("table %s has no fixture inserter: add it to fixture.CompanyInserters so the isolation sweep covers it", table)
		}
		if !isolation.HasAnyUpdate(t, a.Role, table) {
			noUpdate = append(noUpdate, table)
		}
		if isolation.HasPrivilege(t, a.Role, table, "DELETE") {
			withDelete = append(withDelete, table)
		}
	}
	if !slices.Equal(noUpdate, tablesWithoutCompanyUpdate) {
		t.Errorf("tables where the company role has no UPDATE = %v, want the declared %v (declare the change in this test)", noUpdate, tablesWithoutCompanyUpdate)
	}
	if !slices.Equal(withDelete, tablesWithCompanyDelete) {
		t.Errorf("tables where the company role can DELETE = %v, want the declared %v", withDelete, tablesWithCompanyDelete)
	}

	for _, table := range tables {
		table := table
		ran := isolation.Sweep(t, a, b, table, a.Rows[table], b.Rows[table],
			func(ctx context.Context, tx pgx.Tx, tenantID, userID uuid.UUID) error {
				_, err := fixture.Insert(ctx, tx, table, tenantID, userID)
				return err
			})
		wantUpdate := !slices.Contains(tablesWithoutCompanyUpdate, table)
		if got := slices.Contains(ran, "update"); got != wantUpdate {
			t.Errorf("%s: update cases ran = %v, want %v", table, got, wantUpdate)
		}
	}

	// B's rows are intact, as seen by B.
	asRole(t, app, b.Role, func(ctx context.Context, tx pgx.Tx) {
		for _, table := range tables {
			var n int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.`+table+` WHERE id = $1`, b.Rows[table]).Scan(&n); err != nil || n != 1 {
				t.Errorf("after the attempts, B's row in %s: count = %d (err %v), want 1", table, n, err)
			}
		}
	})
}

// tenants is special: the company sees only its own row.
func TestIsolation_TenantsTableShowsOnlyTheOwnCompany(t *testing.T) {
	t.Parallel()
	app := pgtest.AppPool(t)
	a := fixture.NewCompany(t, app)
	fixture.NewCompany(t, app)
	asRole(t, app, a.Role, func(ctx context.Context, tx pgx.Tx) {
		var ids []uuid.UUID
		rows, err := tx.Query(ctx, `SELECT id FROM app.tenants`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			ids = append(ids, id)
		}
		if len(ids) != 1 || ids[0] != a.ID {
			t.Errorf("tenants visible to A = %v, want only %v", ids, a.ID)
		}
	})
}

// login_throttles has no company: the company roles cannot touch it at all.
func TestIsolation_CompanyRoleCannotTouchLoginThrottles(t *testing.T) {
	t.Parallel()
	app := pgtest.AppPool(t)
	a := fixture.NewCompany(t, app)
	asRole(t, app, a.Role, func(ctx context.Context, tx pgx.Tx) {
		if _, err := tx.Exec(ctx, `SELECT * FROM app.login_throttles`); sqlState(err) != "42501" {
			t.Errorf("SELECT as a company role: err = %v, want SQLSTATE 42501", err)
		}
	})
}

// The routing role crm_auth sees id, tenant_id and email of users of every company (plan §4.4,
// documented and bounded): nothing else.
func TestIsolation_AuthRoleSeesOnlyRoutingColumnsOfUsers(t *testing.T) {
	t.Parallel()
	app := pgtest.AppPool(t)
	a := fixture.NewCompany(t, app)
	b := fixture.NewCompany(t, app)
	asRole(t, app, "crm_auth", func(ctx context.Context, tx pgx.Tx) {
		var n int
		err := tx.QueryRow(ctx, `SELECT count(*) FROM app.users WHERE id IN ($1, $2)`, a.UserID, b.UserID).Scan(&n)
		if err != nil || n != 2 {
			t.Errorf("crm_auth sees %d of the two users (err %v), want both", n, err)
		}
		var tenant uuid.UUID
		var email string
		if err := tx.QueryRow(ctx, `SELECT tenant_id, email FROM app.users WHERE id = $1`, b.UserID).Scan(&tenant, &email); err != nil || tenant != b.ID {
			t.Errorf("routing lookup: tenant %v err %v, want %v", tenant, err, b.ID)
		}
	})
	// Any other column is refused, in a separate transaction because the error aborts it.
	asRole(t, app, "crm_auth", func(ctx context.Context, tx pgx.Tx) {
		if _, err := tx.Exec(ctx, `SELECT password_hash FROM app.users`); sqlState(err) != "42501" {
			t.Errorf("SELECT password_hash as crm_auth: err = %v, want SQLSTATE 42501", err)
		}
	})
}

// The company tables that queryrules (T-B112) reads out of the migration files must be exactly the ones
// the database has: otherwise a table it does not see would go unchecked (INV-04).
func TestCatalog_QueryRulesSeeTheSameCompanyTablesAsTheDatabase(t *testing.T) {
	t.Parallel()
	fromFiles, err := queryrules.CompanyTables("../migrations")
	if err != nil {
		t.Fatal(err)
	}
	if fromCatalog := companyTables(t); !slices.Equal(fromFiles, fromCatalog) {
		t.Errorf("queryrules.CompanyTables(migrations) = %v\ncatalog (tables with tenant_id, plus tenants) = %v", fromFiles, fromCatalog)
	}
}

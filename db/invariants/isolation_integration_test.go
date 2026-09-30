//go:build integration

package invariants_test

import (
	"context"
	"testing"

	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
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

func hasPrivilege(t *testing.T, role, table, priv string) bool {
	t.Helper()
	var has bool
	if err := pgtest.SuperuserPool(t).QueryRow(context.Background(),
		`SELECT has_table_privilege($1, $2, $3)`, role, "app."+table, priv).Scan(&has); err != nil {
		t.Fatal(err)
	}
	return has
}

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
	for _, table := range tables {
		if _, ok := fixture.CompanyInserters[table]; !ok {
			t.Fatalf("table %s has no fixture inserter: add it to fixture.CompanyInserters so the isolation sweep covers it", table)
		}
	}

	for _, table := range tables {
		col := "tenant_id"
		if table == "tenants" {
			col = "id"
		}
		rowOfB := b.Rows[table]
		qualified := "app." + table

		t.Run(table+"/select", func(t *testing.T) {
			asRole(t, app, a.Role, func(ctx context.Context, tx pgx.Tx) {
				var total, foreign int
				if err := tx.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE `+col+` <> $1) FROM `+qualified, a.ID).Scan(&total, &foreign); err != nil {
					t.Fatal(err)
				}
				if total < 1 || foreign != 0 {
					t.Errorf("SELECT * without WHERE: %d rows, %d of another company; want at least A's own and none foreign", total, foreign)
				}
				var byID int
				if err := tx.QueryRow(ctx, `SELECT count(*) FROM `+qualified+` WHERE id = $1`, rowOfB).Scan(&byID); err != nil {
					t.Fatal(err)
				}
				if byID != 0 {
					t.Errorf("SELECT ... WHERE id = <B's row> returned %d rows, want 0", byID)
				}
			})
		})

		if hasPrivilege(t, a.Role, table, "UPDATE") {
			t.Run(table+"/update", func(t *testing.T) {
				asRole(t, app, a.Role, func(ctx context.Context, tx pgx.Tx) {
					tag, err := tx.Exec(ctx, `UPDATE `+qualified+` SET `+col+` = `+col+` WHERE id = $1`, rowOfB)
					if err != nil || tag.RowsAffected() != 0 {
						t.Errorf("UPDATE of B's row: affected %d, err %v; want 0 rows and no error", tag.RowsAffected(), err)
					}
					// Moving one of A's own rows to B must be refused by WITH CHECK.
					_, err = tx.Exec(ctx, `UPDATE `+qualified+` SET `+col+` = $2 WHERE id = $1`, a.Rows[table], b.ID)
					if sqlState(err) != "42501" {
						t.Errorf("UPDATE of A's row setting %s = B: err = %v, want SQLSTATE 42501", col, err)
					}
				})
			})
		}
		if hasPrivilege(t, a.Role, table, "DELETE") {
			t.Run(table+"/delete", func(t *testing.T) {
				asRole(t, app, a.Role, func(ctx context.Context, tx pgx.Tx) {
					tag, err := tx.Exec(ctx, `DELETE FROM `+qualified+` WHERE id = $1`, rowOfB)
					if err != nil || tag.RowsAffected() != 0 {
						t.Errorf("DELETE of B's row: affected %d, err %v; want 0", tag.RowsAffected(), err)
					}
				})
			})
		}

		t.Run(table+"/insert for B", func(t *testing.T) {
			asRole(t, app, a.Role, func(ctx context.Context, tx pgx.Tx) {
				// B's own user is used for tables that reference one, so only RLS can stop the insert.
				_, err := fixture.Insert(ctx, tx, table, b.ID, b.UserID)
				if sqlState(err) != "42501" {
					t.Errorf("INSERT with tenant_id = B as A: err = %v, want SQLSTATE 42501", err)
				}
			})
		})
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

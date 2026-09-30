// Package isolation attacks a company table from one company towards another. It is the sweep behind
// the isolation tests (principle III, SC-002 at database level), shared by the invariants that run over
// the real tables and by the probes that prove the sweep also covers column-level UPDATEs.
package isolation

import (
	"context"
	"errors"
	"testing"

	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func sqlState(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// asRole runs fn in a transaction of the runtime pool after SET LOCAL ROLE role, and rolls back.
func asRole(t testing.TB, role string, fn func(ctx context.Context, tx pgx.Tx)) {
	t.Helper()
	ctx := context.Background()
	tx, err := pgtest.AppPool(t).Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fixture.SetRole(ctx, tx, role); err != nil {
		t.Fatalf("SET LOCAL ROLE %s: %v", role, err)
	}
	fn(ctx, tx)
}

// updatableColumn returns a column of table that role may UPDATE (table-level or column-level grant), or "".
func updatableColumn(t *testing.T, role, table string) string {
	t.Helper()
	var col *string
	err := pgtest.SuperuserPool(t).QueryRow(context.Background(), `
		SELECT (SELECT a.attname FROM pg_attribute a
		        WHERE a.attrelid = c.oid AND a.attnum > 0 AND NOT a.attisdropped AND a.attname <> 'id'
		          AND has_column_privilege($1, c.oid, a.attnum, 'UPDATE')
		        ORDER BY a.attnum LIMIT 1)
		FROM pg_class c WHERE c.oid = ('app.' || $2)::regclass`, role, table).Scan(&col)
	if err != nil {
		t.Fatal(err)
	}
	if col == nil {
		return ""
	}
	return *col
}

func HasPrivilege(t *testing.T, role, table, priv string) bool {
	t.Helper()
	var has bool
	if err := pgtest.SuperuserPool(t).QueryRow(context.Background(),
		`SELECT has_table_privilege($1, $2, $3)`, role, "app."+table, priv).Scan(&has); err != nil {
		t.Fatal(err)
	}
	return has
}

// HasAnyUpdate is true for a table-level UPDATE and for a UPDATE granted only on some columns (the
// immutable tables of the financial specs will have the latter, data-model.md §6).
func HasAnyUpdate(t *testing.T, role, table string) bool {
	t.Helper()
	var has bool
	if err := pgtest.SuperuserPool(t).QueryRow(context.Background(),
		`SELECT has_any_column_privilege($1, $2, 'UPDATE')`, role, "app."+table).Scan(&has); err != nil {
		t.Fatal(err)
	}
	return has
}

// Sweep attacks table from company a towards company b and returns the names of the cases it ran, so the
// caller can prove none was skipped. rowA and rowB are ids of one row of each company; insert makes a
// valid row of the table for the company in tenantID.
func Sweep(t *testing.T, a, b fixture.Company, table string, rowA, rowB uuid.UUID,
	insert func(ctx context.Context, tx pgx.Tx, tenantID, userID uuid.UUID) error) []string {
	t.Helper()
	col := "tenant_id"
	if table == "tenants" {
		col = "id"
	}
	qualified := "app." + table
	var ran []string
	run := func(name string, fn func(t *testing.T)) {
		ran = append(ran, name)
		t.Run(table+"/"+name, fn)
	}

	run("select", func(t *testing.T) {
		asRole(t, a.Role, func(ctx context.Context, tx pgx.Tx) {
			var total, foreign int
			if err := tx.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE `+col+` <> $1) FROM `+qualified, a.ID).Scan(&total, &foreign); err != nil {
				t.Fatal(err)
			}
			if total < 1 || foreign != 0 {
				t.Errorf("SELECT * without WHERE: %d rows, %d of another company; want at least A's own and none foreign", total, foreign)
			}
			var byID int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM `+qualified+` WHERE id = $1`, rowB).Scan(&byID); err != nil || byID != 0 {
				t.Errorf("SELECT ... WHERE id = <B's row> returned %d rows (err %v), want 0", byID, err)
			}
		})
	})

	if HasAnyUpdate(t, a.Role, table) {
		updCol := updatableColumn(t, a.Role, table)
		if updCol == "" {
			t.Fatalf("%s: A has UPDATE but no updatable column was found", table)
		}
		run("update", func(t *testing.T) {
			asRole(t, a.Role, func(ctx context.Context, tx pgx.Tx) {
				tag, err := tx.Exec(ctx, `UPDATE `+qualified+` SET `+updCol+` = `+updCol+` WHERE id = $1`, rowB)
				if err != nil || tag.RowsAffected() != 0 {
					t.Errorf("UPDATE of B's row (column %s): affected %d, err %v; want 0 rows and no error", updCol, tag.RowsAffected(), err)
				}
				tag, err = tx.Exec(ctx, `UPDATE `+qualified+` SET `+updCol+` = `+updCol+` WHERE id = $1`, rowA)
				if err != nil || tag.RowsAffected() != 1 {
					t.Errorf("control: UPDATE of A's own row: affected %d, err %v; want 1", tag.RowsAffected(), err)
				}
			})
		})
		run("move own row to B", func(t *testing.T) {
			asRole(t, a.Role, func(ctx context.Context, tx pgx.Tx) {
				// Refused by WITH CHECK when the column can be updated, by privilege when it cannot: both 42501.
				_, err := tx.Exec(ctx, `UPDATE `+qualified+` SET `+col+` = $2 WHERE id = $1`, rowA, b.ID)
				if sqlState(err) != "42501" {
					t.Errorf("UPDATE of A's row setting %s = B: err = %v, want SQLSTATE 42501", col, err)
				}
			})
		})
	}
	if HasPrivilege(t, a.Role, table, "DELETE") {
		run("delete", func(t *testing.T) {
			asRole(t, a.Role, func(ctx context.Context, tx pgx.Tx) {
				tag, err := tx.Exec(ctx, `DELETE FROM `+qualified+` WHERE id = $1`, rowB)
				if err != nil || tag.RowsAffected() != 0 {
					t.Errorf("DELETE of B's row: affected %d, err %v; want 0", tag.RowsAffected(), err)
				}
			})
		})
	}
	run("insert for B", func(t *testing.T) {
		asRole(t, a.Role, func(ctx context.Context, tx pgx.Tx) {
			// B's own user is used for tables that reference one, so only RLS can stop the insert.
			if err := insert(ctx, tx, b.ID, b.UserID); sqlState(err) != "42501" {
				t.Errorf("INSERT with tenant_id = B as A: err = %v, want SQLSTATE 42501", err)
			}
		})
	})
	return ran
}

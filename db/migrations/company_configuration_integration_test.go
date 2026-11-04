//go:build integration

package migrations_test

import (
	"context"
	"database/sql"
	"slices"
	"testing"

	"github.com/fdelillo/crm/db/migrations"
	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pressly/goose/v3"
)

var configurationTables = []string{"app.tenants", "app.tenant_settings", "app.stages", "app.categories", "app.catalog_items"}

// companyBeforeConfiguration creates a company as the code of 001 did: its role, its tenants row and one
// user, and nothing in the configuration tables (which do not exist yet at version 00009).
func companyBeforeConfiguration(t *testing.T) fixture.Company {
	t.Helper()
	ctx := context.Background()
	c := fixture.Company{ID: uuid.New(), Rows: map[string]uuid.UUID{}}
	c.Role = fixture.ProvisionRole(t, pgtest.AppPool(t), c.ID)
	tx, err := pgtest.AppPool(t).Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fixture.SetRole(ctx, tx, c.Role); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"tenants", "users"} {
		if _, err := fixture.Insert(ctx, tx, table, c.ID, uuid.Nil); err != nil {
			t.Fatalf("inserting %s: %v", table, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return c
}

// asCompany runs fn as the company role in a transaction that is rolled back.
func asCompany(t *testing.T, c fixture.Company, fn func(ctx context.Context, tx pgx.Tx)) {
	t.Helper()
	ctx := context.Background()
	tx, err := pgtest.AppPool(t).Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fixture.SetRole(ctx, tx, c.Role); err != nil {
		t.Fatal(err)
	}
	fn(ctx, tx)
}

func tableExists(t *testing.T, name string) bool {
	t.Helper()
	return queryBool(t, pgtest.OwnerPool(t), `SELECT to_regclass($1) IS NOT NULL`, name)
}

func forced(t *testing.T, name string) bool {
	t.Helper()
	return queryBool(t, pgtest.OwnerPool(t), `SELECT relrowsecurity AND relforcerowsecurity FROM pg_class WHERE oid = $1::regclass`, name)
}

// T-002-B101 (002/DD-4, INV-1, INV-4, ADR-004 §6): migration 00010 leaves every company that already
// existed with the minimum configuration, and forces RLS again on the five tables it had to open.
// Down removes the four tables and keeps app.tenants forced. It is sequential on purpose: it moves the
// whole database down to 00009 and back, and parallel tests resume after it.
func TestMigration00010_BackfillsExistingCompanies(t *testing.T) {
	ctx := context.Background()
	migrationDB, err := sql.Open("pgx", pgtest.OwnerURL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = migrationDB.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, migrationDB, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	// Whatever happens, the database goes back to the latest version for the tests that follow.
	t.Cleanup(func() {
		if _, err := provider.Up(context.Background()); err != nil {
			t.Error(err)
		}
	})

	if _, err := provider.DownTo(ctx, 9); err != nil {
		t.Fatalf("down to 00009: %v", err)
	}
	for _, name := range configurationTables[1:] {
		if tableExists(t, name) {
			t.Fatalf("%s exists at version 00009", name)
		}
	}
	companies := []fixture.Company{companyBeforeConfiguration(t), companyBeforeConfiguration(t)}

	if _, err := provider.UpTo(ctx, 10); err != nil {
		t.Fatalf("up to 00010: %v", err)
	}

	for _, name := range configurationTables {
		if !forced(t, name) {
			t.Errorf("%s: RLS is not enabled and forced after the migration", name)
		}
	}
	for _, c := range companies {
		asCompany(t, c, func(ctx context.Context, tx pgx.Tx) {
			var labels [9]string
			var days int
			var terms *string
			var n int
			err := tx.QueryRow(ctx, `
				SELECT count(*) OVER (), project_label_singular, project_label_plural, project_label_gender,
				       customer_label_singular, customer_label_plural, customer_label_gender,
				       quote_label_singular, quote_label_plural, quote_label_gender, quote_validity_days, quote_terms
				FROM app.tenant_settings`).Scan(&n, &labels[0], &labels[1], &labels[2], &labels[3], &labels[4], &labels[5], &labels[6], &labels[7], &labels[8], &days, &terms)
			if err != nil {
				t.Fatalf("company %v: reading tenant_settings: %v", c.ID, err)
			}
			wantLabels := [9]string{"Proyecto", "Proyectos", "masculine", "Cliente", "Clientes", "masculine", "Presupuesto", "Presupuestos", "masculine"} //nolint:misspell // Spanish seed text
			if n != 1 || labels != wantLabels || days != 15 || terms != nil {
				t.Errorf("tenant_settings: rows=%d labels=%v days=%d terms=%v, want 1 row, %v, 15, NULL", n, labels, days, terms, wantLabels)
			}

			if got := readRows(t, ctx, tx, `SELECT system_key || '|' || type || '|' || name FROM app.categories ORDER BY system_key`); !slices.Equal(got, []string{
				"customer_receipts|income|Cobros de clientes", //nolint:misspell // Spanish seed text
				"employee_payments|expense|Pagos a empleados",
				"partner_contributions|income|Aportes de socios",
				"partner_withdrawals|expense|Retiros de socios",
				"supplier_payments|expense|Pagos a proveedores",
			}) {
				t.Errorf("system categories = %v", got)
			}
			if got := readRows(t, ctx, tx, `SELECT count(*)::text FROM app.categories WHERE system_key IS NULL OR archived_at IS NOT NULL`); got[0] != "0" {
				t.Errorf("categories that are not system ones or are archived: %s, want 0", got[0])
			}
			if got := readRows(t, ctx, tx, `SELECT position || '|' || name || '|' || type || '|' || color FROM app.stages WHERE deleted_at IS NULL ORDER BY position`); !slices.Equal(got, []string{
				"1|Presupuesto|open|blue", "2|Aprobado|open|teal", "3|En curso|open|yellow", "4|Finalizado|won|green", "5|Perdido|lost|gray",
			}) {
				t.Errorf("stages = %v", got)
			}
			if got := readRows(t, ctx, tx, `SELECT count(*)::text FROM app.catalog_items`); got[0] != "0" {
				t.Errorf("catalog items = %s, want none", got[0])
			}
			if got := readRows(t, ctx, tx, `SELECT industry_template_version::text FROM app.tenants`); !slices.Equal(got, []string{"1"}) {
				t.Errorf("industry_template_version = %v, want it unchanged (1)", got)
			}
		})
	}

	if _, err := provider.DownTo(ctx, 9); err != nil {
		t.Fatalf("down to 00009: %v", err)
	}
	for _, name := range configurationTables[1:] {
		if tableExists(t, name) {
			t.Errorf("%s still exists after going down to 00009", name)
		}
	}
	if !forced(t, "app.tenants") {
		t.Error("app.tenants lost FORCE ROW LEVEL SECURITY after Down")
	}
}

func readRows(t *testing.T, ctx context.Context, tx pgx.Tx, query string) []string {
	t.Helper()
	rows, err := tx.Query(ctx, query)
	if err != nil {
		t.Fatalf("%s: %v", query, err)
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
	return out
}

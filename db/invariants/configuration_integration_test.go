//go:build integration

package invariants_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// configCompany is a company that has no row in the configuration tables, so each case controls
// exactly what exists. Only its tenants row is created (the foreign key of the four tables).
func configCompany(t *testing.T) fixture.Company {
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
	if _, err := fixture.Insert(ctx, tx, "tenants", c.ID, uuid.Nil); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return c
}

// configCase runs setup and then stmt as the company role in one transaction that is rolled back.
// Every statement receives the company id as $1. setup must succeed; stmt fails with code (and, when
// constraint is not empty, with that constraint or index), or succeeds when code is empty.
type configCase struct {
	name       string
	setup      []string
	stmt       string
	code       string
	constraint string
}

func runConfigCases(t *testing.T, c fixture.Company, cases []configCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			asRole(t, pgtest.AppPool(t), c.Role, func(ctx context.Context, tx pgx.Tx) {
				exec := func(sql string) error {
					var args []any
					if strings.Contains(sql, "$1") {
						args = append(args, c.ID)
					}
					_, err := tx.Exec(ctx, sql, args...)
					return err
				}
				for _, s := range tc.setup {
					if err := exec(s); err != nil {
						t.Fatalf("setup %q: %v", s, err)
					}
				}
				err := exec(tc.stmt)
				if tc.code == "" {
					if err != nil {
						t.Errorf("%s: %v, want success", tc.stmt, err)
					}
					return
				}
				var pgErr *pgconn.PgError
				if !errors.As(err, &pgErr) {
					t.Errorf("%s: err = %v, want SQLSTATE %s %s", tc.stmt, err, tc.code, tc.constraint)
					return
				}
				if pgErr.Code != tc.code || (tc.constraint != "" && pgErr.ConstraintName != tc.constraint) {
					t.Errorf("%s: SQLSTATE %s constraint %q (%s), want %s %q", tc.stmt, pgErr.Code, pgErr.ConstraintName, pgErr.Message, tc.code, tc.constraint)
				}
			})
		})
	}
}

// The statements below are valid rows of each table; the cases vary one column at a time.
const (
	insertStage        = `INSERT INTO app.stages (tenant_id, name, type, color, position) VALUES ($1, '%s', '%s', '%s', 1)`
	insertCategoryOf   = `INSERT INTO app.categories (tenant_id, name, type) VALUES ($1, '%s', '%s')`
	insertSystemOf     = `INSERT INTO app.categories (tenant_id, name, type, system_key) VALUES ($1, 'Sistema', '%s', '%s')`
	insertCatalogItem  = `INSERT INTO app.catalog_items (tenant_id, name, unit%s) VALUES ($1, 'Ventana', '%s'%s)`
	validStageInsert   = `INSERT INTO app.stages (tenant_id, name, type, color, position) VALUES ($1, 'Vidrios', 'open', 'blue', 1)`
	validCategoryExp   = `INSERT INTO app.categories (tenant_id, name, type) VALUES ($1, 'Flete', 'expense')`
	validCatalogInsert = `INSERT INTO app.catalog_items (tenant_id, name, unit) VALUES ($1, 'Ventana', 'unit')`
)

func stage(name, typ, color string) string { return fmt.Sprintf(insertStage, name, typ, color) }
func category(name, typ string) string     { return fmt.Sprintf(insertCategoryOf, name, typ) }
func system(typ, key string) string        { return fmt.Sprintf(insertSystemOf, typ, key) }

// catalog inserts an item of the given unit; extraCols and extraVals are appended verbatim.
func catalog(unit, extraCols, extraVals string) string {
	return fmt.Sprintf(insertCatalogItem, extraCols, unit, extraVals)
}

// settings inserts a valid tenant_settings row with one column replaced.
func settings(column, value string) string {
	cols := []string{
		"project_label_singular", "project_label_plural", "project_label_gender",
		"customer_label_singular", "customer_label_plural", "customer_label_gender",
		"quote_label_singular", "quote_label_plural", "quote_label_gender", "quote_validity_days",
	}
	vals := map[string]string{
		"project_label_singular": "'Proyecto'", "project_label_plural": "'Proyectos'", "project_label_gender": "'masculine'",
		"customer_label_singular": "'Cliente'", "customer_label_plural": "'Clientes'", "customer_label_gender": "'masculine'",
		"quote_label_singular": "'Presupuesto'", "quote_label_plural": "'Presupuestos'", "quote_label_gender": "'masculine'",
		"quote_validity_days": "15",
	}
	if _, ok := vals[column]; ok {
		vals[column] = value
	} else {
		cols = append(cols, column)
		vals[column] = value
	}
	var list []string
	for _, col := range cols {
		list = append(list, vals[col])
	}
	return `INSERT INTO app.tenant_settings (tenant_id, ` + strings.Join(cols, ", ") + `) VALUES ($1, ` + strings.Join(list, ", ") + `)`
}

// T-002-B101 (002/INV-1, INV-6, INV-7, DD-8, DD-9): the constraints and indexes of the four tables, with
// the names data-model.md gives them (the error mapping of the services uses them).
func TestConfigurationTables_Constraints(t *testing.T) {
	t.Parallel()
	c := configCompany(t)
	long := func(n int) string { return strings.Repeat("a", n) }

	t.Run("categories", func(t *testing.T) {
		runConfigCases(t, c, []configCase{
			{name: "customer_receipts is income only", stmt: system("expense", "customer_receipts"), code: "23514", constraint: "categories_system_type_chk"},
			{name: "supplier_payments is expense only", stmt: system("income", "supplier_payments"), code: "23514", constraint: "categories_system_type_chk"},
			{name: "customer_receipts as income", stmt: system("income", "customer_receipts")},
			{name: "partner_withdrawals as expense", stmt: system("expense", "partner_withdrawals")},
			{name: "cash_count_adjustment does not exist yet", stmt: system("income", "cash_count_adjustment"), code: "23514", constraint: "categories_system_key_chk"},
			{name: "system category cannot be archived",
				setup: []string{system("income", "customer_receipts")},
				stmt:  `UPDATE app.categories SET archived_at = now() WHERE system_key = 'customer_receipts'`, code: "23514", constraint: "categories_system_not_archived_chk"},
			{name: "own category can be archived",
				setup: []string{category("Flete", "expense")},
				stmt:  `UPDATE app.categories SET archived_at = now() WHERE name = 'Flete'`},
			{name: "same system_key twice in one company",
				setup: []string{system("income", "customer_receipts")},
				stmt:  system("income", "customer_receipts"), code: "23505", constraint: "categories_system_key_key"},
			{name: "two active expenses named Flete",
				setup: []string{category("Flete", "expense")},
				stmt:  category("Flete", "expense"), code: "23505", constraint: "categories_active_name_key"},
			{name: "uniqueness ignores case",
				setup: []string{category("Flete", "expense")},
				stmt:  category("FLETE", "expense"), code: "23505", constraint: "categories_active_name_key"},
			{name: "the same name as an income",
				setup: []string{category("Flete", "expense")},
				stmt:  category("Flete", "income")},
			{name: "the same name as an archived one",
				setup: []string{`INSERT INTO app.categories (tenant_id, name, type, archived_at) VALUES ($1, 'Flete', 'expense', now())`},
				stmt:  category("Flete", "expense")},
			{name: "type outside the enumeration", stmt: category("Flete", "transfer"), code: "23514", constraint: "categories_type_chk"},
			{name: "empty name", stmt: category("", "expense"), code: "23514", constraint: "categories_name_chk"},
			{name: "name of 60 characters", stmt: category(long(60), "expense")},
			{name: "name of 61 characters", stmt: category(long(61), "expense"), code: "23514", constraint: "categories_name_chk"},
		})
	})

	t.Run("categories in two companies", func(t *testing.T) {
		// categories_system_key_key is per company: another company has its own customer_receipts.
		for _, co := range []fixture.Company{configCompany(t), configCompany(t)} {
			asRoleCommit(t, pgtest.AppPool(t), co.Role, func(ctx context.Context, tx pgx.Tx) {
				if _, err := tx.Exec(ctx, system("income", "customer_receipts"), co.ID); err != nil {
					t.Fatalf("company %v: %v", co.ID, err)
				}
			})
		}
	})

	t.Run("stages", func(t *testing.T) {
		runConfigCases(t, c, []configCase{
			{name: "two active stages Vidrios and vidrios",
				setup: []string{validStageInsert},
				stmt:  stage("vidrios", "open", "blue"), code: "23505", constraint: "stages_active_name_key"},
			{name: "the first one deleted",
				setup: []string{`INSERT INTO app.stages (tenant_id, name, type, color, position, deleted_at) VALUES ($1, 'Vidrios', 'open', 'blue', 1, now())`},
				stmt:  stage("vidrios", "open", "blue")},
			{name: "the same position twice is allowed (no unique index on position, DD-5)",
				setup: []string{validStageInsert},
				stmt:  stage("Otra", "open", "blue")},
			{name: "color outside the palette", stmt: stage("Vidrios", "open", "pink"), code: "23514", constraint: "stages_color_chk"},
			{name: "every palette color", stmt: stage("Vidrios", "open", "purple")},
			{name: "type closed", stmt: stage("Vidrios", "closed", "blue"), code: "23514", constraint: "stages_type_chk"},
			{name: "type won", stmt: stage("Vidrios", "won", "green")},
			{name: "type lost", stmt: stage("Vidrios", "lost", "gray")},
			{name: "name of 60 characters", stmt: stage(long(60), "open", "blue")},
			{name: "name of 61 characters", stmt: stage(long(61), "open", "blue"), code: "23514", constraint: "stages_name_chk"},
			{name: "empty name", stmt: stage("", "open", "blue"), code: "23514", constraint: "stages_name_chk"},
			{name: "position 0",
				stmt: `INSERT INTO app.stages (tenant_id, name, type, color, position) VALUES ($1, 'Vidrios', 'open', 'blue', 0)`, code: "23514", constraint: "stages_position_chk"},
		})
	})

	t.Run("catalog_items", func(t *testing.T) {
		runConfigCases(t, c, []configCase{
			{name: "price without currency", stmt: catalog("unit", ", reference_price_cents", ", 100"), code: "23514", constraint: "catalog_items_reference_price_pair_chk"},
			{name: "currency without price", stmt: catalog("unit", ", reference_price_currency", ", 'ARS'"), code: "23514", constraint: "catalog_items_reference_price_pair_chk"},
			{name: "price and currency", stmt: catalog("unit", ", reference_price_cents, reference_price_currency", ", 100, 'USD'")},
			{name: "no price at all", stmt: validCatalogInsert},
			{name: "price -1", stmt: catalog("unit", ", reference_price_cents, reference_price_currency", ", -1, 'ARS'"), code: "23514", constraint: "catalog_items_reference_price_cents_chk"},
			{name: "price 2^53", stmt: catalog("unit", ", reference_price_cents, reference_price_currency", ", 9007199254740992, 'ARS'"), code: "23514", constraint: "catalog_items_reference_price_cents_chk"},
			{name: "price 2^53 - 1", stmt: catalog("unit", ", reference_price_cents, reference_price_currency", ", 9007199254740991, 'ARS'")},
			{name: "price 0", stmt: catalog("unit", ", reference_price_cents, reference_price_currency", ", 0, 'ARS'")},
			{name: "currency EUR", stmt: catalog("unit", ", reference_price_cents, reference_price_currency", ", 1, 'EUR'"), code: "23514", constraint: "catalog_items_reference_price_currency_chk"},
			{name: "unit other without label", stmt: catalog("other", "", ""), code: "23514", constraint: "catalog_items_unit_label_chk"},
			{name: "unit other with label", stmt: catalog("other", ", unit_label", ", 'jornada'")},
			{name: "unit hour with label", stmt: catalog("hour", ", unit_label", ", 'jornada'"), code: "23514", constraint: "catalog_items_unit_label_chk"},
			{name: "unit label of 21 characters", stmt: catalog("other", ", unit_label", ", '"+long(21)+"'"), code: "23514", constraint: "catalog_items_unit_label_length_chk"},
			{name: "empty unit label", stmt: catalog("other", ", unit_label", ", ''"), code: "23514", constraint: "catalog_items_unit_label_length_chk"},
			{name: "unit kg", stmt: catalog("kg", "", ""), code: "23514", constraint: "catalog_items_unit_chk"},
			{name: "unit square_meter", stmt: catalog("square_meter", "", "")},
			{name: "unit linear_meter", stmt: catalog("linear_meter", "", "")},
			{name: "same name twice is allowed (DD-9)", setup: []string{validCatalogInsert}, stmt: validCatalogInsert},
			{name: "name of 120 characters", stmt: `INSERT INTO app.catalog_items (tenant_id, name, unit) VALUES ($1, '` + long(120) + `', 'unit')`},
			{name: "name of 121 characters", stmt: `INSERT INTO app.catalog_items (tenant_id, name, unit) VALUES ($1, '` + long(121) + `', 'unit')`, code: "23514", constraint: "catalog_items_name_chk"},
			{name: "description of 1000 characters", stmt: catalog("unit", ", description", ", '"+long(1000)+"'")},
			{name: "description of 1001 characters", stmt: catalog("unit", ", description", ", '"+long(1001)+"'"), code: "23514", constraint: "catalog_items_description_chk"},
			{name: "empty description", stmt: catalog("unit", ", description", ", ''"), code: "23514", constraint: "catalog_items_description_chk"},
		})
	})

	t.Run("tenant_settings", func(t *testing.T) {
		runConfigCases(t, c, []configCase{
			{name: "a valid row", stmt: settings("quote_terms", "NULL")},
			{name: "a second row for the company",
				setup: []string{settings("quote_terms", "NULL")},
				stmt:  settings("quote_terms", "NULL"), code: "23505", constraint: "tenant_settings_tenant_id_key"},
			{name: "project gender neutral", stmt: settings("project_label_gender", "'neutral'"), code: "23514", constraint: "tenant_settings_project_label_gender_chk"},
			{name: "customer gender neutral", stmt: settings("customer_label_gender", "'neutral'"), code: "23514", constraint: "tenant_settings_customer_label_gender_chk"},
			{name: "quote gender neutral", stmt: settings("quote_label_gender", "'neutral'"), code: "23514", constraint: "tenant_settings_quote_label_gender_chk"},
			{name: "feminine", stmt: settings("quote_label_gender", "'feminine'")},
			{name: "validity 0", stmt: settings("quote_validity_days", "0"), code: "23514", constraint: "tenant_settings_quote_validity_days_chk"},
			{name: "validity 1", stmt: settings("quote_validity_days", "1")},
			{name: "validity 365", stmt: settings("quote_validity_days", "365")},
			{name: "validity 366", stmt: settings("quote_validity_days", "366"), code: "23514", constraint: "tenant_settings_quote_validity_days_chk"},
			{name: "empty terms", stmt: settings("quote_terms", "''"), code: "23514", constraint: "tenant_settings_quote_terms_chk"},
			{name: "terms of 2000 characters", stmt: settings("quote_terms", "'"+long(2000)+"'")},
			{name: "terms of 2001 characters", stmt: settings("quote_terms", "'"+long(2001)+"'"), code: "23514", constraint: "tenant_settings_quote_terms_chk"},
			{name: "empty singular", stmt: settings("project_label_singular", "''"), code: "23514", constraint: "tenant_settings_project_label_singular_chk"},
			{name: "label of 40 characters", stmt: settings("customer_label_plural", "'"+long(40)+"'")},
			{name: "label of 41 characters", stmt: settings("customer_label_plural", "'"+long(41)+"'"), code: "23514", constraint: "tenant_settings_customer_label_plural_chk"},
			{name: "quote label of 41 characters", stmt: settings("quote_label_singular", "'"+long(41)+"'"), code: "23514", constraint: "tenant_settings_quote_label_singular_chk"},
		})
	})
}

// T-002-B101 (002/INV-3, INV-5): the company role cannot delete, and cannot update the columns that are
// immutable by design; the controls show that the columns it may update do work.
func TestConfigurationTables_PrivilegesOfTheCompanyRole(t *testing.T) {
	t.Parallel()
	c := configCompany(t)
	setups := map[string]string{
		"tenant_settings": settings("quote_terms", "NULL"),
		"stages":          validStageInsert,
		"categories":      validCategoryExp,
		"catalog_items":   validCatalogInsert,
	}
	var cases []configCase
	for _, table := range []string{"tenant_settings", "stages", "categories", "catalog_items"} {
		cases = append(cases, configCase{name: table + ": DELETE", setup: []string{setups[table]}, stmt: `DELETE FROM app.` + table, code: "42501"})
		for _, set := range []string{"id = uuidv7()", "tenant_id = uuidv7()", "created_at = now()"} {
			cases = append(cases, configCase{name: table + ": SET " + set, setup: []string{setups[table]},
				stmt: `UPDATE app.` + table + ` SET ` + set, code: "42501"})
		}
	}
	cases = append(cases,
		configCase{name: "categories: SET type", setup: []string{validCategoryExp}, stmt: `UPDATE app.categories SET type = 'income'`, code: "42501"},
		configCase{name: "categories: SET system_key", setup: []string{validCategoryExp}, stmt: `UPDATE app.categories SET system_key = 'supplier_payments'`, code: "42501"},
		configCase{name: "categories: SET name, archived_at, updated_at (control)", setup: []string{validCategoryExp},
			stmt: `UPDATE app.categories SET name = 'Otro', archived_at = now(), updated_at = now()`},
		configCase{name: "stages: SET name, type, color, position, deleted_at, updated_at (control)", setup: []string{validStageInsert},
			stmt: `UPDATE app.stages SET name = 'Otra', type = 'won', color = 'red', position = 2, deleted_at = now(), updated_at = now()`},
		configCase{name: "catalog_items: SET every editable column (control)", setup: []string{validCatalogInsert},
			stmt: `UPDATE app.catalog_items SET name = 'X', description = 'd', unit = 'other', unit_label = 'jornada',
				reference_price_cents = 1, reference_price_currency = 'USD', archived_at = now(), updated_at = now()`},
		configCase{name: "tenant_settings: SET every editable column (control)", setup: []string{settings("quote_terms", "NULL")},
			stmt: `UPDATE app.tenant_settings SET project_label_singular = 'Obra', project_label_plural = 'Obras', project_label_gender = 'feminine',
				customer_label_singular = 'Cliente', customer_label_plural = 'Clientes', customer_label_gender = 'masculine',
				quote_label_singular = 'Cotización', quote_label_plural = 'Cotizaciones', quote_label_gender = 'feminine',
				quote_validity_days = 30, quote_terms = 'Seña del 50 %', updated_at = now()`},
	)
	runConfigCases(t, c, cases)
}

// T-002-B101 (002/INV-5, ADR-028): the exact columns the company role may UPDATE, as data-model.md §3.2
// lists them, read from the catalog; and nothing for the system roles (T-B109 keeps them empty).
func TestConfigurationTables_UpdatableColumnsAreExactlyTheDocumentedOnes(t *testing.T) {
	t.Parallel()
	want := map[string][]string{
		"tenant_settings": {
			"customer_label_gender", "customer_label_plural", "customer_label_singular",
			"project_label_gender", "project_label_plural", "project_label_singular",
			"quote_label_gender", "quote_label_plural", "quote_label_singular",
			"quote_terms", "quote_validity_days", "updated_at",
		},
		"stages":        {"color", "deleted_at", "name", "position", "type", "updated_at"},
		"categories":    {"archived_at", "name", "updated_at"},
		"catalog_items": {"archived_at", "description", "name", "reference_price_cents", "reference_price_currency", "unit", "unit_label", "updated_at"},
	}
	for table, columns := range want {
		got := queryStrings(t, pgtest.SuperuserPool(t), `
			SELECT a.attname FROM pg_attribute a
			WHERE a.attrelid = ('app.' || $1)::regclass AND a.attnum > 0 AND NOT a.attisdropped
			  AND has_column_privilege('crm_tenant', a.attrelid, a.attnum, 'UPDATE')`, table)
		if !slices.Equal(got, columns) {
			t.Errorf("%s: crm_tenant can UPDATE %v, want %v", table, got, columns)
		}
		for _, priv := range []string{"DELETE", "TRUNCATE"} {
			if got := queryStrings(t, pgtest.SuperuserPool(t), `SELECT $2 WHERE has_table_privilege('crm_tenant', 'app.'||$1, $2)`, table, priv); len(got) > 0 {
				t.Errorf("%s: crm_tenant has %s", table, priv)
			}
		}
	}
}

// T-002-B101 (data-model.md §2): the named constraints and indexes exist with the documented names,
// because the services map their SQLSTATE to errors by name (phases 4 and 5) and 004 to 009 point
// composite foreign keys at the (tenant_id, id) keys.
func TestConfigurationTables_NamedConstraintsAndIndexesExist(t *testing.T) {
	t.Parallel()
	pool := pgtest.OwnerPool(t)
	want := []string{
		"catalog_items.catalog_items_reference_price_pair_chk",
		"catalog_items.catalog_items_tenant_id_id_key",
		"catalog_items.catalog_items_unit_label_chk",
		"categories.categories_active_name_key",
		"categories.categories_system_key_chk",
		"categories.categories_system_key_key",
		"categories.categories_system_not_archived_chk",
		"categories.categories_system_type_chk",
		"categories.categories_tenant_id_id_key",
		"stages.stages_active_name_key",
		"stages.stages_tenant_id_id_key",
		"tenant_settings.tenant_settings_tenant_id_key",
	}
	got := queryStrings(t, pool, `
		SELECT tablename || '.' || indexname FROM pg_indexes WHERE schemaname = 'app' AND indexname = ANY($1)
		UNION
		SELECT conrelid::regclass::text || '.' || conname FROM pg_constraint
		WHERE connamespace = 'app'::regnamespace AND conname = ANY($1)`, trimTables(want))
	for i := range got {
		got[i] = strings.TrimPrefix(got[i], "app.")
	}
	slices.Sort(got)
	got = slices.Compact(got)
	if !slices.Equal(got, want) {
		t.Errorf("named constraints and indexes\n got %v\nwant %v", got, want)
	}

	// The partial unique indexes: what they are made of is what INV-4 and INV-6 depend on.
	defs := map[string][]string{
		"stages_active_name_key":     {"(tenant_id, lower(name))", "WHERE (deleted_at IS NULL)"},
		"categories_active_name_key": {"(tenant_id, type, lower(name))", "WHERE (archived_at IS NULL)"},
		"categories_system_key_key":  {"(tenant_id, system_key)", "WHERE (system_key IS NOT NULL)"},
	}
	for name, parts := range defs {
		var def string
		if err := pool.QueryRow(context.Background(), `SELECT indexdef FROM pg_indexes WHERE schemaname='app' AND indexname=$1`, name).Scan(&def); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if !strings.Contains(def, "CREATE UNIQUE INDEX") {
			t.Errorf("%s is not unique: %s", name, def)
		}
		for _, part := range parts {
			if !strings.Contains(def, part) {
				t.Errorf("%s = %s, want it to contain %s", name, def, part)
			}
		}
	}
}

func trimTables(qualified []string) []string {
	out := make([]string, len(qualified))
	for i, q := range qualified {
		_, name, _ := strings.Cut(q, ".")
		out[i] = name
	}
	return out
}

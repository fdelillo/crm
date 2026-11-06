//go:build integration

package app_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/industrytemplate"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/outbox"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/settings"
	"github.com/fdelillo/crm/internal/tenant"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// The tests of this file register a company with the real tenant.Service and the real
// settings.TemplateSeeder. They live in internal/app because it is the one place that may import both
// modules (depguard: settings does not import tenant, tenant does not import settings).

var errInjectedStatement = errors.New("injected statement failure")

// meteredTx counts the statements a seeder sends through it and fails the failAt-th one (0 = never),
// without running it. Everything else is the real transaction of the registration.
type meteredTx struct {
	db.Tx
	statements *int
	failAt     int
}

func (m meteredTx) next() error {
	*m.statements++
	if *m.statements == m.failAt {
		return errInjectedStatement
	}
	return nil
}

func (m meteredTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if err := m.next(); err != nil {
		return pgconn.CommandTag{}, err
	}
	return m.Tx.Exec(ctx, sql, args...)
}

func (m meteredTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if err := m.next(); err != nil {
		return nil, err
	}
	return m.Tx.Query(ctx, sql, args...)
}

func (m meteredTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if err := m.next(); err != nil {
		return failedRow{err}
	}
	return m.Tx.QueryRow(ctx, sql, args...)
}

type failedRow struct{ err error }

func (r failedRow) Scan(...any) error { return r.err }

// meteredSeeder runs the real TemplateSeeder through a meteredTx. With a non-nil replacement it seeds
// that template instead of the one the registration looked up (Register only knows the catalog).
type meteredSeeder struct {
	inner       settings.TemplateSeeder
	failAt      int
	replacement *industrytemplate.Template
	statements  int
	tenantID    uuid.UUID
}

func (s *meteredSeeder) Seed(ctx context.Context, tx db.Tx, id uuid.UUID, t industrytemplate.Template) error {
	s.tenantID = id
	if s.replacement != nil {
		t = *s.replacement
	}
	return s.inner.Seed(ctx, meteredTx{Tx: tx, statements: &s.statements, failAt: s.failAt}, id, t)
}

var seedingMoment = time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)

type fixedClock struct{ at time.Time }

func (c fixedClock) Now() time.Time { return c.at }

func newSeededRegistration(t *testing.T, seeder industrytemplate.Seeder) (*tenant.Service, db.TxRunner) {
	t.Helper()
	runner := db.NewTxRunner(pgtest.AppPool(t))
	c := clock.Real{}
	users := identity.NewService(runner, outbox.NewEnqueuer(c), c, 24*time.Hour, 7*24*time.Hour)
	return tenant.NewService(runner, users, seeder, password.NewHasher(2), audit.NewRecorder(),
		slog.New(slog.NewTextHandler(io.Discard, nil))), runner
}

func registerWithTemplate(t *testing.T, svc *tenant.Service, code string) tenant.Registration {
	t.Helper()
	got, err := svc.Register(context.Background(), tenant.Signup{Name: "Ana", Email: uuid.NewString() + "@example.test",
		Password: "a safe password 123", CompanyName: "Empresa", BaseCurrency: "ARS", IndustryTemplateCode: code},
		identity.RequestMeta{})
	if err != nil {
		t.Fatalf("register %s: %v", code, err)
	}
	return got
}

type storedConfiguration struct {
	settings   []string // one line per tenant_settings row, every column but the ids
	stages     []string // in position order: "position|name|type|color"
	categories []string // sorted: "type|name|system_key" (system_key empty = none)
	items      []string // sorted: "name|description|unit|unit_label|price"
	stamps     []time.Time
	versions   string
	audit      []string
}

// readConfiguration reads what a company sees of its own configuration, with its own role.
func readConfiguration(t *testing.T, runner db.TxRunner, tenantID uuid.UUID) storedConfiguration {
	t.Helper()
	var got storedConfiguration
	collect := func(ctx context.Context, tx db.Tx, query string, into *[]string, args ...any) error {
		rows, err := tx.Query(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				return err
			}
			*into = append(*into, line)
		}
		return rows.Err()
	}
	err := runner.InTenantTx(context.Background(), tenantID, func(ctx context.Context, tx db.Tx) error {
		queries := []struct {
			sql  string
			into *[]string
		}{
			{`SELECT concat_ws('|', project_label_singular, project_label_plural, project_label_gender,
				customer_label_singular, customer_label_plural, customer_label_gender,
				quote_label_singular, quote_label_plural, quote_label_gender, quote_validity_days,
				coalesce(quote_terms, '<NULL>')) FROM app.tenant_settings WHERE tenant_id = $1`, &got.settings},
			{`SELECT concat_ws('|', position, name, type, color, coalesce(deleted_at::text, '<NULL>'))
				FROM app.stages WHERE tenant_id = $1 ORDER BY position`, &got.stages},
			{`SELECT concat_ws('|', type, name, coalesce(system_key, ''), coalesce(archived_at::text, '<NULL>'))
				FROM app.categories WHERE tenant_id = $1 ORDER BY type, name`, &got.categories},
			{`SELECT concat_ws('|', name, coalesce(description, '<NULL>'), unit, coalesce(unit_label, '<NULL>'),
				coalesce(reference_price_cents::text, '<NULL>'), coalesce(reference_price_currency, '<NULL>'),
				coalesce(archived_at::text, '<NULL>')) FROM app.catalog_items WHERE tenant_id = $1 ORDER BY name`, &got.items},
			{`SELECT action || ' ' || target_type FROM app.audit_log WHERE tenant_id = $1 ORDER BY action`, &got.audit},
		}
		for _, q := range queries {
			if err := collect(ctx, tx, q.sql, q.into, tenantID); err != nil {
				return err
			}
		}
		rows, err := tx.Query(ctx, `SELECT created_at, updated_at FROM app.tenant_settings WHERE tenant_id = $1
			UNION ALL SELECT created_at, updated_at FROM app.stages WHERE tenant_id = $1
			UNION ALL SELECT created_at, updated_at FROM app.categories WHERE tenant_id = $1
			UNION ALL SELECT created_at, updated_at FROM app.catalog_items WHERE tenant_id = $1`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var created, updated time.Time
			if err := rows.Scan(&created, &updated); err != nil {
				return err
			}
			got.stamps = append(got.stamps, created, updated)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `SELECT industry_template_code || '@' || industry_template_version
			FROM app.tenants WHERE id = $1`, tenantID).Scan(&got.versions)
	})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

var systemCategoryLines = []string{
	"expense|Pagos a empleados|employee_payments|<NULL>",
	"expense|Pagos a proveedores|supplier_payments|<NULL>",
	"expense|Retiros de socios|partner_withdrawals|<NULL>",
	"income|Aportes de socios|partner_contributions|<NULL>",
	"income|Cobros de clientes|customer_receipts|<NULL>",
}

func sortedLines(groups ...[]string) []string {
	out := slices.Concat(groups...)
	slices.Sort(out)
	return out
}

// T-002-B305 (FR-006, DD-3, INV-1, INV-4): registering with each template leaves the whole configuration of
// that template in the company, and registering does not audit the configuration.
func TestRegisterSeedsTheTemplateConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name, code, version string
		settings            string
		stages              []string
		categories          []string
		items               []string
	}{
		{
			name: "aluminum carpentry", code: "aluminum_carpentry", version: "aluminum_carpentry@2",
			settings: "Proyecto|Proyectos|masculine|Cliente|Clientes|masculine|Presupuesto|Presupuestos|masculine|15|" +
				"Seña del 50 % al aprobar el presupuesto y saldo contra entrega.",
			stages: []string{
				"1|Presupuesto|open|blue|<NULL>", "2|Aprobado|open|teal|<NULL>", "3|En producción|open|orange|<NULL>",
				"4|Instalación|open|purple|<NULL>", "5|Finalizado|won|green|<NULL>", "6|Perdido|lost|gray|<NULL>",
			},
			categories: sortedLines(systemCategoryLines, []string{
				"expense|Perfiles de aluminio||<NULL>", "expense|Vidrios||<NULL>", "expense|Accesorios||<NULL>",
				"expense|Flete||<NULL>", "expense|Mano de obra||<NULL>", "expense|Alquiler||<NULL>",
				"expense|Servicios||<NULL>", "expense|Impuestos||<NULL>", "income|Otros ingresos||<NULL>",
			}),
			items: sortedLines([]string{
				"Ventana corrediza|<NULL>|square_meter|<NULL>|<NULL>|<NULL>|<NULL>",
				"Puerta de abrir|<NULL>|unit|<NULL>|<NULL>|<NULL>|<NULL>",
				"Mampara de baño|<NULL>|unit|<NULL>|<NULL>|<NULL>|<NULL>",
				"Colocación|<NULL>|unit|<NULL>|<NULL>|<NULL>|<NULL>",
			}),
		},
		{
			name: "generic", code: "generic", version: "generic@2",
			settings: "Proyecto|Proyectos|masculine|Cliente|Clientes|masculine|Presupuesto|Presupuestos|masculine|15|<NULL>",
			stages: []string{
				"1|Presupuesto|open|blue|<NULL>", "2|Aprobado|open|teal|<NULL>", "3|En curso|open|yellow|<NULL>",
				"4|Finalizado|won|green|<NULL>", "5|Perdido|lost|gray|<NULL>",
			},
			categories: sortedLines(systemCategoryLines, []string{
				"expense|Materiales||<NULL>", "expense|Mano de obra||<NULL>", "expense|Flete||<NULL>",
				"expense|Alquiler||<NULL>", "expense|Servicios||<NULL>", "expense|Impuestos||<NULL>",
				"expense|Otros gastos||<NULL>", "income|Otros ingresos||<NULL>",
			}),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, runner := newSeededRegistration(t, settings.NewTemplateSeeder(fixedClock{seedingMoment}))
			got := registerWithTemplate(t, svc, tc.code)
			cfg := readConfiguration(t, runner, got.Tenant.ID)

			if !slices.Equal(cfg.settings, []string{tc.settings}) { // INV-1: exactly one row
				t.Errorf("tenant_settings = %q, want exactly %q", cfg.settings, tc.settings)
			}
			if !slices.Equal(cfg.stages, tc.stages) {
				t.Errorf("stages = %q, want %q", cfg.stages, tc.stages)
			}
			if !slices.Equal(cfg.categories, tc.categories) { // INV-4: the 5 system ones plus the template's
				t.Errorf("categories = %q, want %q", cfg.categories, tc.categories)
			}
			if !slices.Equal(cfg.items, tc.items) {
				t.Errorf("catalog_items = %q, want %q", cfg.items, tc.items)
			}
			if cfg.versions != tc.version {
				t.Errorf("company template = %q, want %q", cfg.versions, tc.version)
			}
			// The seeder audits nothing: tenant.registered is the only entry, as before the configuration existed.
			if !slices.Equal(cfg.audit, []string{"tenant.registered tenant"}) {
				t.Errorf("audit entries = %q, want only tenant.registered", cfg.audit)
			}
			// Every row is stamped with the seeder's clock, never with the database's now().
			if want := 1 + len(tc.stages) + len(tc.categories) + len(tc.items); len(cfg.stamps) != 2*want {
				t.Errorf("rows stamped = %d, want %d", len(cfg.stamps)/2, want)
			}
			for _, stamp := range cfg.stamps {
				if !stamp.Equal(seedingMoment) {
					t.Fatalf("a row is stamped %s, want %s", stamp, seedingMoment)
				}
			}
		})
	}
}

// thirtyStageTemplate is a valid template with a 30 stage pipeline and no catalog items.
func thirtyStageTemplate() industrytemplate.Template {
	base, _ := industrytemplate.Lookup("generic")
	tpl := base
	tpl.Stages = nil
	for i := 1; i <= 28; i++ {
		tpl.Stages = append(tpl.Stages, industrytemplate.TemplateStage{Name: fmt.Sprintf("Etapa %02d", i), Type: "open", Color: "blue"})
	}
	tpl.Stages = append(tpl.Stages,
		industrytemplate.TemplateStage{Name: "Ganada", Type: "won", Color: "green"},
		industrytemplate.TemplateStage{Name: "Perdida", Type: "lost", Color: "gray"})
	tpl.CatalogItems = nil
	return tpl
}

// T-002-B305 (DD-3, INV-10): Seed sends exactly four statements whatever the size of the template: not one per
// stage, category or item, and the empty lists (no items) do not change the count.
func TestSeedSendsExactlyFourStatements(t *testing.T) {
	thirty := thirtyStageTemplate()
	for _, tc := range []struct {
		name, code  string
		replacement *industrytemplate.Template
		wantStages  int
	}{
		{"generic", "generic", nil, 5},
		{"aluminum carpentry", "aluminum_carpentry", nil, 6},
		{"thirty stages and no items", "generic", &thirty, 30},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seeder := &meteredSeeder{inner: settings.NewTemplateSeeder(clock.Real{}), replacement: tc.replacement}
			svc, runner := newSeededRegistration(t, seeder)
			got := registerWithTemplate(t, svc, tc.code)
			if seeder.statements != 4 {
				t.Fatalf("Seed sent %d statements, want exactly 4", seeder.statements)
			}
			if cfg := readConfiguration(t, runner, got.Tenant.ID); len(cfg.stages) != tc.wantStages {
				t.Fatalf("stages = %d, want %d", len(cfg.stages), tc.wantStages)
			}
		})
	}
}

// T-002-B305 (DD-3, INV-14 of 001): whichever of the four statements fails, Register fails and leaves no
// company, role, user or configuration row behind.
func TestRegisterRollsBackWhenASeedStatementFails(t *testing.T) {
	for failAt := 1; failAt <= 4; failAt++ {
		t.Run(fmt.Sprintf("statement %d", failAt), func(t *testing.T) {
			seeder := &meteredSeeder{inner: settings.NewTemplateSeeder(clock.Real{}), failAt: failAt}
			svc, _ := newSeededRegistration(t, seeder)
			_, err := svc.Register(context.Background(), tenant.Signup{Name: "Ana", Email: uuid.NewString() + "@example.test",
				Password: "a safe password 123", CompanyName: "Empresa", BaseCurrency: "ARS", IndustryTemplateCode: "aluminum_carpentry"},
				identity.RequestMeta{})
			if !errors.Is(err, errInjectedStatement) {
				t.Fatalf("Register error = %v, want the injected failure", err)
			}
			if seeder.statements != failAt {
				t.Fatalf("Seed sent %d statements, want it to stop at %d", seeder.statements, failAt)
			}
			var roles, tenants, users, sessions, tokens, messages, audits, settingsRows, stages, categories, items int
			err = pgtest.SuperuserPool(t).QueryRow(context.Background(), `SELECT
				(SELECT count(*) FROM pg_roles WHERE rolname = $1),
				(SELECT count(*) FROM app.tenants WHERE id = $2),
				(SELECT count(*) FROM app.users WHERE tenant_id = $2),
				(SELECT count(*) FROM app.sessions WHERE tenant_id = $2),
				(SELECT count(*) FROM app.user_tokens WHERE tenant_id = $2),
				(SELECT count(*) FROM app.outbox_messages WHERE tenant_id = $2),
				(SELECT count(*) FROM app.audit_log WHERE tenant_id = $2),
				(SELECT count(*) FROM app.tenant_settings WHERE tenant_id = $2),
				(SELECT count(*) FROM app.stages WHERE tenant_id = $2),
				(SELECT count(*) FROM app.categories WHERE tenant_id = $2),
				(SELECT count(*) FROM app.catalog_items WHERE tenant_id = $2)`,
				db.TenantRoleName(seeder.tenantID), seeder.tenantID).
				Scan(&roles, &tenants, &users, &sessions, &tokens, &messages, &audits, &settingsRows, &stages, &categories, &items)
			if err != nil {
				t.Fatal(err)
			}
			if left := []int{roles, tenants, users, sessions, tokens, messages, audits, settingsRows, stages, categories, items}; slices.Max(left) != 0 {
				t.Fatalf("rows left after the rollback (role, tenant, users, sessions, tokens, messages, audit, settings, stages, categories, items) = %v", left)
			}
		})
	}
}

// T-002-B305 (DD-3, INV-1): the rows a company is seeded with are invisible to the role of another company,
// in the four tables, even when asking for them by the other company's id.
func TestSeededConfigurationIsInvisibleToAnotherCompany(t *testing.T) {
	svc, runner := newSeededRegistration(t, settings.NewTemplateSeeder(clock.Real{}))
	a := registerWithTemplate(t, svc, "aluminum_carpentry")
	b := registerWithTemplate(t, svc, "generic")
	for _, table := range []string{"tenant_settings", "stages", "categories", "catalog_items"} {
		var ofA, all int
		err := runner.InTenantTx(context.Background(), b.Tenant.ID, func(ctx context.Context, tx db.Tx) error {
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.`+table+` WHERE tenant_id = $1`, a.Tenant.ID).Scan(&ofA); err != nil {
				return err
			}
			return tx.QueryRow(ctx, `SELECT count(*) FROM app.`+table).Scan(&all)
		})
		if err != nil {
			t.Fatal(err)
		}
		// B has 1 settings row, 5 stages, 13 categories and no items of its own: it sees only those.
		want := map[string]int{"tenant_settings": 1, "stages": 5, "categories": 13, "catalog_items": 0}[table]
		if ofA != 0 || all != want {
			t.Errorf("%s as company B: rows of A = %d (want 0), rows seen = %d (want %d)", table, ofA, all, want)
		}
	}
}

// T-002-B305 (DD-3, DD-34 of 001), independent test of the phase: POST /auth/signup with each template leaves
// the configuration in place and GET /me, on another connection right after, answers 200.
func TestSignupWithEachTemplateSeedsTheConfigurationAndMeAnswers(t *testing.T) {
	for _, tc := range []struct {
		code                                string
		stages, categories, items, settings int
	}{
		{"aluminum_carpentry", 6, 14, 4, 1},
		{"generic", 5, 13, 0, 1},
	} {
		t.Run(tc.code, func(t *testing.T) {
			server := newAuthServer(t)
			email := uuid.NewString() + "@example.test"
			resp := postSignup(t, server, `{"name":"Ana","email":"`+email+`","password":"example-password","company_name":"ACME","base_currency":"ARS","industry_template_code":"`+tc.code+`"}`, "application/json")
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusCreated {
				t.Fatalf("signup status = %d", resp.StatusCode)
			}
			me, err := server.Client.Get(server.URL + "/api/v1/me")
			if err != nil {
				t.Fatal(err)
			}
			defer me.Body.Close()
			if me.StatusCode != http.StatusOK {
				t.Fatalf("GET /me right after signup = %d, want 200", me.StatusCode)
			}
			var got [4]int
			err = pgtest.SuperuserPool(t).QueryRow(context.Background(), `SELECT
				(SELECT count(*) FROM app.stages s JOIN app.users u ON u.tenant_id = s.tenant_id WHERE u.email = $1),
				(SELECT count(*) FROM app.categories s JOIN app.users u ON u.tenant_id = s.tenant_id WHERE u.email = $1),
				(SELECT count(*) FROM app.catalog_items s JOIN app.users u ON u.tenant_id = s.tenant_id WHERE u.email = $1),
				(SELECT count(*) FROM app.tenant_settings s JOIN app.users u ON u.tenant_id = s.tenant_id WHERE u.email = $1)`,
				email).Scan(&got[0], &got[1], &got[2], &got[3])
			if err != nil {
				t.Fatal(err)
			}
			if want := [4]int{tc.stages, tc.categories, tc.items, tc.settings}; got != want {
				t.Fatalf("stages, categories, items, settings = %v, want %v", got, want)
			}
		})
	}
}

//go:build integration

package settings_test

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/settings"
	"github.com/google/uuid"
)

// categoriesEnv is a company seeded with the "generic" template plus the five system categories.
type categoriesEnv struct {
	t    *testing.T
	clk  *testClock
	svc  *settings.Service
	co   company
	meta settings.RequestMeta
}

func newCategoriesEnv(t *testing.T) *categoriesEnv {
	t.Helper()
	clk := newTestClock()
	return &categoriesEnv{t: t, clk: clk, svc: newService(t, clk), co: newCompany(t, clk, "generic"),
		meta: settings.RequestMeta{IP: netip.MustParseAddr("192.0.2.20"), UserAgent: "categories-test/1.0"}}
}

func (e *categoriesEnv) list(f settings.CategoryFilter) []settings.Category {
	e.t.Helper()
	got, err := e.svc.ListCategories(context.Background(), e.co.Admin, f)
	if err != nil {
		e.t.Fatalf("ListCategories: %v", err)
	}
	return got
}

// find returns the category with that name and type among all of them, archived ones included.
func (e *categoriesEnv) find(name string, typ settings.CategoryType) settings.Category {
	e.t.Helper()
	for _, c := range e.list(settings.CategoryFilter{IncludeArchived: true}) {
		if c.Name == name && c.Type == typ {
			return c
		}
	}
	e.t.Fatalf("no category %q of type %s", name, typ)
	return settings.Category{}
}

func (e *categoriesEnv) create(name string, typ settings.CategoryType) settings.Category {
	e.t.Helper()
	got, err := e.svc.CreateCategory(context.Background(), e.co.Admin, name, typ, e.meta)
	if err != nil {
		e.t.Fatalf("CreateCategory(%q): %v", name, err)
	}
	return got
}

func (e *categoriesEnv) archived(name string, typ settings.CategoryType) settings.Category {
	e.t.Helper()
	c := e.create(name, typ)
	got, err := e.svc.ArchiveCategory(context.Background(), e.co.Admin, c.ID, e.meta)
	if err != nil {
		e.t.Fatalf("ArchiveCategory(%q): %v", name, err)
	}
	return got
}

// assertNothingChanged runs op and checks that no category row and no audit row changed, even after the
// clock moved: a rejected or empty operation writes nothing (002/INV-13).
func (e *categoriesEnv) assertNothingChanged(op func()) {
	e.t.Helper()
	before, auditBefore := storedCategories(e.t, e.co.ID), readAudit(e.t, e.co.ID, "category.")
	e.clk.Advance(time.Hour)
	op()
	if after := storedCategories(e.t, e.co.ID); after != before {
		e.t.Fatalf("categories changed:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if auditAfter := readAudit(e.t, e.co.ID, "category."); len(auditAfter) != len(auditBefore) {
		e.t.Fatalf("audit rows: %d before, %d after", len(auditBefore), len(auditAfter))
	}
}

func names(cs []settings.Category) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = fmt.Sprintf("%s:%s", c.Type, c.Name)
	}
	return out
}

// storedCategories reads every row of app.categories of the company with every column an operation may touch.
func storedCategories(t *testing.T, tenantID uuid.UUID) string {
	t.Helper()
	var lines []string
	err := testRunner(t).InTenantTx(context.Background(), tenantID, func(ctx context.Context, tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id, name, type, coalesce(system_key, ''), archived_at, created_at, updated_at
			FROM app.categories WHERE tenant_id = $1 ORDER BY created_at, id`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id uuid.UUID
			var name, typ, key string
			var archived *time.Time
			var created, updated time.Time
			if err := rows.Scan(&id, &name, &typ, &key, &archived, &created, &updated); err != nil {
				return err
			}
			lines = append(lines, fmt.Sprintf("%s|%s|%s|%s|%v|%s|%s", id, name, typ, key, archived, created.UTC(), updated.UTC()))
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatalf("read categories: %v", err)
	}
	return strings.Join(lines, "\n")
}

func categoryUpdatedAt(t *testing.T, tenantID, id uuid.UUID) time.Time {
	t.Helper()
	var updated time.Time
	err := testRunner(t).InTenantTx(context.Background(), tenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT updated_at FROM app.categories WHERE tenant_id = $1 AND id = $2`, tenantID, id).Scan(&updated)
	})
	if err != nil {
		t.Fatal(err)
	}
	return updated
}

// wantAudit checks that the audit log of the company has exactly one more category row than before, with that action.
func wantCategoryAudit(t *testing.T, e *categoriesEnv, before []auditRow, action string, target uuid.UUID, data map[string]any) {
	t.Helper()
	after := readAudit(t, e.co.ID, "category.")
	if len(after) != len(before)+1 {
		t.Fatalf("audit rows: %d before, %d after, want one more", len(before), len(after))
	}
	row := after[len(after)-1]
	if row.Action != action || row.TargetType != "category" || row.TargetID == nil || *row.TargetID != target ||
		row.ActorID == nil || *row.ActorID != e.co.Admin.UserID || row.UserAgent != e.meta.UserAgent ||
		row.IP == nil || *row.IP != e.meta.IP {
		t.Fatalf("audit row = %+v, want %s on %s", row, action, target)
	}
	equal(t, "audit data", row.Data, data)
}

// T-002-B501 (Historia 3.1, FR-003, 002/DD-8, DD-11): the exact order of the list, and its filters.
func TestListCategories(t *testing.T) {
	e := newCategoriesEnv(t)
	other := newCompany(t, e.clk, "aluminum_carpentry")
	otherBefore, err := e.svc.ListCategories(context.Background(), other.Admin, settings.CategoryFilter{})
	if err != nil {
		t.Fatal(err)
	}

	wantAll := []string{
		"income:Aportes de socios", "income:Cobros de clientes", "income:Otros ingresos",
		"expense:Pagos a empleados", "expense:Pagos a proveedores", "expense:Retiros de socios",
		"expense:Alquiler", "expense:Flete", "expense:Impuestos", "expense:Mano de obra", "expense:Materiales",
		"expense:Otros gastos", "expense:Servicios",
	}
	for name, p := range map[string]authz.Principal{"admin": e.co.Admin, "operator": e.co.Operator} {
		t.Run("default list for "+name, func(t *testing.T) {
			got, err := e.svc.ListCategories(context.Background(), p, settings.CategoryFilter{})
			if err != nil {
				t.Fatal(err)
			}
			equal(t, "categories", names(got), wantAll)
			for _, c := range got {
				if c.ID == uuid.Nil || c.Archived || (c.SystemKey != nil) != (c.Name != "" && isSystemName(c.Name)) {
					t.Fatalf("category %+v", c)
				}
			}
		})
	}
	t.Run("only expenses", func(t *testing.T) {
		expense := settings.CategoryType("expense")
		got := e.list(settings.CategoryFilter{Type: &expense})
		equal(t, "categories", names(got), wantAll[3:])
	})
	t.Run("only incomes", func(t *testing.T) {
		income := settings.CategoryType("income")
		equal(t, "categories", names(e.list(settings.CategoryFilter{Type: &income})), wantAll[:3])
	})
	t.Run("archived ones only with IncludeArchived", func(t *testing.T) {
		gone := e.archived("Combustible", "expense")
		for _, c := range e.list(settings.CategoryFilter{}) {
			if c.ID == gone.ID {
				t.Fatalf("archived category in the default list")
			}
		}
		var found *settings.Category
		for _, c := range e.list(settings.CategoryFilter{IncludeArchived: true}) {
			if c.ID == gone.ID {
				found = &c
			}
		}
		if found == nil || !found.Archived || found.Name != "Combustible" {
			t.Fatalf("archived category = %+v", found)
		}
	})
	t.Run("another company is not shown", func(t *testing.T) {
		got, err := e.svc.ListCategories(context.Background(), other.Admin, settings.CategoryFilter{})
		if err != nil {
			t.Fatal(err)
		}
		equal(t, "other company", names(got), names(otherBefore))
	})
}

func isSystemName(name string) bool {
	switch name {
	case "Cobros de clientes", "Aportes de socios", "Pagos a proveedores", "Pagos a empleados", "Retiros de socios":
		return true
	}
	return false
}

// T-002-B501 (Historia 3.1, 002/INV-6, DD-8): creating a category.
func TestCreateCategory(t *testing.T) {
	ctx := context.Background()

	t.Run("an expense", func(t *testing.T) {
		e := newCategoriesEnv(t)
		before := readAudit(t, e.co.ID, "category.")
		got, err := e.svc.CreateCategory(ctx, e.co.Admin, "  Combustible ", "expense", e.meta)
		if err != nil {
			t.Fatal(err)
		}
		if got.ID == uuid.Nil || got.Name != "Combustible" || got.Type != "expense" || got.SystemKey != nil || got.Archived {
			t.Fatalf("created = %+v", got)
		}
		equal(t, "stored", e.find("Combustible", "expense"), got)
		wantCategoryAudit(t, e, before, "category.created", got.ID, map[string]any{"type": "expense"})
	})
	t.Run("the name is taken ignoring case, within the type", func(t *testing.T) {
		e := newCategoriesEnv(t)
		e.assertNothingChanged(func() {
			_, err := e.svc.CreateCategory(ctx, e.co.Admin, "flete", "expense", e.meta)
			if !errors.Is(err, settings.ErrNameTaken) {
				t.Fatalf("error = %v, want ErrNameTaken", err)
			}
		})
	})
	t.Run("the same name in the other type is free", func(t *testing.T) {
		e := newCategoriesEnv(t)
		e.create("Flete", "income")
	})
	t.Run("the name of an archived category is free", func(t *testing.T) {
		e := newCategoriesEnv(t)
		e.archived("Combustible", "expense")
		e.create("Combustible", "expense")
	})
	t.Run("invalid fields", func(t *testing.T) {
		e := newCategoriesEnv(t)
		e.assertNothingChanged(func() {
			_, err := e.svc.CreateCategory(ctx, e.co.Admin, "  ", "other", e.meta)
			var fields settings.FieldErrors
			if !errors.As(err, &fields) {
				t.Fatalf("error = %v", err)
			}
			equal(t, "fields", map[string]string(fields), map[string]string{"name": "required", "type": "invalid_value"})
		})
		e.assertNothingChanged(func() {
			_, err := e.svc.CreateCategory(ctx, e.co.Admin, strings.Repeat("a", 61), "income", e.meta)
			wantFieldError(t, err, "name", "too_long")
		})
	})
	t.Run("an operator cannot", func(t *testing.T) {
		e := newCategoriesEnv(t)
		e.assertNothingChanged(func() {
			_, err := e.svc.CreateCategory(ctx, e.co.Operator, "Combustible", "expense", e.meta)
			if !errors.Is(err, authz.ErrForbidden) {
				t.Fatalf("error = %v, want ErrForbidden", err)
			}
		})
	})
}

// T-002-B501 (Historia 3.3, FR-004, 002/DD-8, DD-15, INV-6, INV-13): renaming a category.
func TestRenameCategory(t *testing.T) {
	ctx := context.Background()

	t.Run("a system category keeps its key", func(t *testing.T) {
		e := newCategoriesEnv(t)
		original := e.find("Cobros de clientes", "income")
		before := readAudit(t, e.co.ID, "category.")
		e.clk.Advance(time.Hour)
		got, err := e.svc.RenameCategory(ctx, e.co.Admin, original.ID, " Cobranzas ", e.meta)
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "Cobranzas" || got.SystemKey == nil || *got.SystemKey != "customer_receipts" || got.ID != original.ID || got.Type != "income" {
			t.Fatalf("renamed = %+v", got)
		}
		equal(t, "stored", e.find("Cobranzas", "income"), got)
		if want := e.clk.Now(); !categoryUpdatedAt(t, e.co.ID, got.ID).Equal(want) {
			t.Fatalf("updated_at = %s, want %s", categoryUpdatedAt(t, e.co.ID, got.ID), want)
		}
		wantCategoryAudit(t, e, before, "category.renamed", got.ID, map[string]any{})
	})
	t.Run("to the name of another active one of the same type", func(t *testing.T) {
		e := newCategoriesEnv(t)
		c := e.create("Combustible", "expense")
		e.assertNothingChanged(func() {
			_, err := e.svc.RenameCategory(ctx, e.co.Admin, c.ID, "FLETE", e.meta)
			if !errors.Is(err, settings.ErrNameTaken) {
				t.Fatalf("error = %v, want ErrNameTaken", err)
			}
		})
	})
	t.Run("to the name of an active one of the other type", func(t *testing.T) {
		e := newCategoriesEnv(t)
		c := e.create("Combustible", "income")
		if _, err := e.svc.RenameCategory(ctx, e.co.Admin, c.ID, "Flete", e.meta); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("to the name of an archived one", func(t *testing.T) {
		e := newCategoriesEnv(t)
		e.archived("Peajes", "expense")
		c := e.create("Combustible", "expense")
		if _, err := e.svc.RenameCategory(ctx, e.co.Admin, c.ID, "Peajes", e.meta); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("an archived category", func(t *testing.T) {
		e := newCategoriesEnv(t)
		c := e.archived("Peajes", "expense")
		got, err := e.svc.RenameCategory(ctx, e.co.Admin, c.ID, "Peajes y cocheras", e.meta)
		if err != nil || got.Name != "Peajes y cocheras" || !got.Archived {
			t.Fatalf("got %+v, %v", got, err)
		}
	})
	t.Run("the same name writes nothing", func(t *testing.T) {
		e := newCategoriesEnv(t)
		c := e.find("Flete", "expense")
		e.assertNothingChanged(func() {
			got, err := e.svc.RenameCategory(ctx, e.co.Admin, c.ID, "  Flete ", e.meta)
			if err != nil || got != c {
				t.Fatalf("got %+v, %v; want %+v", got, err, c)
			}
		})
	})
	t.Run("a different case is a change", func(t *testing.T) {
		e := newCategoriesEnv(t)
		c := e.find("Flete", "expense")
		got, err := e.svc.RenameCategory(ctx, e.co.Admin, c.ID, "FLETE", e.meta)
		if err != nil || got.Name != "FLETE" {
			t.Fatalf("got %+v, %v", got, err)
		}
	})
	t.Run("invalid name", func(t *testing.T) {
		e := newCategoriesEnv(t)
		c := e.find("Flete", "expense")
		e.assertNothingChanged(func() {
			_, err := e.svc.RenameCategory(ctx, e.co.Admin, c.ID, "   ", e.meta)
			wantFieldError(t, err, "name", "required")
		})
	})
	t.Run("another company or unknown id", func(t *testing.T) {
		e := newCategoriesEnv(t)
		other := newCompany(t, e.clk, "generic")
		foreign, err := e.svc.ListCategories(ctx, other.Admin, settings.CategoryFilter{})
		if err != nil {
			t.Fatal(err)
		}
		e.assertNothingChanged(func() {
			for _, id := range []uuid.UUID{foreign[0].ID, uuid.New()} {
				if _, err := e.svc.RenameCategory(ctx, e.co.Admin, id, "Otro", e.meta); !errors.Is(err, settings.ErrNotFound) {
					t.Fatalf("error = %v, want ErrNotFound", err)
				}
			}
		})
	})
	t.Run("an operator cannot", func(t *testing.T) {
		e := newCategoriesEnv(t)
		c := e.find("Flete", "expense")
		e.assertNothingChanged(func() {
			if _, err := e.svc.RenameCategory(ctx, e.co.Operator, c.ID, "Otro", e.meta); !errors.Is(err, authz.ErrForbidden) {
				t.Fatalf("error = %v, want ErrForbidden", err)
			}
		})
	})
}

// T-002-B501 (Historia 3.2, 002/DD-8, DD-15, INV-4, INV-13): archiving a category.
func TestArchiveCategory(t *testing.T) {
	ctx := context.Background()

	t.Run("a category of the company", func(t *testing.T) {
		e := newCategoriesEnv(t)
		c := e.create("Combustible", "expense")
		before := readAudit(t, e.co.ID, "category.")
		e.clk.Advance(time.Hour)
		got, err := e.svc.ArchiveCategory(ctx, e.co.Admin, c.ID, e.meta)
		if err != nil {
			t.Fatal(err)
		}
		if !got.Archived || got.ID != c.ID || got.Name != "Combustible" {
			t.Fatalf("archived = %+v", got)
		}
		for _, listed := range e.list(settings.CategoryFilter{}) {
			if listed.ID == c.ID {
				t.Fatal("archived category in the default list")
			}
		}
		if want := e.clk.Now(); !categoryUpdatedAt(t, e.co.ID, c.ID).Equal(want) {
			t.Fatalf("updated_at = %s, want %s", categoryUpdatedAt(t, e.co.ID, c.ID), want)
		}
		wantCategoryAudit(t, e, before, "category.archived", c.ID, map[string]any{})
	})
	t.Run("an archived one comes back as it is", func(t *testing.T) {
		e := newCategoriesEnv(t)
		c := e.archived("Combustible", "expense")
		e.assertNothingChanged(func() {
			got, err := e.svc.ArchiveCategory(ctx, e.co.Admin, c.ID, e.meta)
			if err != nil || got != c {
				t.Fatalf("got %+v, %v; want %+v", got, err, c)
			}
		})
	})
	t.Run("a system category", func(t *testing.T) {
		e := newCategoriesEnv(t)
		c := e.find("Cobros de clientes", "income")
		e.assertNothingChanged(func() {
			if _, err := e.svc.ArchiveCategory(ctx, e.co.Admin, c.ID, e.meta); !errors.Is(err, settings.ErrSystemCategory) {
				t.Fatalf("error = %v, want ErrSystemCategory", err)
			}
		})
	})
	t.Run("another company or unknown id", func(t *testing.T) {
		e := newCategoriesEnv(t)
		other := newCompany(t, e.clk, "generic")
		foreign, err := e.svc.ListCategories(ctx, other.Admin, settings.CategoryFilter{})
		if err != nil {
			t.Fatal(err)
		}
		e.assertNothingChanged(func() {
			for _, id := range []uuid.UUID{foreign[len(foreign)-1].ID, uuid.New()} {
				if _, err := e.svc.ArchiveCategory(ctx, e.co.Admin, id, e.meta); !errors.Is(err, settings.ErrNotFound) {
					t.Fatalf("error = %v, want ErrNotFound", err)
				}
			}
		})
		if got, _ := e.svc.ListCategories(ctx, other.Admin, settings.CategoryFilter{}); len(got) != len(foreign) {
			t.Fatalf("the other company lost categories")
		}
	})
	t.Run("an operator cannot", func(t *testing.T) {
		e := newCategoriesEnv(t)
		c := e.find("Flete", "expense")
		e.assertNothingChanged(func() {
			if _, err := e.svc.ArchiveCategory(ctx, e.co.Operator, c.ID, e.meta); !errors.Is(err, authz.ErrForbidden) {
				t.Fatalf("error = %v, want ErrForbidden", err)
			}
		})
	})
}

// T-002-B501 (Historia 3.2, 002/DD-8, DD-15, INV-6, INV-13): restoring a category.
func TestRestoreCategory(t *testing.T) {
	ctx := context.Background()

	t.Run("an archived one", func(t *testing.T) {
		e := newCategoriesEnv(t)
		c := e.archived("Combustible", "expense")
		before := readAudit(t, e.co.ID, "category.")
		e.clk.Advance(time.Hour)
		got, err := e.svc.RestoreCategory(ctx, e.co.Admin, c.ID, e.meta)
		if err != nil {
			t.Fatal(err)
		}
		if got.Archived || got.ID != c.ID {
			t.Fatalf("restored = %+v", got)
		}
		equal(t, "stored", e.find("Combustible", "expense"), got)
		if want := e.clk.Now(); !categoryUpdatedAt(t, e.co.ID, c.ID).Equal(want) {
			t.Fatalf("updated_at = %s, want %s", categoryUpdatedAt(t, e.co.ID, c.ID), want)
		}
		wantCategoryAudit(t, e, before, "category.restored", c.ID, map[string]any{})
	})
	t.Run("the name is now used by another active one", func(t *testing.T) {
		e := newCategoriesEnv(t)
		old := e.archived("Combustible", "expense")
		e.create("combustible", "expense")
		e.assertNothingChanged(func() {
			if _, err := e.svc.RestoreCategory(ctx, e.co.Admin, old.ID, e.meta); !errors.Is(err, settings.ErrNameTaken) {
				t.Fatalf("error = %v, want ErrNameTaken", err)
			}
		})
	})
	t.Run("an active one comes back as it is", func(t *testing.T) {
		e := newCategoriesEnv(t)
		c := e.create("Combustible", "expense")
		e.assertNothingChanged(func() {
			got, err := e.svc.RestoreCategory(ctx, e.co.Admin, c.ID, e.meta)
			if err != nil || got != c {
				t.Fatalf("got %+v, %v; want %+v", got, err, c)
			}
		})
	})
	t.Run("another company or unknown id", func(t *testing.T) {
		e := newCategoriesEnv(t)
		theirs := &categoriesEnv{t: t, clk: e.clk, svc: e.svc, co: newCompany(t, e.clk, "generic"), meta: e.meta}
		gone := theirs.archived("Combustible", "expense")
		e.assertNothingChanged(func() {
			for _, id := range []uuid.UUID{gone.ID, uuid.New()} {
				if _, err := e.svc.RestoreCategory(ctx, e.co.Admin, id, e.meta); !errors.Is(err, settings.ErrNotFound) {
					t.Fatalf("error = %v, want ErrNotFound", err)
				}
			}
		})
		if got := theirs.find("Combustible", "expense"); !got.Archived {
			t.Fatalf("the other company's category was restored: %+v", got)
		}
	})
	t.Run("an operator cannot", func(t *testing.T) {
		e := newCategoriesEnv(t)
		c := e.archived("Combustible", "expense")
		e.assertNothingChanged(func() {
			if _, err := e.svc.RestoreCategory(ctx, e.co.Operator, c.ID, e.meta); !errors.Is(err, authz.ErrForbidden) {
				t.Fatalf("error = %v, want ErrForbidden", err)
			}
		})
	})
}

// T-002-B501 (002/INV-6, DD-8): two concurrent creations of the same name: the unique index decides, with no lock.
func TestCreateCategoryConcurrentlyWithTheSameName(t *testing.T) {
	e := newCategoriesEnv(t)
	errs := make([]error, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, errs[i] = e.svc.CreateCategory(context.Background(), e.co.Admin, "Peajes", "expense", e.meta)
		}()
	}
	close(start)
	wg.Wait()
	var ok, taken int
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, settings.ErrNameTaken):
			taken++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if ok != 1 || taken != 1 {
		t.Fatalf("errors = %v, want one ok and one ErrNameTaken", errs)
	}
	if got := len(readAudit(t, e.co.ID, "category.created")); got != 1 {
		t.Fatalf("category.created rows = %d, want 1", got)
	}
}

const lockCategorySQL = `SELECT 1 FROM app.categories WHERE tenant_id = $1 AND id = $2 FOR NO KEY UPDATE`

// T-002-B501 (002/DD-8, DD-15, INV-13, mechanism E): two concurrent archivings of the same category leave one
// audit row: the second one finds it archived. Mutation: reading the category without LockCategory's lock lets
// both pass the "active" check and audit twice.
func TestArchiveCategoryConcurrentlyAuditsOnce(t *testing.T) {
	e := newCategoriesEnv(t)
	c := e.create("Peajes", "expense")
	results := make([]settings.Category, 2)
	errs := make([]error, 2)
	call := func(i int) func(context.Context) error {
		return func(ctx context.Context) error {
			results[i], errs[i] = e.svc.ArchiveCategory(ctx, e.co.Admin, c.ID, e.meta)
			return nil
		}
	}
	holdRowLockThen(t, e.co.ID, lockCategorySQL, []any{e.co.ID, c.ID}, nil, call(0), call(1))
	for i := range errs {
		if errs[i] != nil || !results[i].Archived {
			t.Fatalf("call %d: %+v, %v", i, results[i], errs[i])
		}
	}
	if got := len(readAudit(t, e.co.ID, "category.archived")); got != 1 {
		t.Fatalf("category.archived rows = %d, want 1", got)
	}
}

// Codex review of PR #63 (002/DD-5), applied to categories: a write stamps updated_at with the instant it holds
// the category lock, not the instant it started waiting. Mutation: reading the clock before LockCategory.
func TestCategoryWriteStampsTheInstantItHoldsTheLock(t *testing.T) {
	e := newCategoriesEnv(t)
	c := e.create("Peajes", "expense")
	waited := e.clk.Now()
	var errRename error
	holdRowLockThen(t, e.co.ID, lockCategorySQL, []any{e.co.ID, c.ID}, func() { e.clk.Advance(time.Hour) },
		func(ctx context.Context) error {
			_, errRename = e.svc.RenameCategory(ctx, e.co.Admin, c.ID, "Peajes y cocheras", e.meta)
			return nil
		},
	)
	if errRename != nil {
		t.Fatal(errRename)
	}
	if want, got := waited.Add(time.Hour), categoryUpdatedAt(t, e.co.ID, c.ID); !got.Equal(want) {
		t.Fatalf("updated_at = %s, want %s (the instant under the lock, not %s)", got.UTC(), want, waited)
	}
}

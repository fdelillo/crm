//go:build integration

package settings_test

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/settings"
	"github.com/google/uuid"
)

// catalogEnv is a company seeded with the "generic" template, which brings no catalog items: the tests look
// items up by id or by their own names, never by position, so they also hold for a template that seeds some.
type catalogEnv struct {
	t    *testing.T
	clk  *testClock
	svc  *settings.Service
	co   company
	meta settings.RequestMeta
}

func newCatalogEnv(t *testing.T) *catalogEnv {
	t.Helper()
	clk := newTestClock()
	return &catalogEnv{t: t, clk: clk, svc: newService(t, clk), co: newCompany(t, clk, "generic"),
		meta: settings.RequestMeta{IP: netip.MustParseAddr("192.0.2.30"), UserAgent: "catalog-test/1.0"}}
}

func (e *catalogEnv) create(in settings.NewCatalogItem) settings.CatalogItem {
	e.t.Helper()
	got, err := e.svc.CreateCatalogItem(context.Background(), e.co.Admin, in, e.meta)
	if err != nil {
		e.t.Fatalf("CreateCatalogItem(%q): %v", in.Name, err)
	}
	return got
}

func (e *catalogEnv) get(id uuid.UUID) settings.CatalogItem {
	e.t.Helper()
	got, err := e.svc.GetCatalogItem(context.Background(), e.co.Admin, id)
	if err != nil {
		e.t.Fatalf("GetCatalogItem: %v", err)
	}
	return got
}

func (e *catalogEnv) update(id uuid.UUID, ch settings.CatalogItemChanges) (settings.CatalogItem, error) {
	return e.svc.UpdateCatalogItem(context.Background(), e.co.Admin, id, ch, e.meta)
}

func (e *catalogEnv) mustUpdate(id uuid.UUID, ch settings.CatalogItemChanges) settings.CatalogItem {
	e.t.Helper()
	got, err := e.update(id, ch)
	if err != nil {
		e.t.Fatalf("UpdateCatalogItem: %v", err)
	}
	return got
}

func (e *catalogEnv) listIDs(includeArchived bool) []uuid.UUID {
	e.t.Helper()
	items, err := e.svc.ListCatalogItems(context.Background(), e.co.Admin, includeArchived)
	if err != nil {
		e.t.Fatalf("ListCatalogItems: %v", err)
	}
	ids := make([]uuid.UUID, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	return ids
}

func containsID(ids []uuid.UUID, id uuid.UUID) bool {
	for _, got := range ids {
		if got == id {
			return true
		}
	}
	return false
}

// assertNothingChanged runs op after the clock moved and checks that no catalog row and no audit row changed:
// an empty operation writes nothing (002/INV-13).
func (e *catalogEnv) assertNothingChanged(op func()) {
	e.t.Helper()
	before, auditBefore := storedCatalogItems(e.t, e.co.ID), readAudit(e.t, e.co.ID, "catalog_item.")
	e.clk.Advance(time.Hour)
	op()
	if after := storedCatalogItems(e.t, e.co.ID); after != before {
		e.t.Fatalf("catalog changed:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if auditAfter := readAudit(e.t, e.co.ID, "catalog_item."); len(auditAfter) != len(auditBefore) {
		e.t.Fatalf("audit rows: %d before, %d after", len(auditBefore), len(auditAfter))
	}
}

// wantAudit checks that the audit log has exactly one more catalog_item row than before, with that action.
func (e *catalogEnv) wantAudit(before []auditRow, action string, target uuid.UUID, data map[string]any) {
	e.t.Helper()
	after := readAudit(e.t, e.co.ID, "catalog_item.")
	if len(after) != len(before)+1 {
		e.t.Fatalf("audit rows: %d before, %d after, want one more", len(before), len(after))
	}
	row := after[len(after)-1]
	if row.Action != action || row.TargetType != "catalog_item" || row.TargetID == nil || *row.TargetID != target ||
		row.ActorID == nil || *row.ActorID != e.co.Admin.UserID || row.UserAgent != e.meta.UserAgent ||
		row.IP == nil || *row.IP != e.meta.IP {
		e.t.Fatalf("audit row = %+v, want %s on %s", row, action, target)
	}
	equal(e.t, "audit data", row.Data, data)
}

func (e *catalogEnv) audit() []auditRow { return readAudit(e.t, e.co.ID, "catalog_item.") }

// storedCatalogItems reads every row of app.catalog_items of the company with every column an operation may touch.
func storedCatalogItems(t *testing.T, tenantID uuid.UUID) string {
	t.Helper()
	var lines []string
	err := testRunner(t).InTenantTx(context.Background(), tenantID, func(ctx context.Context, tx db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id, name, coalesce(description, '<nil>'), unit, coalesce(unit_label, '<nil>'),
				coalesce(reference_price_cents::text, '<nil>'), coalesce(reference_price_currency, '<nil>'),
				archived_at, created_at, updated_at
			FROM app.catalog_items WHERE tenant_id = $1 ORDER BY created_at, id`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id uuid.UUID
			var name, description, unit, label, cents, currency string
			var archived *time.Time
			var created, updated time.Time
			if err := rows.Scan(&id, &name, &description, &unit, &label, &cents, &currency, &archived, &created, &updated); err != nil {
				return err
			}
			lines = append(lines, fmt.Sprintf("%s|%s|%s|%s|%s|%s %s|%v|%s|%s", id, name, description, unit, label, cents, currency,
				archived, created.UTC(), updated.UTC()))
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}
	return strings.Join(lines, "\n")
}

func catalogItemUpdatedAt(t *testing.T, tenantID, id uuid.UUID) time.Time {
	t.Helper()
	var updated time.Time
	err := testRunner(t).InTenantTx(context.Background(), tenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT updated_at FROM app.catalog_items WHERE tenant_id = $1 AND id = $2`, tenantID, id).Scan(&updated)
	})
	if err != nil {
		t.Fatal(err)
	}
	return updated
}

// storedPrice reads the raw price columns: the service result alone would not prove what was stored.
func storedPrice(t *testing.T, tenantID, id uuid.UUID) (cents *int64, currency *string) {
	t.Helper()
	err := testRunner(t).InTenantTx(context.Background(), tenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT reference_price_cents, reference_price_currency FROM app.catalog_items WHERE tenant_id = $1 AND id = $2`,
			tenantID, id).Scan(&cents, &currency)
	})
	if err != nil {
		t.Fatal(err)
	}
	return cents, currency
}

func present[T any](v T) settings.Patch[T] { return settings.Patch[T]{Set: true, Value: v} }

// T-002-B601 (Historia 4 (1), FR-005, 002/DD-9, P-9, INV-7, ADR-029)
func TestCreateCatalogItem(t *testing.T) {
	ctx := context.Background()
	e := newCatalogEnv(t)

	t.Run("without price", func(t *testing.T) {
		before := e.audit()
		got := e.create(settings.NewCatalogItem{Name: "  Portón corredizo ", Unit: "square_meter"})
		if got.ID == uuid.Nil || got.Name != "Portón corredizo" || got.Unit != "square_meter" || got.ReferencePrice != nil ||
			got.Description != nil || got.UnitLabel != nil || got.Archived {
			t.Fatalf("item = %+v", got)
		}
		if !got.CreatedAt.Equal(e.clk.Now()) || !got.UpdatedAt.Equal(e.clk.Now()) {
			t.Fatalf("timestamps = %s, %s, want %s", got.CreatedAt, got.UpdatedAt, e.clk.Now())
		}
		equal(t, "stored", e.get(got.ID), got)
		cents, currency := storedPrice(t, e.co.ID, got.ID)
		if cents != nil || currency != nil {
			t.Fatalf("stored price = %v %v, want none", cents, currency)
		}
		e.wantAudit(before, "catalog_item.created", got.ID, map[string]any{"unit": "square_meter"})
	})

	t.Run("with a price, stored exactly", func(t *testing.T) {
		got := e.create(settings.NewCatalogItem{Name: "Reja", Unit: "unit", ReferencePrice: &settings.Money{AmountCents: 123456, Currency: "USD"}})
		equal(t, "price", got.ReferencePrice, &settings.Money{AmountCents: 123456, Currency: "USD"})
		cents, currency := storedPrice(t, e.co.ID, got.ID)
		if cents == nil || *cents != 123456 || currency == nil || *currency != "USD" {
			t.Fatalf("stored price = %v %v", cents, currency)
		}
	})

	t.Run("the largest amount fits, 2^53 does not", func(t *testing.T) {
		got := e.create(settings.NewCatalogItem{Name: "Máximo", Unit: "unit", ReferencePrice: &settings.Money{AmountCents: settings.MaxAmountCents, Currency: "ARS"}})
		cents, _ := storedPrice(t, e.co.ID, got.ID)
		if cents == nil || *cents != 9007199254740991 {
			t.Fatalf("stored cents = %v", cents)
		}
		e.assertNothingChanged(func() {
			_, err := e.svc.CreateCatalogItem(ctx, e.co.Admin, settings.NewCatalogItem{Name: "Demasiado", Unit: "unit",
				ReferencePrice: &settings.Money{AmountCents: 9007199254740992, Currency: "ARS"}}, e.meta)
			wantFieldError(t, err, "reference_price.amount_cents", "invalid_value")
		})
	})

	t.Run("a price of zero is a price", func(t *testing.T) {
		got := e.create(settings.NewCatalogItem{Name: "Cortesía", Unit: "unit", ReferencePrice: &settings.Money{AmountCents: 0, Currency: "ARS"}})
		equal(t, "price", got.ReferencePrice, &settings.Money{AmountCents: 0, Currency: "ARS"})
		cents, currency := storedPrice(t, e.co.ID, got.ID)
		if cents == nil || *cents != 0 || currency == nil || *currency != "ARS" {
			t.Fatalf("stored price = %v %v, want 0 ARS", cents, currency)
		}
	})

	t.Run("other needs a label, another unit discards it", func(t *testing.T) {
		e.assertNothingChanged(func() {
			_, err := e.svc.CreateCatalogItem(ctx, e.co.Admin, settings.NewCatalogItem{Name: "Flete", Unit: "other"}, e.meta)
			wantFieldError(t, err, "unit_label", "required")
		})
		hour := e.create(settings.NewCatalogItem{Name: "Mano de obra", Unit: "hour", UnitLabel: ptr("jornada")})
		if hour.UnitLabel != nil {
			t.Fatalf("hour kept its label %q", *hour.UnitLabel)
		}
		other := e.create(settings.NewCatalogItem{Name: "Flete", Unit: "other", UnitLabel: ptr(" viaje ")})
		equal(t, "label", other.UnitLabel, ptr("viaje"))
		equal(t, "stored", e.get(other.ID), other)
	})

	t.Run("an empty description is no description", func(t *testing.T) {
		got := e.create(settings.NewCatalogItem{Name: "Sin detalle", Unit: "unit", Description: ptr("")})
		if got.Description != nil {
			t.Fatalf("description = %q, want nil", *got.Description)
		}
		with := e.create(settings.NewCatalogItem{Name: "Con detalle", Unit: "unit", Description: ptr("Chapa calibre 18")})
		equal(t, "description", with.Description, ptr("Chapa calibre 18"))
	})

	t.Run("two items may share a name", func(t *testing.T) {
		a := e.create(settings.NewCatalogItem{Name: "Duplicado", Unit: "unit"})
		b := e.create(settings.NewCatalogItem{Name: "duplicado", Unit: "unit"})
		if a.ID == b.ID {
			t.Fatal("same id")
		}
	})

	t.Run("invalid fields are all reported and nothing is written", func(t *testing.T) {
		e.assertNothingChanged(func() {
			_, err := e.svc.CreateCatalogItem(ctx, e.co.Admin, settings.NewCatalogItem{Name: " ", Unit: "kg",
				ReferencePrice: &settings.Money{AmountCents: -1, Currency: "EUR"}}, e.meta)
			var fields settings.FieldErrors
			if !errors.As(err, &fields) {
				t.Fatalf("error = %v", err)
			}
			equal(t, "fields", map[string]string(fields), map[string]string{"name": "required", "unit": "invalid_value",
				"reference_price.amount_cents": "invalid_value", "reference_price.currency": "invalid_value"})
		})
	})

	t.Run("an operator cannot", func(t *testing.T) {
		e.assertNothingChanged(func() {
			_, err := e.svc.CreateCatalogItem(ctx, e.co.Operator, settings.NewCatalogItem{Name: "Reja", Unit: "unit"}, e.meta)
			if !errors.Is(err, authz.ErrForbidden) {
				t.Fatalf("error = %v, want ErrForbidden", err)
			}
		})
	})
}

// T-002-B601 (Historia 4 (1), FR-005, 002/DD-9, DD-16, DD-11, INV-3)
func TestListAndGetCatalogItems(t *testing.T) {
	ctx := context.Background()
	e := newCatalogEnv(t)
	b := e.create(settings.NewCatalogItem{Name: "bbb", Unit: "unit"})
	a := e.create(settings.NewCatalogItem{Name: "Aaa", Unit: "unit"})
	c := e.create(settings.NewCatalogItem{Name: "ccc", Unit: "unit"})
	if _, err := e.svc.ArchiveCatalogItem(ctx, e.co.Admin, c.ID, e.meta); err != nil {
		t.Fatal(err)
	}

	t.Run("the default list leaves archived out and sorts by name ignoring case", func(t *testing.T) {
		ids := e.listIDs(false)
		if containsID(ids, c.ID) {
			t.Fatal("the archived item is in the default list")
		}
		ia, ib := indexOf(ids, a.ID), indexOf(ids, b.ID)
		if ia < 0 || ib < 0 || ia > ib {
			t.Fatalf("order = %v, want Aaa before bbb", ids)
		}
	})
	t.Run("include_archived brings them back", func(t *testing.T) {
		if !containsID(e.listIDs(true), c.ID) {
			t.Fatal("the archived item is missing")
		}
	})
	t.Run("the list is the same for the same name ordered by id", func(t *testing.T) {
		x := e.create(settings.NewCatalogItem{Name: "Zzz", Unit: "unit"})
		y := e.create(settings.NewCatalogItem{Name: "zzz", Unit: "unit"})
		ids := e.listIDs(false)
		first, second := x.ID, y.ID
		if second.String() < first.String() {
			first, second = second, first
		}
		if indexOf(ids, first) > indexOf(ids, second) {
			t.Fatalf("equal names are not ordered by id: %v", ids)
		}
	})
	t.Run("an archived item can be read", func(t *testing.T) {
		if got := e.get(c.ID); !got.Archived {
			t.Fatalf("item = %+v", got)
		}
	})
	t.Run("an operator can read", func(t *testing.T) {
		if _, err := e.svc.ListCatalogItems(ctx, e.co.Operator, false); err != nil {
			t.Fatal(err)
		}
		if _, err := e.svc.GetCatalogItem(ctx, e.co.Operator, a.ID); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("another company's item and an unknown id are not found", func(t *testing.T) {
		other := newCatalogEnv(t)
		foreign := other.create(settings.NewCatalogItem{Name: "Ajeno", Unit: "unit"})
		if _, err := e.svc.GetCatalogItem(ctx, e.co.Admin, foreign.ID); !errors.Is(err, settings.ErrNotFound) {
			t.Fatalf("foreign: %v", err)
		}
		if _, err := e.svc.GetCatalogItem(ctx, e.co.Admin, uuid.New()); !errors.Is(err, settings.ErrNotFound) {
			t.Fatalf("unknown: %v", err)
		}
		if containsID(e.listIDs(true), foreign.ID) {
			t.Fatal("the list shows another company's item")
		}
	})
}

func indexOf(ids []uuid.UUID, id uuid.UUID) int {
	for i, got := range ids {
		if got == id {
			return i
		}
	}
	return -1
}

// T-002-B601 (Historia 4 (3), FR-005, 002/DD-9, DD-15, INV-13, P-9)
func TestUpdateCatalogItem(t *testing.T) {
	ctx := context.Background()
	e := newCatalogEnv(t)
	price := &settings.Money{AmountCents: 450000, Currency: "USD"}
	newItem := func(name string) settings.CatalogItem {
		return e.create(settings.NewCatalogItem{Name: name, Description: ptr("detalle"), Unit: "unit", ReferencePrice: price})
	}

	t.Run("only the name", func(t *testing.T) {
		item := newItem("Portón")
		e.clk.Advance(time.Hour)
		before := e.audit()
		got := e.mustUpdate(item.ID, settings.CatalogItemChanges{Name: present(" Portón corredizo ")})
		want := item
		want.Name = "Portón corredizo"
		want.UpdatedAt = e.clk.Now()
		equal(t, "result", got, want)
		equal(t, "stored", e.get(item.ID), want)
		e.wantAudit(before, "catalog_item.updated", item.ID, map[string]any{"fields": []any{"name"}})
	})

	t.Run("only the price keeps the rest", func(t *testing.T) {
		item := newItem("Reja")
		before := e.audit()
		got := e.mustUpdate(item.ID, settings.CatalogItemChanges{ReferencePrice: present(&settings.Money{AmountCents: 500000, Currency: "ARS"})})
		equal(t, "price", got.ReferencePrice, &settings.Money{AmountCents: 500000, Currency: "ARS"})
		if got.Name != item.Name || got.Unit != item.Unit {
			t.Fatalf("item = %+v", got)
		}
		e.wantAudit(before, "catalog_item.updated", item.ID, map[string]any{"fields": []any{"reference_price"}})
	})

	t.Run("a null description and a null price are erased and named in the audit", func(t *testing.T) {
		item := newItem("Ventana")
		before := e.audit()
		got := e.mustUpdate(item.ID, settings.CatalogItemChanges{Description: present[*string](nil), ReferencePrice: present[*settings.Money](nil)})
		if got.Description != nil || got.ReferencePrice != nil {
			t.Fatalf("item = %+v", got)
		}
		cents, currency := storedPrice(t, e.co.ID, item.ID)
		if cents != nil || currency != nil {
			t.Fatalf("stored price = %v %v", cents, currency)
		}
		e.wantAudit(before, "catalog_item.updated", item.ID, map[string]any{"fields": []any{"description", "reference_price"}})
	})

	t.Run("an empty description is no description", func(t *testing.T) {
		item := newItem("Puerta")
		got := e.mustUpdate(item.ID, settings.CatalogItemChanges{Description: present(ptr("  "))})
		if got.Description != nil {
			t.Fatalf("description = %q", *got.Description)
		}
	})

	t.Run("a price of zero is a price, not no price", func(t *testing.T) {
		item := newItem("Cortesía")
		got := e.mustUpdate(item.ID, settings.CatalogItemChanges{ReferencePrice: present(&settings.Money{AmountCents: 0, Currency: "ARS"})})
		equal(t, "price", got.ReferencePrice, &settings.Money{AmountCents: 0, Currency: "ARS"})
		cents, currency := storedPrice(t, e.co.ID, item.ID)
		if cents == nil || *cents != 0 || currency == nil || *currency != "ARS" {
			t.Fatalf("stored price = %v %v, want 0 ARS", cents, currency)
		}
	})

	t.Run("a price over 2^53-1 is invalid and writes nothing", func(t *testing.T) {
		item := newItem("Caro")
		e.assertNothingChanged(func() {
			_, err := e.update(item.ID, settings.CatalogItemChanges{ReferencePrice: present(&settings.Money{AmountCents: 9007199254740992, Currency: "ARS"})})
			wantFieldError(t, err, "reference_price.amount_cents", "invalid_value")
		})
	})

	t.Run("from other to another unit the label is erased", func(t *testing.T) {
		item := e.create(settings.NewCatalogItem{Name: "Flete", Unit: "other", UnitLabel: ptr("viaje")})
		before := e.audit()
		got := e.mustUpdate(item.ID, settings.CatalogItemChanges{Unit: present(settings.Unit("hour"))})
		if got.Unit != "hour" || got.UnitLabel != nil {
			t.Fatalf("item = %+v", got)
		}
		equal(t, "stored", e.get(item.ID), got)
		e.wantAudit(before, "catalog_item.updated", item.ID, map[string]any{"fields": []any{"unit", "unit_label"}})
	})

	t.Run("from another unit to other needs the label", func(t *testing.T) {
		item := newItem("Mano de obra")
		e.assertNothingChanged(func() {
			_, err := e.update(item.ID, settings.CatalogItemChanges{Unit: present(settings.Unit("other"))})
			wantFieldError(t, err, "unit_label", "required")
		})
		before := e.audit()
		got := e.mustUpdate(item.ID, settings.CatalogItemChanges{Unit: present(settings.Unit("other")), UnitLabel: present(ptr(" jornada "))})
		equal(t, "label", got.UnitLabel, ptr("jornada"))
		e.wantAudit(before, "catalog_item.updated", item.ID, map[string]any{"fields": []any{"unit", "unit_label"}})
	})

	t.Run("with unit other the label cannot be erased but can change", func(t *testing.T) {
		item := e.create(settings.NewCatalogItem{Name: "Traslado", Unit: "other", UnitLabel: ptr("viaje")})
		e.assertNothingChanged(func() {
			_, err := e.update(item.ID, settings.CatalogItemChanges{UnitLabel: present[*string](nil)})
			wantFieldError(t, err, "unit_label", "required")
		})
		before := e.audit()
		got := e.mustUpdate(item.ID, settings.CatalogItemChanges{UnitLabel: present(ptr("flete"))})
		equal(t, "label", got.UnitLabel, ptr("flete"))
		e.wantAudit(before, "catalog_item.updated", item.ID, map[string]any{"fields": []any{"unit_label"}})
	})

	t.Run("a label sent for a unit that has none is discarded: no change", func(t *testing.T) {
		item := newItem("Sin etiqueta")
		e.assertNothingChanged(func() {
			got := e.mustUpdate(item.ID, settings.CatalogItemChanges{UnitLabel: present(ptr("x"))})
			if got.UnitLabel != nil {
				t.Fatalf("label = %q", *got.UnitLabel)
			}
		})
	})

	t.Run("every audited field comes in the catalog order", func(t *testing.T) {
		item := newItem("Orden")
		before := e.audit()
		e.mustUpdate(item.ID, settings.CatalogItemChanges{
			ReferencePrice: present[*settings.Money](nil), UnitLabel: present(ptr("hora")), Unit: present(settings.Unit("other")),
			Description: present[*string](nil), Name: present("Orden nuevo"),
		})
		e.wantAudit(before, "catalog_item.updated", item.ID,
			map[string]any{"fields": []any{"name", "description", "unit", "unit_label", "reference_price"}})
	})

	t.Run("invalid values are all reported and nothing is written", func(t *testing.T) {
		item := newItem("Inválido")
		e.assertNothingChanged(func() {
			_, err := e.update(item.ID, settings.CatalogItemChanges{Name: present(" "), Unit: present(settings.Unit("kg")),
				ReferencePrice: present(&settings.Money{AmountCents: 1, Currency: ""})})
			var fields settings.FieldErrors
			if !errors.As(err, &fields) {
				t.Fatalf("error = %v", err)
			}
			equal(t, "fields", map[string]string(fields), map[string]string{"name": "required", "unit": "invalid_value", "reference_price.currency": "required"})
		})
	})

	// Every no-op has its own case: archived or not, the same values change nothing.
	t.Run("no-op: the current values", func(t *testing.T) {
		item := newItem("Igual")
		e.assertNothingChanged(func() {
			got := e.mustUpdate(item.ID, settings.CatalogItemChanges{Name: present(" Igual "), Description: present(ptr("detalle")),
				Unit: present(settings.Unit("unit")), ReferencePrice: present(&settings.Money{AmountCents: 450000, Currency: "USD"})})
			equal(t, "result", got, item)
		})
	})
	t.Run("no-op: erasing what is already empty", func(t *testing.T) {
		item := e.create(settings.NewCatalogItem{Name: "Vacío", Unit: "unit"})
		e.assertNothingChanged(func() {
			e.mustUpdate(item.ID, settings.CatalogItemChanges{Description: present[*string](nil), ReferencePrice: present[*settings.Money](nil)})
		})
	})
	t.Run("no-op: the same label on an item with unit other", func(t *testing.T) {
		item := e.create(settings.NewCatalogItem{Name: "Otra", Unit: "other", UnitLabel: ptr("viaje")})
		e.assertNothingChanged(func() {
			e.mustUpdate(item.ID, settings.CatalogItemChanges{Unit: present(settings.Unit("other")), UnitLabel: present(ptr("viaje"))})
		})
	})

	t.Run("no field at all is malformed", func(t *testing.T) {
		item := newItem("Vacía")
		e.assertNothingChanged(func() {
			if _, err := e.update(item.ID, settings.CatalogItemChanges{}); !errors.Is(err, settings.ErrMalformedChanges) {
				t.Fatalf("error = %v", err)
			}
		})
	})

	t.Run("an archived item can be edited and stays archived", func(t *testing.T) {
		item := newItem("Archivado")
		if _, err := e.svc.ArchiveCatalogItem(ctx, e.co.Admin, item.ID, e.meta); err != nil {
			t.Fatal(err)
		}
		got := e.mustUpdate(item.ID, settings.CatalogItemChanges{Name: present("Archivado y editado")})
		if got.Name != "Archivado y editado" || !got.Archived {
			t.Fatalf("item = %+v", got)
		}
	})

	t.Run("another company's item and an unknown id are not found", func(t *testing.T) {
		foreign := newCatalogEnv(t).create(settings.NewCatalogItem{Name: "Ajeno", Unit: "unit"})
		e.assertNothingChanged(func() {
			for _, id := range []uuid.UUID{foreign.ID, uuid.New()} {
				if _, err := e.update(id, settings.CatalogItemChanges{Name: present("x")}); !errors.Is(err, settings.ErrNotFound) {
					t.Fatalf("%s: %v", id, err)
				}
			}
		})
	})

	t.Run("an operator cannot", func(t *testing.T) {
		item := newItem("Operador")
		e.assertNothingChanged(func() {
			_, err := e.svc.UpdateCatalogItem(ctx, e.co.Operator, item.ID, settings.CatalogItemChanges{Name: present("x")}, e.meta)
			if !errors.Is(err, authz.ErrForbidden) {
				t.Fatalf("error = %v", err)
			}
		})
	})
}

// T-002-B601 (Historia 4 (3), 002/DD-9, DD-15, INV-13)
func TestArchiveAndRestoreCatalogItem(t *testing.T) {
	ctx := context.Background()
	e := newCatalogEnv(t)
	item := e.create(settings.NewCatalogItem{Name: "Reja", Unit: "unit", ReferencePrice: &settings.Money{AmountCents: 100, Currency: "ARS"}})

	t.Run("restoring an active item writes and audits nothing", func(t *testing.T) {
		e.assertNothingChanged(func() {
			got, err := e.svc.RestoreCatalogItem(ctx, e.co.Admin, item.ID, e.meta)
			if err != nil || got.Archived {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	})

	t.Run("archive", func(t *testing.T) {
		e.clk.Advance(time.Hour)
		before := e.audit()
		got, err := e.svc.ArchiveCatalogItem(ctx, e.co.Admin, item.ID, e.meta)
		if err != nil || !got.Archived {
			t.Fatalf("got %+v, %v", got, err)
		}
		equal(t, "stored", e.get(item.ID), got)
		if !got.UpdatedAt.Equal(e.clk.Now()) {
			t.Fatalf("updated_at = %s, want %s", got.UpdatedAt, e.clk.Now())
		}
		e.wantAudit(before, "catalog_item.archived", item.ID, map[string]any{})
		if containsID(e.listIDs(false), item.ID) {
			t.Fatal("still in the default list")
		}
	})

	t.Run("archiving again writes and audits nothing", func(t *testing.T) {
		e.assertNothingChanged(func() {
			got, err := e.svc.ArchiveCatalogItem(ctx, e.co.Admin, item.ID, e.meta)
			if err != nil || !got.Archived {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	})

	t.Run("restore", func(t *testing.T) {
		e.clk.Advance(time.Hour)
		before := e.audit()
		got, err := e.svc.RestoreCatalogItem(ctx, e.co.Admin, item.ID, e.meta)
		if err != nil || got.Archived {
			t.Fatalf("got %+v, %v", got, err)
		}
		equal(t, "stored", e.get(item.ID), got)
		e.wantAudit(before, "catalog_item.restored", item.ID, map[string]any{})
		if !containsID(e.listIDs(false), item.ID) {
			t.Fatal("not back in the default list")
		}
	})

	t.Run("restoring again writes and audits nothing", func(t *testing.T) {
		e.assertNothingChanged(func() {
			if got, err := e.svc.RestoreCatalogItem(ctx, e.co.Admin, item.ID, e.meta); err != nil || got.Archived {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	})

	t.Run("another company's item and an unknown id are not found", func(t *testing.T) {
		foreign := newCatalogEnv(t).create(settings.NewCatalogItem{Name: "Ajeno", Unit: "unit"})
		for _, id := range []uuid.UUID{foreign.ID, uuid.New()} {
			if _, err := e.svc.ArchiveCatalogItem(ctx, e.co.Admin, id, e.meta); !errors.Is(err, settings.ErrNotFound) {
				t.Fatalf("archive %s: %v", id, err)
			}
			if _, err := e.svc.RestoreCatalogItem(ctx, e.co.Admin, id, e.meta); !errors.Is(err, settings.ErrNotFound) {
				t.Fatalf("restore %s: %v", id, err)
			}
		}
	})

	t.Run("an operator cannot", func(t *testing.T) {
		e.assertNothingChanged(func() {
			if _, err := e.svc.ArchiveCatalogItem(ctx, e.co.Operator, item.ID, e.meta); !errors.Is(err, authz.ErrForbidden) {
				t.Fatalf("archive: %v", err)
			}
			if _, err := e.svc.RestoreCatalogItem(ctx, e.co.Operator, item.ID, e.meta); !errors.Is(err, authz.ErrForbidden) {
				t.Fatalf("restore: %v", err)
			}
		})
	})
}

const lockCatalogItemSQL = `SELECT 1 FROM app.catalog_items WHERE tenant_id = $1 AND id = $2 FOR NO KEY UPDATE`

// T-002-B601 (002/DD-9, DD-15, INV-13, mechanism E): two concurrent updates, one of the name and one of the price,
// keep both changes and audit twice. Mutation: reading the item without LockCatalogItem's lock and rewriting every
// column lets the second write undo the first.
func TestUpdateCatalogItemConcurrentlyKeepsBothChanges(t *testing.T) {
	e := newCatalogEnv(t)
	item := e.create(settings.NewCatalogItem{Name: "Portón", Unit: "unit", ReferencePrice: &settings.Money{AmountCents: 100, Currency: "ARS"}})
	errs := make([]error, 2)
	changes := []settings.CatalogItemChanges{
		{Name: present("Portón corredizo")},
		{ReferencePrice: present(&settings.Money{AmountCents: 250000, Currency: "USD"})},
	}
	call := func(i int) func(context.Context) error {
		return func(ctx context.Context) error {
			_, errs[i] = e.svc.UpdateCatalogItem(ctx, e.co.Admin, item.ID, changes[i], e.meta)
			return nil
		}
	}
	holdRowLockThen(t, e.co.ID, lockCatalogItemSQL, []any{e.co.ID, item.ID}, nil, call(0), call(1))
	for i, err := range errs {
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	got := e.get(item.ID)
	if got.Name != "Portón corredizo" {
		t.Fatalf("name = %q, the price update undid it", got.Name)
	}
	equal(t, "price", got.ReferencePrice, &settings.Money{AmountCents: 250000, Currency: "USD"})
	if n := len(readAudit(t, e.co.ID, "catalog_item.updated")); n != 2 {
		t.Fatalf("catalog_item.updated rows = %d, want 2", n)
	}
}

// Codex review of PR #68 (002/DD-9, DD-15, INV-13, mechanism E): the unit_label rule depends on the unit, so two
// concurrent updates that touch each of them must serialize. From unit "other" with a label, one call changes the
// unit to "hour" and the other only the label. Whichever goes first, the final item is "hour" without a label (a
// label on "hour" is the state a lost update would leave) and the audit tells one of two coherent stories: the
// unit change alone (the label change then found nothing to do on "hour"), or the label change followed by the
// unit change. Mutation: GetCatalogItem instead of LockCatalogItem in writeCatalogItem.
func TestUpdateCatalogItemUnitAndLabelConcurrentlyNeverLeaveLabelOnHour(t *testing.T) {
	e := newCatalogEnv(t)
	label := "jornal"
	item := e.create(settings.NewCatalogItem{Name: "Colocación", Unit: "other", UnitLabel: &label})
	newLabel := "día"
	changes := []settings.CatalogItemChanges{
		{Unit: present(settings.Unit("hour"))},
		{UnitLabel: present(&newLabel)},
	}
	errs := make([]error, 2)
	call := func(i int) func(context.Context) error {
		return func(ctx context.Context) error {
			_, errs[i] = e.svc.UpdateCatalogItem(ctx, e.co.Admin, item.ID, changes[i], e.meta)
			return nil
		}
	}
	holdRowLockThen(t, e.co.ID, lockCatalogItemSQL, []any{e.co.ID, item.ID}, nil, call(0), call(1))
	for i, err := range errs {
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	got := e.get(item.ID)
	if got.Unit != "hour" || got.UnitLabel != nil {
		t.Fatalf("item = unit %q label %v, want hour without a label", got.Unit, got.UnitLabel)
	}
	var stories []string
	for _, row := range readAudit(t, e.co.ID, "catalog_item.updated") {
		stories = append(stories, fmt.Sprint(row.Data["fields"]))
	}
	slices.Sort(stories)
	unitFirst := []string{"[unit unit_label]"}
	labelFirst := []string{"[unit unit_label]", "[unit_label]"}
	if !slices.Equal(stories, unitFirst) && !slices.Equal(stories, labelFirst) {
		t.Fatalf("audited fields = %v, want %v (unit first) or %v (label first)", stories, unitFirst, labelFirst)
	}
}

// T-002-B601 (002/DD-9, DD-15, INV-13, mechanism E): two concurrent archivings leave one audit row.
func TestArchiveCatalogItemConcurrentlyAuditsOnce(t *testing.T) {
	e := newCatalogEnv(t)
	item := e.create(settings.NewCatalogItem{Name: "Reja", Unit: "unit"})
	errs := make([]error, 2)
	results := make([]settings.CatalogItem, 2)
	call := func(i int) func(context.Context) error {
		return func(ctx context.Context) error {
			results[i], errs[i] = e.svc.ArchiveCatalogItem(ctx, e.co.Admin, item.ID, e.meta)
			return nil
		}
	}
	holdRowLockThen(t, e.co.ID, lockCatalogItemSQL, []any{e.co.ID, item.ID}, nil, call(0), call(1))
	for i := range errs {
		if errs[i] != nil || !results[i].Archived {
			t.Fatalf("call %d: %+v, %v", i, results[i], errs[i])
		}
	}
	if n := len(readAudit(t, e.co.ID, "catalog_item.archived")); n != 1 {
		t.Fatalf("catalog_item.archived rows = %d, want 1", n)
	}
}

// Codex review of PR #63 (002/DD-5), applied to the catalog: a write stamps updated_at with the instant it holds
// the item lock, not the instant it started waiting. Mutation: reading the clock before LockCatalogItem.
func TestCatalogItemWriteStampsTheInstantItHoldsTheLock(t *testing.T) {
	e := newCatalogEnv(t)
	item := e.create(settings.NewCatalogItem{Name: "Reja", Unit: "unit"})
	waited := e.clk.Now()
	var errUpdate error
	holdRowLockThen(t, e.co.ID, lockCatalogItemSQL, []any{e.co.ID, item.ID}, func() { e.clk.Advance(time.Hour) },
		func(ctx context.Context) error {
			_, errUpdate = e.svc.UpdateCatalogItem(ctx, e.co.Admin, item.ID, settings.CatalogItemChanges{Name: present("Reja doble")}, e.meta)
			return nil
		},
	)
	if errUpdate != nil {
		t.Fatal(errUpdate)
	}
	if want, got := waited.Add(time.Hour), catalogItemUpdatedAt(t, e.co.ID, item.ID); !got.Equal(want) {
		t.Fatalf("updated_at = %s, want %s (the instant under the lock, not %s)", got.UTC(), want, waited)
	}
}

//go:build integration

package settings_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/settings"
	"github.com/google/uuid"
)

var (
	genericLabels = settings.Labels{
		Project:  settings.EntityLabel{Singular: "Proyecto", Plural: "Proyectos", Gender: "masculine"},
		Customer: settings.EntityLabel{Singular: "Cliente", Plural: "Clientes", Gender: "masculine"},
		Quote:    settings.EntityLabel{Singular: "Presupuesto", Plural: "Presupuestos", Gender: "masculine"},
	}
	aluminumTerms = "Seña del 50 % al aprobar el presupuesto y saldo contra entrega."
)

// storedSettings reads the settings row with every column a write may touch: two equal snapshots mean "nothing changed".
func storedSettings(t *testing.T, tenantID uuid.UUID) string {
	t.Helper()
	var out string
	err := testRunner(t).InTenantTx(context.Background(), tenantID, func(ctx context.Context, tx db.Tx) error {
		var line string
		err := tx.QueryRow(ctx, `SELECT concat_ws('|', project_label_singular, project_label_plural, project_label_gender,
				customer_label_singular, customer_label_plural, customer_label_gender,
				quote_label_singular, quote_label_plural, quote_label_gender, quote_validity_days,
				coalesce(quote_terms, '<null>'), created_at, updated_at)
			FROM app.tenant_settings WHERE tenant_id = $1`, tenantID).Scan(&line)
		out = line
		return err
	})
	if err != nil {
		t.Fatalf("read settings row: %v", err)
	}
	return out
}

// assertSettingsUntouched runs op and checks that the settings row and the settings audit rows did not change, even
// after the clock moved: a rejected or empty write writes nothing (002/INV-13).
func (e *stagesEnv) assertSettingsUntouched(op func()) {
	e.t.Helper()
	before, auditBefore := storedSettings(e.t, e.co.ID), readAudit(e.t, e.co.ID, "settings.")
	e.clk.Advance(time.Hour)
	op()
	if after := storedSettings(e.t, e.co.ID); after != before {
		e.t.Fatalf("settings row changed:\nbefore: %s\nafter:  %s", before, after)
	}
	if auditAfter := readAudit(e.t, e.co.ID, "settings."); len(auditAfter) != len(auditBefore) {
		e.t.Fatalf("audit rows: %d before, %d after", len(auditBefore), len(auditAfter))
	}
}

// T-002-B701 (Historia 2, 002/DD-10): a new company has the labels of its template; any role reads them.
func TestGetLabelsReturnsTheTemplateDefaults(t *testing.T) {
	e := newStagesEnv(t)
	for name, p := range map[string]authz.Principal{"admin": e.co.Admin, "operator": e.co.Operator} {
		t.Run(name, func(t *testing.T) {
			got, err := e.svc.GetLabels(context.Background(), p)
			if err != nil {
				t.Fatal(err)
			}
			equal(t, "labels", got, genericLabels)
		})
	}
}

// T-002-B701 (Historia 2, 002/DD-10, DD-15): a changed entity is saved and audited by name; the saved labels come back.
func TestReplaceLabelsSavesAndAudits(t *testing.T) {
	e := newStagesEnv(t)
	in := genericLabels
	in.Project = settings.EntityLabel{Singular: " Obra ", Plural: "Obras", Gender: "feminine"}
	got, err := e.svc.ReplaceLabels(context.Background(), e.co.Admin, in, e.meta)
	if err != nil {
		t.Fatal(err)
	}
	want := genericLabels
	want.Project = settings.EntityLabel{Singular: "Obra", Plural: "Obras", Gender: "feminine"}
	equal(t, "returned labels", got, want)
	stored, err := e.svc.GetLabels(context.Background(), e.co.Operator)
	if err != nil {
		t.Fatal(err)
	}
	equal(t, "stored labels", stored, want)

	rows := readAudit(t, e.co.ID, "settings.")
	if len(rows) != 1 {
		t.Fatalf("audit rows = %+v, want one", rows)
	}
	g := rows[0]
	if g.Action != "settings.labels_updated" || g.TargetType != "tenant" || g.TargetID == nil || *g.TargetID != e.co.ID ||
		g.ActorID == nil || *g.ActorID != e.co.Admin.UserID || g.UserAgent != "stages-test/1.0" {
		t.Fatalf("audit row = %+v", g)
	}
	equal(t, "audit data", g.Data, map[string]any{"entities": []any{"project"}})
}

// T-002-B701 (002/DD-15): the entities are listed in the order project, customer, quote, whatever changed first.
func TestReplaceLabelsAuditsEntitiesInOrder(t *testing.T) {
	e := newStagesEnv(t)
	in := genericLabels
	in.Quote.Singular = "Cotización"
	in.Project.Plural = "Trabajos"
	if _, err := e.svc.ReplaceLabels(context.Background(), e.co.Admin, in, e.meta); err != nil {
		t.Fatal(err)
	}
	rows := readAudit(t, e.co.ID, "settings.")
	if len(rows) != 1 {
		t.Fatalf("audit rows = %+v", rows)
	}
	equal(t, "audit data", rows[0].Data, map[string]any{"entities": []any{"project", "quote"}})
}

// T-002-B701 (002/INV-13): the current values, even with spaces around, write and audit nothing.
func TestReplaceLabelsWithCurrentValuesWritesNothing(t *testing.T) {
	e := newStagesEnv(t)
	in := genericLabels
	in.Customer.Singular = "  Cliente "
	e.assertSettingsUntouched(func() {
		got, err := e.svc.ReplaceLabels(context.Background(), e.co.Admin, in, e.meta)
		if err != nil {
			t.Fatal(err)
		}
		equal(t, "labels", got, genericLabels)
	})
}

// T-002-B701 (002/DD-10, P-3): every invalid field is reported and nothing is written.
func TestReplaceLabelsRejectsInvalidFields(t *testing.T) {
	e := newStagesEnv(t)
	tests := []struct {
		name         string
		mutate       func(*settings.Labels)
		field, wantC string
	}{
		{"empty singular", func(l *settings.Labels) { l.Project.Singular = "" }, "project.singular", "required"},
		{"41 characters", func(l *settings.Labels) { l.Project.Singular = fmt.Sprintf("%041d", 0) }, "project.singular", "too_long"},
		{"gender neutral", func(l *settings.Labels) { l.Project.Gender = "neutral" }, "project.gender", "invalid_value"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := genericLabels
			tc.mutate(&in)
			e.assertSettingsUntouched(func() {
				_, err := e.svc.ReplaceLabels(context.Background(), e.co.Admin, in, e.meta)
				wantFieldError(t, err, tc.field, tc.wantC)
			})
		})
	}
}

// T-002-B701 (002/DD-10): the conditions of a new company come from its template.
func TestGetQuoteDefaultsReturnsTheTemplateDefaults(t *testing.T) {
	clk := newTestClock()
	svc := newService(t, clk)
	for _, tc := range []struct {
		template string
		want     settings.QuoteDefaults
	}{
		{"generic", settings.QuoteDefaults{ValidityDays: 15}},
		{"aluminum_carpentry", settings.QuoteDefaults{ValidityDays: 15, Terms: &aluminumTerms}},
	} {
		t.Run(tc.template, func(t *testing.T) {
			co := newCompany(t, clk, tc.template)
			for _, p := range []authz.Principal{co.Admin, co.Operator} {
				got, err := svc.GetQuoteDefaults(context.Background(), p)
				if err != nil {
					t.Fatal(err)
				}
				equal(t, "quote defaults", got, tc.want)
			}
		})
	}
}

// T-002-B701 (Historia 6, 002/DD-10, DD-15): the terms are stored as typed text; both fields changed are audited.
func TestReplaceQuoteDefaultsSavesAndAudits(t *testing.T) {
	e := newStagesEnv(t)
	terms := "Seña del 50 %"
	got, err := e.svc.ReplaceQuoteDefaults(context.Background(), e.co.Admin, settings.QuoteDefaults{ValidityDays: 30, Terms: &terms}, e.meta)
	if err != nil {
		t.Fatal(err)
	}
	want := settings.QuoteDefaults{ValidityDays: 30, Terms: &terms}
	equal(t, "returned", got, want)
	stored, err := e.svc.GetQuoteDefaults(context.Background(), e.co.Operator)
	if err != nil {
		t.Fatal(err)
	}
	equal(t, "stored", stored, want)
	rows := readAudit(t, e.co.ID, "settings.")
	if len(rows) != 1 {
		t.Fatalf("audit rows = %+v", rows)
	}
	g := rows[0]
	if g.Action != "settings.quote_defaults_updated" || g.TargetType != "tenant" || g.TargetID == nil || *g.TargetID != e.co.ID ||
		g.ActorID == nil || *g.ActorID != e.co.Admin.UserID {
		t.Fatalf("audit row = %+v", g)
	}
	equal(t, "audit data", g.Data, map[string]any{"fields": []any{"validity_days", "terms"}})
}

// T-002-B701 (002/DD-15): only the fields that changed are audited.
func TestReplaceQuoteDefaultsAuditsOnlyChangedFields(t *testing.T) {
	e := newStagesEnv(t)
	if _, err := e.svc.ReplaceQuoteDefaults(context.Background(), e.co.Admin, settings.QuoteDefaults{ValidityDays: 45}, e.meta); err != nil {
		t.Fatal(err)
	}
	rows := readAudit(t, e.co.ID, "settings.")
	if len(rows) != 1 {
		t.Fatalf("audit rows = %+v", rows)
	}
	equal(t, "audit data", rows[0].Data, map[string]any{"fields": []any{"validity_days"}})
}

// T-002-B701 (002/P-10): terms with only spaces are no terms; removing the terms of a company that had some is a change.
func TestReplaceQuoteDefaultsBlankTermsAreNil(t *testing.T) {
	clk := newTestClock()
	svc := newService(t, clk)
	co := newCompany(t, clk, "aluminum_carpentry")
	blank := "   "
	got, err := svc.ReplaceQuoteDefaults(context.Background(), co.Admin, settings.QuoteDefaults{ValidityDays: 15, Terms: &blank}, settings.RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	equal(t, "returned", got, settings.QuoteDefaults{ValidityDays: 15})
	stored, err := svc.GetQuoteDefaults(context.Background(), co.Operator)
	if err != nil {
		t.Fatal(err)
	}
	equal(t, "stored", stored, settings.QuoteDefaults{ValidityDays: 15})
	rows := readAudit(t, co.ID, "settings.")
	if len(rows) != 1 {
		t.Fatalf("audit rows = %+v", rows)
	}
	equal(t, "audit data", rows[0].Data, map[string]any{"fields": []any{"terms"}})
}

// T-002-B701 (002/INV-13): the current values write and audit nothing; blank terms over no terms is the same value.
func TestReplaceQuoteDefaultsWithCurrentValuesWritesNothing(t *testing.T) {
	e := newStagesEnv(t)
	blank := "  "
	e.assertSettingsUntouched(func() {
		got, err := e.svc.ReplaceQuoteDefaults(context.Background(), e.co.Admin, settings.QuoteDefaults{ValidityDays: 15, Terms: &blank}, e.meta)
		if err != nil {
			t.Fatal(err)
		}
		equal(t, "quote defaults", got, settings.QuoteDefaults{ValidityDays: 15})
	})
}

// T-002-B701 (002/DD-10, P-10): validity out of range is invalid_value and nothing is written.
func TestReplaceQuoteDefaultsRejectsInvalidValidity(t *testing.T) {
	e := newStagesEnv(t)
	for _, days := range []int{0, 366} {
		t.Run(fmt.Sprint(days), func(t *testing.T) {
			e.assertSettingsUntouched(func() {
				_, err := e.svc.ReplaceQuoteDefaults(context.Background(), e.co.Admin, settings.QuoteDefaults{ValidityDays: days}, e.meta)
				wantFieldError(t, err, "validity_days", "invalid_value")
			})
		})
	}
}

// T-002-B701 (002/DD-11): the writes need the configuration permission; an operator is refused before any change.
func TestPreferenceWritesAsOperatorAreForbidden(t *testing.T) {
	e := newStagesEnv(t)
	e.assertSettingsUntouched(func() {
		if _, err := e.svc.ReplaceLabels(context.Background(), e.co.Operator, genericLabels, e.meta); !errors.Is(err, authz.ErrForbidden) {
			t.Fatalf("ReplaceLabels error = %v, want authz.ErrForbidden", err)
		}
		if _, err := e.svc.ReplaceQuoteDefaults(context.Background(), e.co.Operator, settings.QuoteDefaults{ValidityDays: 30}, e.meta); !errors.Is(err, authz.ErrForbidden) {
			t.Fatalf("ReplaceQuoteDefaults error = %v, want authz.ErrForbidden", err)
		}
	})
}

// T-002-B701 (002/DD-19, INV-1, INV-11): without its settings row the company gets ErrTenantSettingsMissing from the
// two reads and the two writes, never an error that reads as "not found", and nothing is written. Valid input, so
// the missing row is the only reason to fail; the writes use values that differ from the defaults.
func TestPreferencesWithoutSettingsRow(t *testing.T) {
	ctx := context.Background()
	e := newStagesEnv(t)
	deleteSettingsRow(t, e.co.ID)
	changed := genericLabels
	changed.Project.Singular = "Obra"

	ops := map[string]func() error{
		"GetLabels": func() error { _, err := e.svc.GetLabels(ctx, e.co.Operator); return err },
		"GetQuoteDefaults": func() error {
			_, err := e.svc.GetQuoteDefaults(ctx, e.co.Operator)
			return err
		},
		"ReplaceLabels": func() error { _, err := e.svc.ReplaceLabels(ctx, e.co.Admin, changed, e.meta); return err },
		"ReplaceQuoteDefaults": func() error {
			_, err := e.svc.ReplaceQuoteDefaults(ctx, e.co.Admin, settings.QuoteDefaults{ValidityDays: 30}, e.meta)
			return err
		},
	}
	for name, op := range ops {
		t.Run(name, func(t *testing.T) {
			auditBefore := len(readAudit(t, e.co.ID, "settings."))
			err := op()
			if !errors.Is(err, settings.ErrTenantSettingsMissing) {
				t.Fatalf("error = %v, want ErrTenantSettingsMissing", err)
			}
			if errors.Is(err, settings.ErrNotFound) || errors.Is(err, db.ErrNotFound) {
				t.Fatalf("error %v reads as not found", err)
			}
			if got := len(readAudit(t, e.co.ID, "settings.")); got != auditBefore {
				t.Fatalf("audit rows: %d before, %d after", auditBefore, got)
			}
		})
	}
}

// T-002-B701 (002/DD-5, DD-10, mechanism E): ReplaceLabels and DeleteStage of one company share the settings lock,
// so they confirm one after the other and never deadlock (40P01). Mutation: without the lock in ReplaceLabels no
// call blocks behind T_admin and the helper fails.
func TestReplaceLabelsAndDeleteStageConfirmInTurn(t *testing.T) {
	e := newStagesEnv(t)
	finished := e.id("Perdido")
	in := genericLabels
	in.Project.Singular = "Obra"
	var errLabels, errDelete error
	holdSettingsLock(t, e.co.ID,
		func(ctx context.Context) error {
			_, errLabels = e.svc.ReplaceLabels(ctx, e.co.Admin, in, e.meta)
			return nil
		},
		func(ctx context.Context) error {
			errDelete = e.svc.DeleteStage(ctx, e.co.Admin, finished, e.meta)
			return nil
		},
	)
	if errLabels != nil || errDelete != nil {
		t.Fatalf("errors: %v, %v", errLabels, errDelete)
	}
	got, err := e.svc.GetLabels(context.Background(), e.co.Admin)
	if err != nil {
		t.Fatal(err)
	}
	equal(t, "project singular", got.Project.Singular, "Obra")
	equal(t, "pipeline", e.summary(), []string{"Presupuesto:open", "Aprobado:open", "En curso:open", "Finalizado:won"})
}

// T-002-B701 (002/DD-10, mechanism E): the two writes take the settings lock before reading the row, even when
// the values turn out to be the current ones and no UPDATE follows, so what they compare against cannot go stale.
// Mutation: without the lock they only block when they reach an UPDATE, and these no-op calls return at once.
func TestPreferenceWritesWaitForTheSettingsLock(t *testing.T) {
	e := newStagesEnv(t)
	var errLabels, errDefaults error
	holdSettingsLock(t, e.co.ID,
		func(ctx context.Context) error {
			_, errLabels = e.svc.ReplaceLabels(ctx, e.co.Admin, genericLabels, e.meta)
			return nil
		},
		func(ctx context.Context) error {
			_, errDefaults = e.svc.ReplaceQuoteDefaults(ctx, e.co.Admin, settings.QuoteDefaults{ValidityDays: 15}, e.meta)
			return nil
		},
	)
	if errLabels != nil || errDefaults != nil {
		t.Fatalf("errors: %v, %v", errLabels, errDefaults)
	}
}

//go:build integration

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/fdelillo/crm/internal/settings"
	"github.com/fdelillo/crm/internal/testsupport/fixture/e2e"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
)

const (
	labelsPath        = "/api/v1/settings/labels"
	quoteDefaultsPath = "/api/v1/settings/quote-defaults"
)

type entityLabelBody struct {
	Singular string `json:"singular"`
	Plural   string `json:"plural"`
	Gender   string `json:"gender"`
}

type labelsBody struct {
	Project  entityLabelBody `json:"project"`
	Customer entityLabelBody `json:"customer"`
	Quote    entityLabelBody `json:"quote"`
}

type quoteDefaultsBody struct {
	ValidityDays int     `json:"validity_days"`
	Terms        *string `json:"terms"`
}

// genericLabels are the names the "generic" template gives company A.
var genericLabels = labelsBody{
	Project:  entityLabelBody{"Proyecto", "Proyectos", "masculine"},
	Customer: entityLabelBody{"Cliente", "Clientes", "masculine"},
	Quote:    entityLabelBody{"Presupuesto", "Presupuestos", "masculine"},
}

func decodeLabels(t *testing.T, rec *httptest.ResponseRecorder) labelsBody {
	t.Helper()
	var out labelsBody
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func decodeQuoteDefaults(t *testing.T, rec *httptest.ResponseRecorder) quoteDefaultsBody {
	t.Helper()
	var out quoteDefaultsBody
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func labelsJSON(l labelsBody) string {
	body, _ := json.Marshal(l)
	return string(body)
}

func termsOf(s string) *string { return &s }

// T-002-B703 (Historias 2 y 6, 002/DD-10, DD-11): both resources are read by any role and written by the
// administrator; every response matches the contract and the next read prints what the write answered.
func TestPreferencesHTTPScenario(t *testing.T) {
	a := newStagesAPI(t)
	for _, who := range []string{"admin", "operator"} {
		if got := decodeLabels(t, a.do(who, "GET", labelsPath, "", "", 200)); got != genericLabels {
			t.Fatalf("%s sees labels %+v", who, got)
		}
		if got := decodeQuoteDefaults(t, a.do(who, "GET", quoteDefaultsPath, "", "", 200)); got.ValidityDays != 15 || got.Terms != nil {
			t.Fatalf("%s sees quote defaults %+v", who, got)
		}
	}
	// The names are replaced as a whole and trimmed.
	custom := labelsBody{
		Project:  entityLabelBody{"Obra", "Obras", "feminine"},
		Customer: entityLabelBody{"Comitente", "Comitentes", "masculine"},
		Quote:    entityLabelBody{"Cotización", "Cotizaciones", "feminine"},
	}
	untrimmed := `{"project":{"singular":" Obra ","plural":"Obras","gender":"feminine"},"customer":{"singular":"Comitente","plural":"Comitentes","gender":"masculine"},"quote":{"singular":"Cotización","plural":"Cotizaciones","gender":"feminine"}}`
	if got := decodeLabels(t, a.json("admin", "PUT", labelsPath, untrimmed, 200)); got != custom {
		t.Fatalf("replaced labels %+v, want %+v", got, custom)
	}
	for _, who := range []string{"admin", "operator"} {
		if got := decodeLabels(t, a.do(who, "GET", labelsPath, "", "", 200)); got != custom {
			t.Fatalf("%s reads labels %+v after the write", who, got)
		}
	}
	// The same names again are a 200 and write nothing.
	before := a.f.Snapshot(t, a.f.A.ID)
	if got := decodeLabels(t, a.json("admin", "PUT", labelsPath, labelsJSON(custom), 200)); got != custom {
		t.Fatalf("no-op labels %+v", got)
	}
	assertUnchanged(t, a.f, a.f.A.ID, before)

	// The conditions: a text is kept as written (trimmed), null and blank mean none, the limits are 1 and 365.
	for _, tc := range []struct {
		name, body string
		want       quoteDefaultsBody
	}{
		{"text", `{"validity_days":30,"terms":"  Seña del 50 %  "}`, quoteDefaultsBody{30, termsOf("Seña del 50 %")}},
		{"null terms", `{"validity_days":45,"terms":null}`, quoteDefaultsBody{45, nil}},
		{"blank terms", `{"validity_days":1,"terms":"   "}`, quoteDefaultsBody{1, nil}},
		{"upper limit with terms", `{"validity_days":365,"terms":"Pago a 30 días"}`, quoteDefaultsBody{365, termsOf("Pago a 30 días")}},
		{"empty terms", `{"validity_days":365,"terms":""}`, quoteDefaultsBody{365, nil}},
	} {
		got := decodeQuoteDefaults(t, a.json("admin", "PUT", quoteDefaultsPath, tc.body, 200))
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s: answered %+v, want %+v", tc.name, got, tc.want)
		}
		if read := decodeQuoteDefaults(t, a.do("operator", "GET", quoteDefaultsPath, "", "", 200)); !reflect.DeepEqual(read, tc.want) {
			t.Fatalf("%s: read %+v, want %+v", tc.name, read, tc.want)
		}
	}
	// Clearing the terms is not a no-op for the terms that were there, and repeating it is.
	a.json("admin", "PUT", quoteDefaultsPath, `{"validity_days":30,"terms":"Condición"}`, 200)
	a.json("admin", "PUT", quoteDefaultsPath, `{"validity_days":30,"terms":null}`, 200)
	if got := decodeQuoteDefaults(t, a.do("admin", "GET", quoteDefaultsPath, "", "", 200)); got.Terms != nil {
		t.Fatalf("terms after null: %+v", got)
	}
}

// T-002-B703 (002/DD-10, plan 9.1): the validation, the malformed bodies and the missing members of each
// operation; a rejected request changes nothing.
func TestPreferencesHTTPErrors(t *testing.T) {
	a := newStagesAPI(t)
	project := `"project":{"singular":"Obra","plural":"Obras","gender":"feminine"}`
	customer := `"customer":{"singular":"Cliente","plural":"Clientes","gender":"masculine"}`
	quote := `"quote":{"singular":"Presupuesto","plural":"Presupuestos","gender":"masculine"}`
	for _, tc := range []struct {
		name, path, body string
		want             int
		code             string
	}{
		{"labels without customer", labelsPath, `{` + project + `,` + quote + `}`, 422, "validation_failed:customer/required"},
		{"labels with a blank singular", labelsPath, `{"project":{"singular":"  ","plural":"Obras","gender":"feminine"},` + customer + `,` + quote + `}`, 422, "validation_failed:project.singular/required"},
		{"labels with a long plural", labelsPath, `{"project":{"singular":"Obra","plural":"` + strings.Repeat("x", 41) + `","gender":"feminine"},` + customer + `,` + quote + `}`, 422, "validation_failed:project.plural/too_long"},
		{"labels with a bad gender", labelsPath, `{` + project + `,` + customer + `,"quote":{"singular":"Presupuesto","plural":"Presupuestos","gender":"neutral"}}`, 422, "validation_failed:quote.gender/invalid_value"},
		{"labels without gender", labelsPath, `{"project":{"singular":"Obra","plural":"Obras"},` + customer + `,` + quote + `}`, 422, "validation_failed:project.gender/required"},
		{"labels unknown field", labelsPath, `{` + project + `,` + customer + `,` + quote + `,"extra":1}`, 400, "malformed_request"},
		{"labels nested unknown field", labelsPath, `{"project":{"singular":"Obra","plural":"Obras","gender":"feminine","extra":1},` + customer + `,` + quote + `}`, 400, "malformed_request"},
		{"labels null object", labelsPath, `{"project":null,` + customer + `,` + quote + `}`, 400, "malformed_request"},
		{"labels broken json", labelsPath, `{"project":`, 400, "malformed_request"},
		{"quote defaults validity 400", quoteDefaultsPath, `{"validity_days":400,"terms":null}`, 422, "validation_failed:validity_days/invalid_value"},
		{"quote defaults validity 366", quoteDefaultsPath, `{"validity_days":366,"terms":null}`, 422, "validation_failed:validity_days/invalid_value"},
		{"quote defaults validity 0", quoteDefaultsPath, `{"validity_days":0,"terms":null}`, 422, "validation_failed:validity_days/invalid_value"},
		{"quote defaults negative validity", quoteDefaultsPath, `{"validity_days":-1,"terms":null}`, 422, "validation_failed:validity_days/invalid_value"},
		{"quote defaults long terms", quoteDefaultsPath, `{"validity_days":30,"terms":"` + strings.Repeat("x", 2001) + `"}`, 422, "validation_failed:terms/too_long"},
		{"quote defaults without terms", quoteDefaultsPath, `{"validity_days":30}`, 422, "validation_failed:terms/required"},
		{"quote defaults without validity", quoteDefaultsPath, `{"terms":null}`, 422, "validation_failed:validity_days/required"},
		{"quote defaults validity as string", quoteDefaultsPath, `{"validity_days":"30","terms":null}`, 400, "malformed_request"},
		{"quote defaults validity with a zero decimal", quoteDefaultsPath, `{"validity_days":30.0,"terms":null}`, 400, "malformed_request"},
		{"quote defaults null validity", quoteDefaultsPath, `{"validity_days":null,"terms":null}`, 400, "malformed_request"},
		{"quote defaults unknown field", quoteDefaultsPath, `{"validity_days":30,"terms":null,"extra":1}`, 400, "malformed_request"},
		{"quote defaults terms as number", quoteDefaultsPath, `{"validity_days":30,"terms":1}`, 400, "malformed_request"},
		{"quote defaults broken json", quoteDefaultsPath, `{"validity_days":`, 400, "malformed_request"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := a.f.Snapshot(t, a.f.A.ID)
			rec := a.json("admin", "PUT", tc.path, tc.body, tc.want)
			if got := problemCode(t, rec); got != tc.code {
				t.Fatalf("problem=%s, want %s", rec.Body.String(), tc.code)
			}
			assertUnchanged(t, a.f, a.f.A.ID, before)
		})
	}
	// terms: null is a valid body.
	a.json("admin", "PUT", quoteDefaultsPath, `{"validity_days":30,"terms":null}`, 200)
}

// T-002-B703 (FR-008, 001/INV-12): the permission is decided before the body is read; an operator reads but never
// writes, and an anonymous request is a 401 for both.
func TestPreferencesHTTPOperatorCannotWrite(t *testing.T) {
	a := newStagesAPI(t)
	before := a.f.Snapshot(t, a.f.A.ID)
	for _, path := range []string{labelsPath, quoteDefaultsPath} {
		for _, body := range []string{labelsJSON(genericLabels), `{"validity_days":30,"terms":null}`, `{"broken":`, `{}`} {
			if got := problemCode(t, a.json("operator", "PUT", path, body, 403)); got != "forbidden" {
				t.Fatalf("PUT %s: code=%s", path, got)
			}
		}
		a.do("operator", "GET", path, "", "", 200)
		a.do("", "GET", path, "", "", 401)
		a.json("", "PUT", path, `{}`, 401)
	}
	assertUnchanged(t, a.f, a.f.A.ID, before)
}

// T-002-B703 (DD-30 and DD-17 de 001): body limit, media type and cross-origin protection apply to both writes.
func TestPreferencesHTTPTransportRules(t *testing.T) {
	a := newStagesAPI(t)
	before := a.f.Snapshot(t, a.f.A.ID)
	for _, tc := range []struct{ path, body string }{
		{labelsPath, labelsJSON(genericLabels)},
		{quoteDefaultsPath, `{"validity_days":30,"terms":null}`},
	} {
		oversize := tc.body[:len(tc.body)-1] + `,"pad":"` + strings.Repeat("x", 64*1024) + `"}`
		if got := problemCode(t, a.json("admin", "PUT", tc.path, oversize, 413)); got != "payload_too_large" {
			t.Fatalf("PUT %s: code=%s", tc.path, got)
		}
		if got := problemCode(t, a.do("admin", "PUT", tc.path, "", tc.body, 415)); got != "unsupported_media_type" {
			t.Fatalf("PUT %s: code=%s", tc.path, got)
		}
		if got := problemCode(t, a.do("admin", "PUT", tc.path, "text/plain", tc.body, 415)); got != "unsupported_media_type" {
			t.Fatalf("PUT %s: code=%s", tc.path, got)
		}
		req := httptest.NewRequest("PUT", tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		req.AddCookie(a.f.A.Users["admin"].Cookie)
		rec := httptest.NewRecorder()
		a.h.ServeHTTP(rec, req)
		if rec.Code != 403 || problemCode(t, rec) != "forbidden" {
			t.Fatalf("PUT %s cross-origin: %d %s", tc.path, rec.Code, rec.Body.String())
		}
	}
	assertUnchanged(t, a.f, a.f.A.ID, before)
}

// T-002-B703 (002/DD-19, INV-1, plan 12.2): a company that lost its settings row is a bug, never a 404 or an
// empty answer. The four operations answer 500 internal, log one settings_row_missing line with the request id at
// ERROR and write nothing.
func TestPreferencesHTTPMissingSettingsRowIs500(t *testing.T) {
	var logs bytes.Buffer
	a := newStagesAPIWithLogger(t, slog.New(slog.NewJSONHandler(&logs, nil)))
	if _, err := pgtest.SuperuserPool(t).Exec(context.Background(), `DELETE FROM app.tenant_settings WHERE tenant_id=$1`, a.f.A.ID); err != nil {
		t.Fatal(err)
	}
	before := a.f.Snapshot(t, a.f.A.ID)
	for _, tc := range []struct{ name, method, path, body string }{
		{"get labels", "GET", labelsPath, ""},
		{"replace labels", "PUT", labelsPath, labelsJSON(genericLabels)},
		{"get quote defaults", "GET", quoteDefaultsPath, ""},
		{"replace quote defaults", "PUT", quoteDefaultsPath, `{"validity_days":30,"terms":null}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs.Reset()
			ct := ""
			if tc.body != "" {
				ct = "application/json"
			}
			rec := a.do("admin", tc.method, tc.path, ct, tc.body, 500)
			if got := problemCode(t, rec); got != "internal" {
				t.Fatalf("code=%s", got)
			}
			lines := 0
			for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
				var entry map[string]any
				if err := json.Unmarshal([]byte(line), &entry); err != nil {
					t.Fatalf("log line %q: %v", line, err)
				}
				if entry["event"] != "settings_row_missing" {
					continue
				}
				lines++
				if id, _ := entry["request_id"].(string); id == "" || entry["level"] != "ERROR" {
					t.Fatalf("settings_row_missing line = %s, want ERROR with request_id", line)
				}
			}
			if lines != 1 {
				t.Fatalf("settings_row_missing lines = %d, want 1; log: %s", lines, logs.String())
			}
		})
	}
	assertUnchanged(t, a.f, a.f.A.ID, before)
}

func labelsBodyOf(l settings.Labels) labelsBody {
	of := func(e settings.EntityLabel) entityLabelBody {
		return entityLabelBody{Singular: e.Singular, Plural: e.Plural, Gender: string(e.Gender)}
	}
	return labelsBody{Project: of(l.Project), Customer: of(l.Customer), Quote: of(l.Quote)}
}

func quoteDefaultsBodyOf(q settings.QuoteDefaults) quoteDefaultsBody {
	return quoteDefaultsBody{ValidityDays: q.ValidityDays, Terms: q.Terms}
}

// foreignPreferences gives company B names and conditions that company A never has, so a body that carries them
// is a cross-company read. It returns the texts to look for and what B holds afterwards.
func foreignPreferences(t *testing.T, f *e2e.Scenario) (needles []string, labels labelsBody, defaults quoteDefaultsBody) {
	t.Helper()
	ctx, p := context.Background(), f.B.Users["admin"].Principal
	l, err := f.Settings.ReplaceLabels(ctx, p, settings.Labels{
		Project:  settings.EntityLabel{Singular: "Faena", Plural: "Faenas", Gender: "feminine"},
		Customer: settings.EntityLabel{Singular: "Mandante", Plural: "Mandantes", Gender: "masculine"},
		Quote:    settings.EntityLabel{Singular: "Propuesta", Plural: "Propuestas", Gender: "feminine"},
	}, settings.RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	q, err := f.Settings.ReplaceQuoteDefaults(ctx, p, settings.QuoteDefaults{ValidityDays: 77, Terms: termsOf("Condiciones exclusivas de B")}, settings.RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	return []string{"Faena", "Mandante", "Propuesta", "Condiciones exclusivas de B"}, labelsBodyOf(l), quoteDefaultsBodyOf(q)
}

//go:build integration

package app_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/fdelillo/crm/internal/settings"
	"github.com/google/uuid"
)

// categoryBody is Category as the API answers it.
type categoryBody struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	System    bool    `json:"system"`
	SystemKey *string `json:"system_key"`
	Archived  bool    `json:"archived"`
}

func decodeCategory(t *testing.T, rec *httptest.ResponseRecorder) categoryBody {
	t.Helper()
	var c categoryBody
	if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	return c
}

// categories reads GET /categories with the given query (e.g. "?type=income") as who.
func (a *stagesAPI) categories(who, query string) []categoryBody {
	a.t.Helper()
	var out struct {
		Items []categoryBody `json:"items"`
	}
	if err := json.Unmarshal(a.do(who, "GET", "/api/v1/categories"+query, "", "", 200).Body.Bytes(), &out); err != nil {
		a.t.Fatal(err)
	}
	return out.Items
}

func categoryNames(cs []categoryBody) []string {
	names := make([]string, len(cs))
	for i, c := range cs {
		names[i] = c.Type + ":" + c.Name
	}
	return names
}

func (a *stagesAPI) categoryID(name, typ string) string {
	a.t.Helper()
	for _, c := range a.categories("admin", "?include_archived=true") {
		if c.Name == name && c.Type == typ {
			return c.ID
		}
	}
	a.t.Fatalf("no category %q of type %s", name, typ)
	return ""
}

// T-002-B503 (Historia 3, 002/DD-12): the five operations walk the independent test of the phase and every
// response matches the contract.
func TestCategoriesHTTPScenario(t *testing.T) {
	a := newStagesAPI(t)
	// The five system categories and then the ones the "generic" template adds, income first.
	defaults := []string{"income:Aportes de socios", "income:Cobros de clientes", "income:Otros ingresos",
		"expense:Pagos a empleados", "expense:Pagos a proveedores", "expense:Retiros de socios",
		"expense:Alquiler", "expense:Flete", "expense:Impuestos", "expense:Mano de obra", "expense:Materiales", "expense:Otros gastos", "expense:Servicios"}
	for _, who := range []string{"admin", "operator"} {
		if got := categoryNames(a.categories(who, "")); !reflect.DeepEqual(got, defaults) {
			t.Fatalf("%s sees %v", who, got)
		}
	}
	// A system category is renamed, with its key and its system flag intact, but never archived.
	receipts := a.categoryID("Cobros de clientes", "income")
	renamed := decodeCategory(t, a.json("admin", "PATCH", "/api/v1/categories/"+receipts, `{"name":"  Cobranzas "}`, 200))
	if renamed.ID != receipts || renamed.Name != "Cobranzas" || !renamed.System || renamed.SystemKey == nil || *renamed.SystemKey != "customer_receipts" || renamed.Archived {
		t.Fatalf("renamed %+v", renamed)
	}
	if got := problemCode(t, a.json("admin", "POST", "/api/v1/categories/"+receipts+"/archive", "", 409)); got != "invalid_state" {
		t.Fatalf("code=%s", got)
	}
	// A category of the company is created, archived, seen only with include_archived and restored.
	created := decodeCategory(t, a.json("admin", "POST", "/api/v1/categories", `{"name":" Combustible ","type":"expense"}`, 201))
	if created.Name != "Combustible" || created.Type != "expense" || created.System || created.SystemKey != nil || created.Archived || created.ID == "" {
		t.Fatalf("created %+v", created)
	}
	archived := decodeCategory(t, a.json("admin", "POST", "/api/v1/categories/"+created.ID+"/archive", "", 200))
	if archived.ID != created.ID || !archived.Archived {
		t.Fatalf("archived %+v", archived)
	}
	if again := decodeCategory(t, a.json("admin", "POST", "/api/v1/categories/"+created.ID+"/archive", "", 200)); again != archived {
		t.Fatalf("archiving an archived one returned %+v, want %+v", again, archived)
	}
	for _, c := range a.categories("operator", "") {
		if c.ID == created.ID {
			t.Fatalf("archived category listed by default: %+v", c)
		}
	}
	withArchived := a.categories("operator", "?include_archived=true")
	if got := categoryNames(withArchived); !slices.Contains(got, "expense:Combustible") || len(got) != len(defaults)+1 {
		t.Fatalf("with include_archived: %v", got)
	}
	for _, c := range withArchived {
		if c.ID == created.ID && !c.Archived {
			t.Fatalf("archived flag missing: %+v", c)
		}
	}
	if got := categoryNames(a.categories("admin", "?type=income")); !reflect.DeepEqual(got, []string{"income:Aportes de socios", "income:Cobranzas", "income:Otros ingresos"}) {
		t.Fatalf("type=income: %v", got)
	}
	for _, c := range a.categories("admin", "?type=expense&include_archived=false") {
		if c.Type != "expense" || c.ID == created.ID {
			t.Fatalf("type=expense listed %+v", c)
		}
	}
	restored := decodeCategory(t, a.json("admin", "POST", "/api/v1/categories/"+created.ID+"/restore", "", 200))
	if restored != created {
		t.Fatalf("restored %+v, want %+v", restored, created)
	}
	if again := decodeCategory(t, a.json("admin", "POST", "/api/v1/categories/"+created.ID+"/restore", "", 200)); again != created {
		t.Fatalf("restoring an active one returned %+v, want %+v", again, created)
	}
	// An archived category is renamed too, and the same name again is a 200 with the category.
	a.json("admin", "POST", "/api/v1/categories/"+created.ID+"/archive", "", 200)
	if got := decodeCategory(t, a.json("admin", "PATCH", "/api/v1/categories/"+created.ID, `{"name":"Nafta"}`, 200)); got.Name != "Nafta" || !got.Archived {
		t.Fatalf("renamed archived %+v", got)
	}
	if got := decodeCategory(t, a.json("admin", "PATCH", "/api/v1/categories/"+created.ID, `{"name":"Nafta"}`, 200)); got.Name != "Nafta" {
		t.Fatalf("no-op rename %+v", got)
	}
}

// T-002-B503 (002/DD-12, DD-16, INV-6, INV-11): the query, validation, malformed bodies and conflicts of each
// operation; a rejected request changes nothing.
func TestCategoriesHTTPErrors(t *testing.T) {
	a := newStagesAPI(t)
	// "Combustible" is archived and "combustible" is active: restoring the first one collides with the second.
	old := decodeCategory(t, a.json("admin", "POST", "/api/v1/categories", `{"name":"Combustible","type":"expense"}`, 201))
	a.json("admin", "POST", "/api/v1/categories/"+old.ID+"/archive", "", 200)
	a.json("admin", "POST", "/api/v1/categories", `{"name":"combustible","type":"expense"}`, 201)
	system := a.categoryID("Retiros de socios", "expense")
	other := a.categoryID("Aportes de socios", "income")
	for _, tc := range []struct {
		name, method, path, body string
		want                     int
		code                     string
	}{
		{"list type foo", "GET", "/api/v1/categories?type=foo", "", 400, "malformed_request"},
		{"list type empty", "GET", "/api/v1/categories?type=", "", 400, "malformed_request"},
		{"list include_archived maybe", "GET", "/api/v1/categories?include_archived=maybe", "", 400, "malformed_request"},
		{"create repeated name", "POST", "/api/v1/categories", `{"name":"RETIROS DE SOCIOS","type":"expense"}`, 409, "name_taken"},
		{"create repeated name of the company", "POST", "/api/v1/categories", `{"name":"COMBUSTIBLE","type":"expense"}`, 409, "name_taken"},
		{"create bad type", "POST", "/api/v1/categories", `{"name":"Nueva","type":"other"}`, 422, "validation_failed:type/invalid_value"},
		{"create without name", "POST", "/api/v1/categories", `{"type":"income"}`, 422, "validation_failed:name/required"},
		{"create blank name", "POST", "/api/v1/categories", `{"name":"   ","type":"income"}`, 422, "validation_failed:name/required"},
		{"create name too long", "POST", "/api/v1/categories", `{"name":"` + strings.Repeat("x", 61) + `","type":"income"}`, 422, "validation_failed:name/too_long"},
		{"create without type", "POST", "/api/v1/categories", `{"name":"Nueva"}`, 422, "validation_failed:type/invalid_value"},
		{"create null body", "POST", "/api/v1/categories", `null`, 400, "malformed_request"},
		{"create array body", "POST", "/api/v1/categories", `[]`, 400, "malformed_request"},
		{"create null name", "POST", "/api/v1/categories", `{"name":null,"type":"income"}`, 400, "malformed_request"},
		{"create unknown field", "POST", "/api/v1/categories", `{"name":"Nueva","type":"income","extra":1}`, 400, "malformed_request"},
		{"create name as number", "POST", "/api/v1/categories", `{"name":1,"type":"income"}`, 400, "malformed_request"},
		{"create broken json", "POST", "/api/v1/categories", `{"name":`, 400, "malformed_request"},
		{"rename empty", "PATCH", "/api/v1/categories/{system}", `{}`, 400, "malformed_request"},
		{"rename null body", "PATCH", "/api/v1/categories/{system}", `null`, 400, "malformed_request"},
		{"rename null name", "PATCH", "/api/v1/categories/{system}", `{"name":null}`, 400, "malformed_request"},
		{"rename unknown field", "PATCH", "/api/v1/categories/{system}", `{"type":"income"}`, 400, "malformed_request"},
		{"rename blank", "PATCH", "/api/v1/categories/{system}", `{"name":"  "}`, 422, "validation_failed:name/required"},
		// The fields are validated before the category is looked up: an invalid body on a missing one is a 422.
		{"rename invalid name of a missing category", "PATCH", "/api/v1/categories/" + uuid.NewString(), `{"name":""}`, 422, "validation_failed:name/required"},
		{"rename to the name of another of its type", "PATCH", "/api/v1/categories/{system}", `{"name":"pagos a proveedores"}`, 409, "name_taken"},
		{"archive a system one", "POST", "/api/v1/categories/{system}/archive", "", 409, "invalid_state"},
		{"archive a system one of the other type", "POST", "/api/v1/categories/{other}/archive", "", 409, "invalid_state"},
		{"restore with the name taken", "POST", "/api/v1/categories/{old}/restore", "", 409, "name_taken"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := strings.NewReplacer("{system}", system, "{other}", other, "{old}", old.ID).Replace(tc.path)
			ct := ""
			if tc.body != "" {
				ct = "application/json"
			}
			before := a.f.Snapshot(t, a.f.A.ID)
			rec := a.do("admin", tc.method, path, ct, tc.body, tc.want)
			if got := problemCode(t, rec); got != tc.code {
				t.Fatalf("problem=%s, want %s", rec.Body.String(), tc.code)
			}
			assertUnchanged(t, a.f, a.f.A.ID, before)
		})
	}
	// The same name in the other type is not a collision.
	a.json("admin", "POST", "/api/v1/categories", `{"name":"Retiros de socios","type":"income"}`, 201)
}

// T-002-B503 (002/DD-15, INV-11): another company's category, a missing one and a malformed id are
// indistinguishable 404s, and company B does not change.
func TestCategoriesHTTPUnknownCategoryIs404(t *testing.T) {
	a := newStagesAPI(t)
	foreign, err := a.f.Settings.ListCategories(context.Background(), a.f.B.Users["admin"].Principal, settings.CategoryFilter{IncludeArchived: true})
	if err != nil || len(foreign) == 0 {
		t.Fatalf("categories of B: %v %v", foreign, err)
	}
	// The routes exist: a category of the company is found where the others are not.
	a.json("admin", "PATCH", "/api/v1/categories/"+a.categoryID("Aportes de socios", "income"), `{"name":"Aportes"}`, 200)
	bBefore := a.f.Snapshot(t, a.f.B.ID)
	for _, id := range []string{foreign[0].ID.String(), uuid.NewString(), "no-uuid"} {
		for _, tc := range []struct{ method, suffix, body string }{
			{"PATCH", "", `{"name":"Otra"}`},
			{"POST", "/archive", ""},
			{"POST", "/restore", ""},
		} {
			ct := ""
			if tc.body != "" {
				ct = "application/json"
			}
			rec := a.do("admin", tc.method, "/api/v1/categories/"+id+tc.suffix, ct, tc.body, 404)
			if got := problemCode(t, rec); got != "not_found" {
				t.Fatalf("%s %s%s: code=%s", tc.method, id, tc.suffix, got)
			}
			assertNoCompanyResponseData(t, rec, a.f.B, false)
		}
	}
	assertUnchanged(t, a.f, a.f.B.ID, bBefore)
}

// T-002-B503 (FR-007 de 001, INV-11): the permission is decided before the body is read and before the category
// is looked up; an operator reads but never writes.
func TestCategoriesHTTPOperatorCannotWrite(t *testing.T) {
	a := newStagesAPI(t)
	id := a.categoryID("Aportes de socios", "income")
	before := a.f.Snapshot(t, a.f.A.ID)
	for _, tc := range []struct{ method, path string }{
		{"POST", "/api/v1/categories"},
		{"PATCH", "/api/v1/categories/" + id},
		{"POST", "/api/v1/categories/" + id + "/archive"},
		{"POST", "/api/v1/categories/" + id + "/restore"},
		{"PATCH", "/api/v1/categories/no-uuid"},
		{"POST", "/api/v1/categories/" + uuid.NewString() + "/archive"},
	} {
		for _, body := range []string{`{"name":"Nueva","type":"income"}`, `{"broken":`, `{}`} {
			rec := a.json("operator", tc.method, tc.path, body, 403)
			if got := problemCode(t, rec); got != "forbidden" {
				t.Fatalf("%s %s: code=%s", tc.method, tc.path, got)
			}
		}
	}
	assertUnchanged(t, a.f, a.f.A.ID, before)
	a.do("operator", "GET", "/api/v1/categories", "", "", 200)
	a.do("", "GET", "/api/v1/categories", "", "", 401)
	a.json("", "POST", "/api/v1/categories", `{"name":"Nueva","type":"income"}`, 401)
}

// T-002-B503 (DD-30 and DD-17 de 001): body limit, media type and cross-origin protection apply to the category
// writes that take a body.
func TestCategoriesHTTPTransportRules(t *testing.T) {
	a := newStagesAPI(t)
	id := a.categoryID("Aportes de socios", "income")
	oversize := `{"name":"Nueva","type":"income","pad":"` + strings.Repeat("x", 64*1024) + `"}`
	before := a.f.Snapshot(t, a.f.A.ID)
	for _, tc := range []struct{ method, path, body string }{
		{"POST", "/api/v1/categories", `{"name":"Nueva","type":"income"}`},
		{"PATCH", "/api/v1/categories/" + id, `{"name":"Otra"}`},
	} {
		if got := problemCode(t, a.json("admin", tc.method, tc.path, oversize, 413)); got != "payload_too_large" {
			t.Fatalf("%s %s: code=%s", tc.method, tc.path, got)
		}
		if got := problemCode(t, a.do("admin", tc.method, tc.path, "", tc.body, 415)); got != "unsupported_media_type" {
			t.Fatalf("%s %s: code=%s", tc.method, tc.path, got)
		}
		if got := problemCode(t, a.do("admin", tc.method, tc.path, "text/plain", tc.body, 415)); got != "unsupported_media_type" {
			t.Fatalf("%s %s: code=%s", tc.method, tc.path, got)
		}
	}
	for _, tc := range []struct{ method, path, body string }{
		{"POST", "/api/v1/categories", `{"name":"Nueva","type":"income"}`},
		{"PATCH", "/api/v1/categories/" + id, `{"name":"Otra"}`},
		{"POST", "/api/v1/categories/" + id + "/archive", ""},
		{"POST", "/api/v1/categories/" + id + "/restore", ""},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		req.AddCookie(a.f.A.Users["admin"].Cookie)
		rec := httptest.NewRecorder()
		a.h.ServeHTTP(rec, req)
		if rec.Code != 403 || problemCode(t, rec) != "forbidden" {
			t.Fatalf("%s %s cross-origin: %d %s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
	assertUnchanged(t, a.f, a.f.A.ID, before)
}

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

	"github.com/google/uuid"
)

// moneyBody is Money as the API answers it.
type moneyBody struct {
	AmountCents int64  `json:"amount_cents"`
	Currency    string `json:"currency"`
}

// catalogItemBody is CatalogItem as the API answers it.
type catalogItemBody struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Description    *string    `json:"description"`
	Unit           string     `json:"unit"`
	UnitLabel      *string    `json:"unit_label"`
	ReferencePrice *moneyBody `json:"reference_price"`
	Archived       bool       `json:"archived"`
	CreatedAt      string     `json:"created_at"`
	UpdatedAt      string     `json:"updated_at"`
}

func decodeCatalogItem(t *testing.T, rec *httptest.ResponseRecorder) catalogItemBody {
	t.Helper()
	var c catalogItemBody
	if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	return c
}

// catalogItems reads GET /catalog-items with the given query (e.g. "?include_archived=true") as who.
func (a *stagesAPI) catalogItems(who, query string) []catalogItemBody {
	a.t.Helper()
	var out struct {
		Items []catalogItemBody `json:"items"`
	}
	if err := json.Unmarshal(a.do(who, "GET", "/api/v1/catalog-items"+query, "", "", 200).Body.Bytes(), &out); err != nil {
		a.t.Fatal(err)
	}
	return out.Items
}

func catalogItemNames(items []catalogItemBody) []string {
	names := make([]string, len(items))
	for i, c := range items {
		names[i] = c.Name
	}
	return names
}

// newCatalogItem creates an item as the administrator and returns it.
func (a *stagesAPI) newCatalogItem(body string) catalogItemBody {
	a.t.Helper()
	return decodeCatalogItem(a.t, a.json("admin", "POST", "/api/v1/catalog-items", body, 201))
}

// T-002-B603 (Historia 4, 002/DD-9, DD-12, ADR-029): the six operations walk the independent test of the phase and
// every response matches the contract.
func TestCatalogItemsHTTPScenario(t *testing.T) {
	a := newStagesAPI(t)
	// The "generic" template brings no catalog; an empty list is [], never null.
	for _, who := range []string{"admin", "operator"} {
		if body := a.do(who, "GET", "/api/v1/catalog-items", "", "", 200).Body.String(); !strings.Contains(body, `"items":[]`) {
			t.Fatalf("%s sees %s", who, body)
		}
	}
	created := a.json("admin", "POST", "/api/v1/catalog-items", `{"name":"  Portón corredizo ","unit":"unit","reference_price":{"amount_cents":450000,"currency":"USD"}}`, 201)
	// The amount leaves as a JSON integer: no decimals, no exponent.
	if body := created.Body.String(); !strings.Contains(body, `"amount_cents":450000,`) && !strings.Contains(body, `"amount_cents":450000}`) {
		t.Fatalf("amount_cents is not a plain integer: %s", body)
	}
	item := decodeCatalogItem(t, created)
	if item.ID == "" || item.Name != "Portón corredizo" || item.Unit != "unit" || item.Description != nil || item.UnitLabel != nil || item.Archived ||
		!reflect.DeepEqual(item.ReferencePrice, &moneyBody{AmountCents: 450000, Currency: "USD"}) || item.CreatedAt == "" || item.UpdatedAt != item.CreatedAt {
		t.Fatalf("created %+v", item)
	}
	for _, who := range []string{"admin", "operator"} {
		if got := a.catalogItems(who, ""); len(got) != 1 || !reflect.DeepEqual(got[0], item) {
			t.Fatalf("%s sees %+v", who, got)
		}
		if got := decodeCatalogItem(t, a.do(who, "GET", "/api/v1/catalog-items/"+item.ID, "", "", 200)); !reflect.DeepEqual(got, item) {
			t.Fatalf("%s gets %+v", who, got)
		}
	}
	// Only the price changes.
	repriced := decodeCatalogItem(t, a.json("admin", "PATCH", "/api/v1/catalog-items/"+item.ID, `{"reference_price":{"amount_cents":520000,"currency":"USD"}}`, 200))
	want := item
	want.ReferencePrice = &moneyBody{AmountCents: 520000, Currency: "USD"}
	want.UpdatedAt = repriced.UpdatedAt
	if !reflect.DeepEqual(repriced, want) || repriced.UpdatedAt <= item.UpdatedAt {
		t.Fatalf("repriced %+v, want %+v after %s", repriced, want, item.UpdatedAt)
	}
	// Archiving takes it out of the default list; only include_archived=true shows it, and GET by id still finds it.
	archived := decodeCatalogItem(t, a.json("admin", "POST", "/api/v1/catalog-items/"+item.ID+"/archive", "", 200))
	if !archived.Archived || archived.ID != item.ID || archived.Name != item.Name || !reflect.DeepEqual(archived.ReferencePrice, want.ReferencePrice) {
		t.Fatalf("archived %+v", archived)
	}
	if again := decodeCatalogItem(t, a.json("admin", "POST", "/api/v1/catalog-items/"+item.ID+"/archive", "", 200)); !reflect.DeepEqual(again, archived) {
		t.Fatalf("archiving an archived one returned %+v, want %+v", again, archived)
	}
	for _, query := range []string{"", "?include_archived=false"} {
		if got := a.catalogItems("operator", query); len(got) != 0 {
			t.Fatalf("archived item listed with %q: %+v", query, got)
		}
	}
	if got := a.catalogItems("operator", "?include_archived=true"); len(got) != 1 || !reflect.DeepEqual(got[0], archived) {
		t.Fatalf("include_archived=true: %+v", got)
	}
	if got := decodeCatalogItem(t, a.do("operator", "GET", "/api/v1/catalog-items/"+item.ID, "", "", 200)); !got.Archived {
		t.Fatalf("archived item by id: %+v", got)
	}
	// An archived item is still editable, and restoring it puts it back, twice the same.
	if got := decodeCatalogItem(t, a.json("admin", "PATCH", "/api/v1/catalog-items/"+item.ID, `{"description":"Con motor"}`, 200)); !got.Archived || got.Description == nil || *got.Description != "Con motor" {
		t.Fatalf("edited archived %+v", got)
	}
	restored := decodeCatalogItem(t, a.json("admin", "POST", "/api/v1/catalog-items/"+item.ID+"/restore", "", 200))
	if restored.Archived {
		t.Fatalf("restored %+v", restored)
	}
	if again := decodeCatalogItem(t, a.json("admin", "POST", "/api/v1/catalog-items/"+item.ID+"/restore", "", 200)); !reflect.DeepEqual(again, restored) {
		t.Fatalf("restoring an active one returned %+v, want %+v", again, restored)
	}
	if got := a.catalogItems("operator", ""); len(got) != 1 || got[0].ID != item.ID {
		t.Fatalf("restored item not listed: %+v", got)
	}
}

// T-002-B603 (002/DD-9, P-9, INV-7, ADR-029): what a valid creation answers.
func TestCatalogItemsHTTPCreate(t *testing.T) {
	a := newStagesAPI(t)
	for _, tc := range []struct {
		name, body string
		check      func(t *testing.T, got catalogItemBody, raw string)
	}{
		{"without price", `{"name":"Reja","unit":"square_meter"}`, func(t *testing.T, got catalogItemBody, raw string) {
			if got.ReferencePrice != nil || !strings.Contains(raw, `"reference_price":null`) || !strings.Contains(raw, `"description":null`) || !strings.Contains(raw, `"unit_label":null`) {
				t.Fatalf("%s", raw)
			}
		}},
		{"explicit nulls", `{"name":"Reja 2","unit":"unit","description":null,"unit_label":null,"reference_price":null}`, func(t *testing.T, got catalogItemBody, raw string) {
			if got.ReferencePrice != nil || got.Description != nil || got.UnitLabel != nil {
				t.Fatalf("%s", raw)
			}
		}},
		{"price zero is a price", `{"name":"Cortesía","unit":"unit","reference_price":{"amount_cents":0,"currency":"ARS"}}`, func(t *testing.T, got catalogItemBody, raw string) {
			if !reflect.DeepEqual(got.ReferencePrice, &moneyBody{AmountCents: 0, Currency: "ARS"}) || !strings.Contains(raw, `"amount_cents":0`) {
				t.Fatalf("%s", raw)
			}
		}},
		{"largest amount", `{"name":"Obra","unit":"linear_meter","reference_price":{"amount_cents":9007199254740991,"currency":"USD"}}`, func(t *testing.T, got catalogItemBody, raw string) {
			if !strings.Contains(raw, `"amount_cents":9007199254740991,`) {
				t.Fatalf("%s", raw)
			}
		}},
		{"other with label", `{"name":"Flete","unit":"other","unit_label":" jornal "}`, func(t *testing.T, got catalogItemBody, raw string) {
			if got.UnitLabel == nil || *got.UnitLabel != "jornal" {
				t.Fatalf("%s", raw)
			}
		}},
		{"label of another unit is discarded", `{"name":"Mano de obra","unit":"hour","unit_label":"jornal"}`, func(t *testing.T, got catalogItemBody, raw string) {
			if got.UnitLabel != nil {
				t.Fatalf("%s", raw)
			}
		}},
		{"empty description is none", `{"name":"Cierre","unit":"unit","description":""}`, func(t *testing.T, got catalogItemBody, raw string) {
			if got.Description != nil {
				t.Fatalf("%s", raw)
			}
		}},
		{"repeated name", `{"name":"reja","unit":"unit"}`, func(t *testing.T, got catalogItemBody, raw string) {
			if got.Name != "reja" {
				t.Fatalf("%s", raw)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := a.json("admin", "POST", "/api/v1/catalog-items", tc.body, 201)
			tc.check(t, decodeCatalogItem(t, rec), rec.Body.String())
		})
	}
	// Names are not unique: "Reja" and "reja" are two items, listed by name ignoring case.
	names := catalogItemNames(a.catalogItems("admin", ""))
	if want := []string{"Cierre", "Cortesía", "Flete", "Mano de obra", "Obra", "Reja", "reja", "Reja 2"}; !slices.Equal(names, want) {
		t.Fatalf("names = %v, want %v", names, want)
	}
}

// T-002-B603 (002/DD-9, DD-12, INV-6, INV-7, ADR-029, plan 9.1): the validation and malformed-body rows of each
// operation; a rejected request changes nothing.
func TestCatalogItemsHTTPErrors(t *testing.T) {
	a := newStagesAPI(t)
	plain := a.newCatalogItem(`{"name":"Reja","unit":"unit","reference_price":{"amount_cents":100,"currency":"ARS"}}`)
	other := a.newCatalogItem(`{"name":"Jornal","unit":"other","unit_label":"jornal"}`)
	for _, tc := range []struct {
		name, method, path, body string
		want                     int
		code                     string
	}{
		{"list include_archived maybe", "GET", "/api/v1/catalog-items?include_archived=maybe", "", 400, "malformed_request"},
		{"create amount with decimals", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"amount_cents":1.5,"currency":"ARS"}}`, 400, "malformed_request"},
		{"create amount as exponent", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"amount_cents":1e3,"currency":"ARS"}}`, 400, "malformed_request"},
		{"create amount as string", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"amount_cents":"100","currency":"ARS"}}`, 400, "malformed_request"},
		{"create amount beyond int64", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"amount_cents":9223372036854775808,"currency":"ARS"}}`, 400, "malformed_request"},
		{"create amount beyond 2^53-1", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"amount_cents":9007199254740992,"currency":"ARS"}}`, 422, "validation_failed:reference_price.amount_cents/invalid_value"},
		{"create negative amount", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"amount_cents":-1,"currency":"ARS"}}`, 422, "validation_failed:reference_price.amount_cents/invalid_value"},
		{"create price without amount", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"currency":"ARS"}}`, 422, "validation_failed:reference_price.amount_cents/required"},
		{"create currency EUR", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"amount_cents":1,"currency":"EUR"}}`, 422, "validation_failed:reference_price.currency/invalid_value"},
		{"create price without currency", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"amount_cents":1}}`, 422, "validation_failed:reference_price.currency/required"},
		{"create empty currency", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"amount_cents":1,"currency":""}}`, 422, "validation_failed:reference_price.currency/required"},
		{"create other without label", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"other"}`, 422, "validation_failed:unit_label/required"},
		{"create other with null label", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"other","unit_label":null}`, 422, "validation_failed:unit_label/required"},
		{"create label too long", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"other","unit_label":"` + strings.Repeat("x", 21) + `"}`, 422, "validation_failed:unit_label/too_long"},
		{"create without name", "POST", "/api/v1/catalog-items", `{"unit":"unit"}`, 422, "validation_failed:name/required"},
		{"create name too long", "POST", "/api/v1/catalog-items", `{"name":"` + strings.Repeat("x", 121) + `","unit":"unit"}`, 422, "validation_failed:name/too_long"},
		{"create description too long", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","description":"` + strings.Repeat("x", 1001) + `"}`, 422, "validation_failed:description/too_long"},
		{"create without unit", "POST", "/api/v1/catalog-items", `{"name":"A"}`, 422, "validation_failed:unit/required"},
		{"create unknown unit", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"box"}`, 422, "validation_failed:unit/invalid_value"},
		{"create null name", "POST", "/api/v1/catalog-items", `{"name":null,"unit":"unit"}`, 400, "malformed_request"},
		{"create unknown field", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","archived":true}`, 400, "malformed_request"},
		{"create price with unknown field", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"amount_cents":1,"currency":"ARS","x":1}}`, 400, "malformed_request"},
		{"create broken json", "POST", "/api/v1/catalog-items", `{"name":`, 400, "malformed_request"},
		{"create repeated amount_cents 100 then 900", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"amount_cents":100,"amount_cents":900,"currency":"ARS"}}`, 400, "malformed_request"},
		{"update repeated amount_cents 100 then 900", "PATCH", "/api/v1/catalog-items/{plain}", `{"reference_price":{"amount_cents":100,"amount_cents":900,"currency":"ARS"}}`, 400, "malformed_request"},
		{"create repeated amount_cents 900 then 100", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"amount_cents":900,"amount_cents":100,"currency":"ARS"}}`, 400, "malformed_request"},
		{"update repeated amount_cents 900 then 100", "PATCH", "/api/v1/catalog-items/{plain}", `{"reference_price":{"amount_cents":900,"amount_cents":100,"currency":"ARS"}}`, 400, "malformed_request"},
		{"create repeated currency", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"amount_cents":1,"currency":"ARS","currency":"USD"}}`, 400, "malformed_request"},
		{"update repeated currency", "PATCH", "/api/v1/catalog-items/{plain}", `{"reference_price":{"amount_cents":1,"currency":"ARS","currency":"USD"}}`, 400, "malformed_request"},
		{"create repeated amount_cents with an escaped key", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"amount_cents":1,"amount_c\u0065nts":2,"currency":"ARS"}}`, 400, "malformed_request"},
		{"update repeated amount_cents with an escaped key", "PATCH", "/api/v1/catalog-items/{plain}", `{"reference_price":{"amount_cents":1,"amount_c\u0065nts":2,"currency":"ARS"}}`, 400, "malformed_request"},
		{"create repeated name", "POST", "/api/v1/catalog-items", `{"name":"A","name":"B","unit":"unit"}`, 400, "malformed_request"},
		{"create name repeated in another case", "POST", "/api/v1/catalog-items", `{"name":"A","Name":"B","unit":"unit"}`, 400, "malformed_request"},
		{"update amount_cents repeated in another case", "PATCH", "/api/v1/catalog-items/{plain}", `{"reference_price":{"amount_cents":1,"AMOUNT_CENTS":2,"currency":"ARS"}}`, 400, "malformed_request"},
		{"update repeated name", "PATCH", "/api/v1/catalog-items/{plain}", `{"name":"A","name":"B"}`, 400, "malformed_request"},
		{"update empty", "PATCH", "/api/v1/catalog-items/{plain}", `{}`, 400, "malformed_request"},
		{"update null name", "PATCH", "/api/v1/catalog-items/{plain}", `{"name":null}`, 400, "malformed_request"},
		{"update null unit", "PATCH", "/api/v1/catalog-items/{plain}", `{"unit":null}`, 400, "malformed_request"},
		{"update amount with decimals", "PATCH", "/api/v1/catalog-items/{plain}", `{"reference_price":{"amount_cents":1.5,"currency":"ARS"}}`, 400, "malformed_request"},
		{"update amount beyond int64", "PATCH", "/api/v1/catalog-items/{plain}", `{"reference_price":{"amount_cents":9223372036854775808,"currency":"ARS"}}`, 400, "malformed_request"},
		{"update unknown field", "PATCH", "/api/v1/catalog-items/{plain}", `{"archived":true}`, 400, "malformed_request"},
		{"update amount beyond 2^53-1", "PATCH", "/api/v1/catalog-items/{plain}", `{"reference_price":{"amount_cents":9007199254740992,"currency":"ARS"}}`, 422, "validation_failed:reference_price.amount_cents/invalid_value"},
		{"update price without amount", "PATCH", "/api/v1/catalog-items/{plain}", `{"reference_price":{"currency":"USD"}}`, 422, "validation_failed:reference_price.amount_cents/required"},
		{"update price without currency", "PATCH", "/api/v1/catalog-items/{plain}", `{"reference_price":{"amount_cents":5}}`, 422, "validation_failed:reference_price.currency/required"},
		{"update currency EUR", "PATCH", "/api/v1/catalog-items/{plain}", `{"reference_price":{"amount_cents":5,"currency":"EUR"}}`, 422, "validation_failed:reference_price.currency/invalid_value"},
		{"update blank name", "PATCH", "/api/v1/catalog-items/{plain}", `{"name":"  "}`, 422, "validation_failed:name/required"},
		{"update unknown unit", "PATCH", "/api/v1/catalog-items/{plain}", `{"unit":"box"}`, 422, "validation_failed:unit/invalid_value"},
		{"update to other without label", "PATCH", "/api/v1/catalog-items/{plain}", `{"unit":"other"}`, 422, "validation_failed:unit_label/required"},
		{"update label to null on other", "PATCH", "/api/v1/catalog-items/{other}", `{"unit_label":null}`, 422, "validation_failed:unit_label/required"},
		// The fields are validated before the item is looked up: an invalid body on a missing one is a 422.
		{"update invalid name of a missing item", "PATCH", "/api/v1/catalog-items/" + uuid.NewString(), `{"name":""}`, 422, "validation_failed:name/required"},
		{"update price without amount of a missing item", "PATCH", "/api/v1/catalog-items/" + uuid.NewString(), `{"reference_price":{"currency":"ARS"}}`, 422, "validation_failed:reference_price.amount_cents/required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := strings.NewReplacer("{plain}", plain.ID, "{other}", other.ID).Replace(tc.path)
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
	// A price object with neither field reports both, so the form can mark both.
	rec := a.json("admin", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{}}`, 422)
	var problem struct {
		Errors []struct{ Field, Code string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range problem.Errors {
		got = append(got, e.Field+"/"+e.Code)
	}
	slices.Sort(got)
	if want := []string{"reference_price.amount_cents/required", "reference_price.currency/required"}; !slices.Equal(got, want) {
		t.Fatalf("errors = %v, want %v", got, want)
	}
}

// T-002-B603 (002/DD-9, DD-15, INV-13, P-9): the effect of each kind of PATCH.
func TestCatalogItemsHTTPUpdate(t *testing.T) {
	a := newStagesAPI(t)
	patch := func(id, body string) catalogItemBody {
		t.Helper()
		return decodeCatalogItem(t, a.json("admin", "PATCH", "/api/v1/catalog-items/"+id, body, 200))
	}
	priced := a.newCatalogItem(`{"name":"Reja","description":"Con pintura","unit":"unit","reference_price":{"amount_cents":100,"currency":"ARS"}}`)
	// null clears the price, and the answer says so.
	if rec := a.json("admin", "PATCH", "/api/v1/catalog-items/"+priced.ID, `{"reference_price":null}`, 200); !strings.Contains(rec.Body.String(), `"reference_price":null`) {
		t.Fatalf("%s", rec.Body.String())
	}
	if got := a.catalogItems("admin", "")[0]; got.ReferencePrice != nil || got.Name != "Reja" || got.Description == nil {
		t.Fatalf("after clearing the price: %+v", got)
	}
	if got := patch(priced.ID, `{"description":null}`); got.Description != nil {
		t.Fatalf("description not cleared: %+v", got)
	}
	// A price of zero is a price, not "to be quoted".
	if got := patch(priced.ID, `{"reference_price":{"amount_cents":0,"currency":"ARS"}}`); !reflect.DeepEqual(got.ReferencePrice, &moneyBody{AmountCents: 0, Currency: "ARS"}) {
		t.Fatalf("zero price: %+v", got)
	}
	// Changing a unit to other needs its label; leaving other discards it.
	if got := patch(priced.ID, `{"unit":"other","unit_label":"jornal"}`); got.Unit != "other" || got.UnitLabel == nil || *got.UnitLabel != "jornal" {
		t.Fatalf("to other: %+v", got)
	}
	if got := patch(priced.ID, `{"unit_label":"día"}`); got.UnitLabel == nil || *got.UnitLabel != "día" {
		t.Fatalf("label only: %+v", got)
	}
	if got := patch(priced.ID, `{"unit":"hour"}`); got.Unit != "hour" || got.UnitLabel != nil {
		t.Fatalf("leaving other: %+v", got)
	}
	// The same values change nothing, updated_at included.
	same := patch(priced.ID, `{"name":"Reja","unit":"hour"}`)
	if again := patch(priced.ID, `{"name":"Reja"}`); !reflect.DeepEqual(again, same) {
		t.Fatalf("no-op changed the item: %+v != %+v", again, same)
	}
}

// T-002-B603 (002/DD-9, DD-15, INV-11): another company's item, a missing one and a malformed id are
// indistinguishable 404s on every operation, and company B does not change.
func TestCatalogItemsHTTPUnknownItemIs404(t *testing.T) {
	a := newStagesAPI(t)
	foreign, err := a.f.Settings.ListCatalogItems(context.Background(), a.f.B.Users["admin"].Principal, true)
	if err != nil || len(foreign) == 0 {
		t.Fatalf("catalog of B: %v %v", foreign, err)
	}
	// The routes exist: an item of the company is found where the others are not.
	own := a.newCatalogItem(`{"name":"Reja","unit":"unit"}`)
	a.do("admin", "GET", "/api/v1/catalog-items/"+own.ID, "", "", 200)
	a.json("admin", "PATCH", "/api/v1/catalog-items/"+own.ID, `{"name":"Reja doble"}`, 200)
	bBefore := a.f.Snapshot(t, a.f.B.ID)
	for _, id := range []string{foreign[0].ID.String(), uuid.NewString(), "no-uuid"} {
		for _, tc := range []struct{ method, suffix, body string }{
			{"GET", "", ""},
			{"PATCH", "", `{"name":"Otra"}`},
			{"POST", "/archive", ""},
			{"POST", "/restore", ""},
		} {
			ct := ""
			if tc.body != "" {
				ct = "application/json"
			}
			rec := a.do("admin", tc.method, "/api/v1/catalog-items/"+id+tc.suffix, ct, tc.body, 404)
			if got := problemCode(t, rec); got != "not_found" {
				t.Fatalf("%s %s%s: code=%s", tc.method, id, tc.suffix, got)
			}
			assertNoCompanyResponseData(t, rec, a.f.B, false)
		}
	}
	assertUnchanged(t, a.f, a.f.B.ID, bBefore)
	// The list never carries the items of B.
	for _, c := range a.catalogItems("admin", "?include_archived=true") {
		if c.ID == foreign[0].ID.String() {
			t.Fatalf("item of B listed: %+v", c)
		}
	}
}

// T-002-B603 (FR-007 de 001, INV-11): the permission is decided before the body is read and before the item is
// looked up; an operator reads but never writes.
func TestCatalogItemsHTTPOperatorCannotWrite(t *testing.T) {
	a := newStagesAPI(t)
	item := a.newCatalogItem(`{"name":"Reja","unit":"unit"}`)
	before := a.f.Snapshot(t, a.f.A.ID)
	for _, tc := range []struct{ method, path string }{
		{"POST", "/api/v1/catalog-items"},
		{"PATCH", "/api/v1/catalog-items/" + item.ID},
		{"POST", "/api/v1/catalog-items/" + item.ID + "/archive"},
		{"POST", "/api/v1/catalog-items/" + item.ID + "/restore"},
		{"PATCH", "/api/v1/catalog-items/no-uuid"},
		{"POST", "/api/v1/catalog-items/" + uuid.NewString() + "/archive"},
	} {
		for _, body := range []string{`{"name":"Nueva","unit":"unit"}`, `{"broken":`, `{}`} {
			rec := a.json("operator", tc.method, tc.path, body, 403)
			if got := problemCode(t, rec); got != "forbidden" {
				t.Fatalf("%s %s: code=%s", tc.method, tc.path, got)
			}
		}
	}
	assertUnchanged(t, a.f, a.f.A.ID, before)
	a.do("operator", "GET", "/api/v1/catalog-items", "", "", 200)
	a.do("operator", "GET", "/api/v1/catalog-items/"+item.ID, "", "", 200)
	a.do("", "GET", "/api/v1/catalog-items", "", "", 401)
	a.do("", "GET", "/api/v1/catalog-items/"+item.ID, "", "", 401)
	a.json("", "POST", "/api/v1/catalog-items", `{"name":"Nueva","unit":"unit"}`, 401)
}

// T-002-B603 (DD-30 and DD-17 de 001): body limit, media type and cross-origin protection apply to the catalog
// writes that take a body.
func TestCatalogItemsHTTPTransportRules(t *testing.T) {
	a := newStagesAPI(t)
	item := a.newCatalogItem(`{"name":"Reja","unit":"unit"}`)
	oversize := `{"name":"Nueva","unit":"unit","pad":"` + strings.Repeat("x", 64*1024) + `"}`
	before := a.f.Snapshot(t, a.f.A.ID)
	for _, tc := range []struct{ method, path, body string }{
		{"POST", "/api/v1/catalog-items", `{"name":"Nueva","unit":"unit"}`},
		{"PATCH", "/api/v1/catalog-items/" + item.ID, `{"name":"Otra"}`},
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
		{"POST", "/api/v1/catalog-items", `{"name":"Nueva","unit":"unit"}`},
		{"PATCH", "/api/v1/catalog-items/" + item.ID, `{"name":"Otra"}`},
		{"POST", "/api/v1/catalog-items/" + item.ID + "/archive", ""},
		{"POST", "/api/v1/catalog-items/" + item.ID + "/restore", ""},
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

// T-002-B603 (plan 9.1): a repeated member is detected by the key it decodes to, and a string or a nested value
// that only looks like a repeated key is not one.
func TestCatalogItemsHTTPRepeatedMemberLookalikes(t *testing.T) {
	a := newStagesAPI(t)
	created := a.newCatalogItem(`{"name":"A \"amount_cents\"","description":"{\"name\":1,\"name\":2}","unit":"unit","reference_price":{"amount_cents":7,"currency":"ARS"}}`)
	if created.Name != `A "amount_cents"` || created.ReferencePrice == nil || created.ReferencePrice.AmountCents != 7 {
		t.Fatalf("created: %+v", created)
	}
	// An escaped spelling of a key that is not repeated is that key.
	got := decodeCatalogItem(t, a.json("admin", "PATCH", "/api/v1/catalog-items/"+created.ID, `{"reference_price":{"amount_c\u0065nts":8,"currency":"ARS"}}`, 200))
	if got.ReferencePrice == nil || got.ReferencePrice.AmountCents != 8 {
		t.Fatalf("patched: %+v", got)
	}
}

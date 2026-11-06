package industrytemplate_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/fdelillo/crm/internal/industrytemplate"
	"github.com/fdelillo/crm/internal/testsupport/contract"
	"github.com/go-chi/chi/v5"
)

// commonLabels are the default names shared by both templates (spec "Plantillas de rubro").
var commonLabels = industrytemplate.TemplateLabels{
	Project:  industrytemplate.TemplateLabel{Singular: "Proyecto", Plural: "Proyectos", Gender: "masculine"},
	Customer: industrytemplate.TemplateLabel{Singular: "Cliente", Plural: "Clientes", Gender: "masculine"},
	Quote:    industrytemplate.TemplateLabel{Singular: "Presupuesto", Plural: "Presupuestos", Gender: "masculine"},
}

// expectedTemplates is written by hand from the spec ("Plantillas de rubro", FR-006, P-4), on purpose
// not derived from catalog.json: the test checks the file against the spec, not against itself.
// The system categories are not here: settings owns them (002/DD-2).
var expectedTemplates = []industrytemplate.Template{
	{
		Code: "generic", Name: "Genérico", Version: 2,
		Labels: commonLabels,
		Stages: []industrytemplate.TemplateStage{
			{Name: "Presupuesto", Type: "open", Color: "blue"},
			{Name: "Aprobado", Type: "open", Color: "teal"},
			{Name: "En curso", Type: "open", Color: "yellow"},
			{Name: "Finalizado", Type: "won", Color: "green"},
			{Name: "Perdido", Type: "lost", Color: "gray"},
		},
		Categories: []industrytemplate.TemplateCategory{
			{Name: "Materiales", Type: "expense"},
			{Name: "Mano de obra", Type: "expense"},
			{Name: "Flete", Type: "expense"},
			{Name: "Alquiler", Type: "expense"},
			{Name: "Servicios", Type: "expense"},
			{Name: "Impuestos", Type: "expense"},
			{Name: "Otros gastos", Type: "expense"},
			{Name: "Otros ingresos", Type: "income"},
		},
		CatalogItems:  []industrytemplate.TemplateCatalogItem{},
		QuoteDefaults: industrytemplate.TemplateQuoteDefaults{ValidityDays: 15, Terms: ""},
	},
	{
		Code: "aluminum_carpentry", Name: "Carpintería de aluminio", Version: 2,
		Labels: commonLabels,
		Stages: []industrytemplate.TemplateStage{
			{Name: "Presupuesto", Type: "open", Color: "blue"},
			{Name: "Aprobado", Type: "open", Color: "teal"},
			{Name: "En producción", Type: "open", Color: "orange"},
			{Name: "Instalación", Type: "open", Color: "purple"},
			{Name: "Finalizado", Type: "won", Color: "green"},
			{Name: "Perdido", Type: "lost", Color: "gray"},
		},
		Categories: []industrytemplate.TemplateCategory{
			{Name: "Perfiles de aluminio", Type: "expense"},
			{Name: "Vidrios", Type: "expense"},
			{Name: "Accesorios", Type: "expense"},
			{Name: "Flete", Type: "expense"},
			{Name: "Mano de obra", Type: "expense"},
			{Name: "Alquiler", Type: "expense"},
			{Name: "Servicios", Type: "expense"},
			{Name: "Impuestos", Type: "expense"},
			{Name: "Otros ingresos", Type: "income"},
		},
		CatalogItems: []industrytemplate.TemplateCatalogItem{
			{Name: "Ventana corrediza", Unit: "square_meter"},
			{Name: "Puerta de abrir", Unit: "unit"},
			{Name: "Mampara de baño", Unit: "unit"},
			{Name: "Colocación", Unit: "unit"},
		},
		QuoteDefaults: industrytemplate.TemplateQuoteDefaults{
			ValidityDays: 15,
			Terms:        "Seña del 50 % al aprobar el presupuesto y saldo contra entrega.",
		},
	},
}

// T-002-B303 (FR-006, P-4): All() has exactly the content of the spec, in the spec's order.
func TestAllHasTheContentOfTheSpec(t *testing.T) {
	got := industrytemplate.All()
	if len(got) != len(expectedTemplates) {
		t.Fatalf("catalog has %d templates, want %d", len(got), len(expectedTemplates))
	}
	for i, want := range expectedTemplates {
		if !reflect.DeepEqual(got[i], want) {
			t.Errorf("template %s:\n got %+v\nwant %+v", want.Code, got[i], want)
		}
	}
}

// T-002-B303 (DD-2): Lookup returns the same content as All(), and nothing for an unknown code.
func TestLookupReturnsTheTemplateWithItsContent(t *testing.T) {
	for _, want := range expectedTemplates {
		got, ok := industrytemplate.Lookup(want.Code)
		if !ok || !reflect.DeepEqual(got, want) {
			t.Errorf("Lookup(%q) = %+v, %v", want.Code, got, ok)
		}
	}
	if _, ok := industrytemplate.Lookup("inexistente"); ok {
		t.Fatal("unknown template found")
	}
}

// T-002-B303 (DD-2): the catalog is shared by every registration, so a caller that edits what All()
// or Lookup returned (including its lists) must not change what the next call returns.
func TestResultsDoNotShareStateWithTheCatalog(t *testing.T) {
	first := industrytemplate.All()
	if len(first) != 2 || len(first[1].Stages) < 2 || len(first[1].Categories) == 0 || len(first[1].CatalogItems) == 0 {
		t.Fatalf("the catalog has no content to mutate: %+v", first)
	}
	first[1].Stages[0].Name = "mutated"
	first[1].Categories[0].Name = "mutated"
	first[1].CatalogItems[0].Name = "mutated"
	looked, _ := industrytemplate.Lookup("aluminum_carpentry")
	looked.Stages[1].Name = "mutated"
	if got := industrytemplate.All(); !reflect.DeepEqual(got, expectedTemplates) {
		t.Errorf("catalog changed through a returned copy:\n got %+v", got)
	}
}

// T-002-B303 (contract of 001, DD-2): the public endpoint does not change with the content: it lists
// only code and name, and nothing of the new fields, byte for byte.
func TestPublicHandlerListsOnlyCodeAndName(t *testing.T) {
	r := chi.NewRouter()
	industrytemplate.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/industry-templates", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	contract.Default(t).RequireRecorded(t, req, rec)
	const want = `{"items":[{"code":"generic","name":"Genérico"},{"code":"aluminum_carpentry","name":"Carpintería de aluminio"}]}`
	if got := rec.Body.String(); got != want {
		t.Errorf("body:\n got %s\nwant %s", got, want)
	}
	var raw struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	for _, item := range raw.Items {
		if len(item) != 2 {
			t.Errorf("item has %d keys, want only code and name: %v", len(item), item)
		}
	}
}

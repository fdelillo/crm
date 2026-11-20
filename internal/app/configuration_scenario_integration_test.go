//go:build integration

package app_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/outbox"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/settings"
	"github.com/fdelillo/crm/internal/tenant"
	"github.com/fdelillo/crm/internal/testsupport/apitest"
	"github.com/fdelillo/crm/internal/testsupport/contract"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
)

// configurationClient is one session against the real router: its own cookie jar, every request and response
// validated against the contracts.
type configurationClient struct {
	t *testing.T
	s *apitest.Server
}

func (c configurationClient) do(method, path, body string, want int) []byte {
	c.t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, c.s.URL+path, strings.NewReader(body))
	if err != nil {
		c.t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.s.Client.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatal(err)
	}
	if resp.StatusCode != want {
		c.t.Fatalf("%s %s: %d, want %d: %s", method, path, resp.StatusCode, want, raw)
	}
	return raw
}

func decodeInto[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decoding %s: %v", raw, err)
	}
	return out
}

type stageView struct {
	Name, Type, Color string
	Position          int
}

func stageViews(t *testing.T, raw []byte) []stageView {
	t.Helper()
	var out []stageView
	for _, s := range decodeInto[struct{ Items []stageBody }](t, raw).Items {
		out = append(out, stageView{s.Name, s.Type, s.Color, s.Position})
	}
	return out
}

type catalogView struct {
	Name, Unit string
	UnitLabel  *string
	Price      *moneyBody
	Archived   bool
}

func catalogViews(t *testing.T, raw []byte) []catalogView {
	t.Helper()
	var out []catalogView
	for _, c := range decodeInto[struct{ Items []catalogItemBody }](t, raw).Items {
		out = append(out, catalogView{c.Name, c.Unit, c.UnitLabel, c.ReferencePrice, c.Archived})
	}
	return out
}

// T-002-B801 (SC-001, FR-001 to FR-008, INV-9, R-9): a company of another industry ("herrería", registered
// with the generic template) is configured only through the API, with no code specific to it, and an invited
// operator reads all of it but cannot change any of it.
func TestConfigureACompanyOfAnotherIndustryThroughTheAPI(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	c := clock.Real{}
	runner := db.NewTxRunner(pgtest.AppPool(t), db.WithLogger(logger))
	hasher := password.NewHasher(2)
	recorder := audit.NewRecorder()
	users := identity.NewService(runner, outbox.NewEnqueuer(c), c, 24*time.Hour, 7*24*time.Hour,
		identity.WithAuthentication(hasher, recorder, []byte("0123456789abcdef0123456789abcdef"), logger))
	companies := tenant.NewService(runner, users, settings.NewTemplateSeeder(c), hasher, recorder, logger)
	configuration := settings.NewService(runner, recorder, c, logger)
	root := app.NewRootHandler(app.RootDeps{API: app.BuildAPIRouter(users, companies, configuration, c, logger),
		Liveness: app.LivenessHandler(), Readiness: app.ReadinessPlaceholder(), SPA: app.SPAUnavailableHandler()},
		app.NewCommonMiddleware(logger, true, nil))
	newClient := func() configurationClient {
		s := apitest.NewServer(t, root)
		contract.Default(t).Wrap(t, s.Client)
		return configurationClient{t: t, s: s}
	}
	admin, operator := newClient(), newClient()

	// Registration with the generic template; the administrator confirms the email so that it can invite (DD-46).
	email := uuid.NewString() + "@example.com"
	signup := decodeInto[map[string]any](t, admin.do("POST", "/api/v1/auth/signup",
		`{"name":"Ana","email":"`+email+`","password":"valid-password","company_name":"Herrería Sur","base_currency":"ARS","industry_template_code":"generic"}`, 201))
	tenantID := uuid.MustParse(signup["tenant"].(map[string]any)["id"].(string))
	admin.do("POST", "/api/v1/auth/email-verification/confirm",
		`{"token":"`+outboxToken(t, runner, tenantID, email, "email_verification")+`"}`, 204)

	// The generic starting point.
	if got := stageViews(t, admin.do("GET", "/api/v1/stages", "", 200)); !reflect.DeepEqual(got, []stageView{
		{"Presupuesto", "open", "blue", 1}, {"Aprobado", "open", "teal", 2}, {"En curso", "open", "yellow", 3},
		{"Finalizado", "won", "green", 4}, {"Perdido", "lost", "gray", 5}}) {
		t.Fatalf("starting pipeline: %+v", got)
	}

	// Names: Proyecto becomes "Trabajo"/"Trabajos", masculine.
	labels := labelsBody{
		Project:  entityLabelBody{"Trabajo", "Trabajos", "masculine"},
		Customer: genericLabels.Customer,
		Quote:    genericLabels.Quote,
	}
	if got := decodeInto[labelsBody](t, admin.do("PUT", labelsPath, labelsJSON(labels), 200)); got != labels {
		t.Fatalf("PUT labels answered %+v", got)
	}
	if got := decodeInto[labelsBody](t, admin.do("GET", labelsPath, "", 200)); got != labels {
		t.Fatalf("GET labels: %+v", got)
	}

	// Pipeline: rename "En curso", add "Colocación" at 4, recolor, move "Aprobado".
	stageID := func(name string) string {
		t.Helper()
		for _, s := range decodeInto[struct{ Items []stageBody }](t, admin.do("GET", "/api/v1/stages", "", 200)).Items {
			if s.Name == name {
				return s.ID
			}
		}
		t.Fatalf("no stage %q", name)
		return ""
	}
	admin.do("PATCH", "/api/v1/stages/"+stageID("En curso"), `{"name":"En taller","color":"orange"}`, 200)
	admin.do("POST", "/api/v1/stages", `{"name":"Colocación","type":"open","color":"purple","position":4}`, 201)
	admin.do("PATCH", "/api/v1/stages/"+stageID("Presupuesto"), `{"color":"gray"}`, 200)
	admin.do("PATCH", "/api/v1/stages/"+stageID("Perdido"), `{"color":"red"}`, 200)
	admin.do("POST", "/api/v1/stages/"+stageID("Aprobado")+"/move", `{"position":3}`, 200)
	wantStages := []stageView{
		{"Presupuesto", "open", "gray", 1}, {"En taller", "open", "orange", 2}, {"Aprobado", "open", "teal", 3},
		{"Colocación", "open", "purple", 4}, {"Finalizado", "won", "green", 5}, {"Perdido", "lost", "red", 6}}
	if got := stageViews(t, admin.do("GET", "/api/v1/stages", "", 200)); !reflect.DeepEqual(got, wantStages) {
		t.Fatalf("pipeline: %+v\nwant %+v", got, wantStages)
	}

	// Categories: three new expense ones, "Alquiler" archived.
	for _, name := range []string{"Hierros", "Pintura", "Electrodos"} {
		admin.do("POST", "/api/v1/categories", `{"name":"`+name+`","type":"expense"}`, 201)
	}
	admin.do("POST", "/api/v1/categories/"+decodeInto[categoryBody](t, findCategory(t, admin, "Alquiler")).ID+"/archive", "", 200)
	wantCategories := []string{"income:Aportes de socios", "income:Cobros de clientes", "income:Otros ingresos",
		"expense:Pagos a empleados", "expense:Pagos a proveedores", "expense:Retiros de socios",
		"expense:Electrodos", "expense:Flete", "expense:Hierros", "expense:Impuestos", "expense:Mano de obra",
		"expense:Materiales", "expense:Otros gastos", "expense:Pintura", "expense:Servicios"}
	if got := categoryNames(decodeInto[struct{ Items []categoryBody }](t, admin.do("GET", "/api/v1/categories", "", 200)).Items); !reflect.DeepEqual(got, wantCategories) {
		t.Fatalf("categories: %v\nwant %v", got, wantCategories)
	}

	// Catalog: a square-meter item without price, a unit item in USD, a free unit in ARS.
	admin.do("POST", "/api/v1/catalog-items", `{"name":"Reja de ventana","unit":"square_meter"}`, 201)
	admin.do("POST", "/api/v1/catalog-items", `{"name":"Portón corredizo","unit":"unit","reference_price":{"amount_cents":450000,"currency":"USD"}}`, 201)
	admin.do("POST", "/api/v1/catalog-items", `{"name":"Soldadura","unit":"other","unit_label":"jornada","reference_price":{"amount_cents":2500000,"currency":"ARS"}}`, 201)
	jornada := "jornada"
	wantCatalog := []catalogView{
		{"Portón corredizo", "unit", nil, &moneyBody{450000, "USD"}, false},
		{"Reja de ventana", "square_meter", nil, nil, false},
		{"Soldadura", "other", &jornada, &moneyBody{2500000, "ARS"}, false}}
	if got := catalogViews(t, admin.do("GET", "/api/v1/catalog-items", "", 200)); !reflect.DeepEqual(got, wantCatalog) {
		t.Fatalf("catalog: %+v\nwant %+v", got, wantCatalog)
	}

	// Quote defaults.
	terms := "Seña del 50 % al aprobar y saldo contra entrega."
	wantDefaults := quoteDefaultsBody{ValidityDays: 30, Terms: &terms}
	if got := decodeInto[quoteDefaultsBody](t, admin.do("PUT", quoteDefaultsPath, `{"validity_days":30,"terms":"`+terms+`"}`, 200)); !reflect.DeepEqual(got, wantDefaults) {
		t.Fatalf("PUT quote defaults answered %+v", got)
	}

	// An invited operator (flow of 001) is a member of the same company.
	operatorEmail := uuid.NewString() + "@example.com"
	admin.do("POST", "/api/v1/users/invitations", `{"email":"`+operatorEmail+`","role":"operator"}`, 201)
	operator.do("POST", "/api/v1/auth/invitations/accept",
		`{"token":"`+invitationHTTPToken(t, runner, tenantID, operatorEmail)+`","name":"Beto","password":"valid-password"}`, 201)

	itemID := decodeInto[struct{ Items []catalogItemBody }](t, admin.do("GET", "/api/v1/catalog-items", "", 200)).Items[0].ID
	reads := []string{"/api/v1/stages", "/api/v1/categories", "/api/v1/catalog-items", "/api/v1/catalog-items/" + itemID, labelsPath, quoteDefaultsPath}
	adminReads := make([][]byte, len(reads))
	for i, path := range reads {
		adminReads[i] = admin.do("GET", path, "", 200)
		if got := operator.do("GET", path, "", 200); string(got) != string(adminReads[i]) {
			t.Fatalf("operator reads %s: %s\nadmin reads %s", path, got, adminReads[i])
		}
	}
	if got := stageViews(t, adminReads[0]); !reflect.DeepEqual(got, wantStages) {
		t.Fatalf("operator pipeline: %+v", got)
	}
	if got := decodeInto[labelsBody](t, adminReads[4]); got != labels {
		t.Fatalf("operator labels: %+v", got)
	}
	if got := decodeInto[quoteDefaultsBody](t, adminReads[5]); !reflect.DeepEqual(got, wantDefaults) {
		t.Fatalf("operator quote defaults: %+v", got)
	}

	// The operator cannot write any resource, and nothing changes.
	stage := stageID("Aprobado")
	category := decodeInto[categoryBody](t, findCategory(t, admin, "Hierros")).ID
	for _, w := range []struct{ method, path, body string }{
		{"POST", "/api/v1/stages", `{"name":"Nueva","type":"open","color":"gray"}`},
		{"PATCH", "/api/v1/stages/" + stage, `{"name":"Otra"}`},
		{"POST", "/api/v1/stages/" + stage + "/move", `{"position":1}`},
		{"DELETE", "/api/v1/stages/" + stage, ""},
		{"POST", "/api/v1/categories", `{"name":"Nueva","type":"income"}`},
		{"PATCH", "/api/v1/categories/" + category, `{"name":"Otra"}`},
		{"POST", "/api/v1/categories/" + category + "/archive", ""},
		{"POST", "/api/v1/categories/" + category + "/restore", ""},
		{"POST", "/api/v1/catalog-items", `{"name":"Nuevo","unit":"unit"}`},
		{"PATCH", "/api/v1/catalog-items/" + itemID, `{"name":"Otro"}`},
		{"POST", "/api/v1/catalog-items/" + itemID + "/archive", ""},
		{"POST", "/api/v1/catalog-items/" + itemID + "/restore", ""},
		{"PUT", labelsPath, labelsJSON(genericLabels)},
		{"PUT", quoteDefaultsPath, `{"validity_days":1,"terms":null}`},
	} {
		if got := problemCodeOf(t, operator.do(w.method, w.path, w.body, 403)); got != "forbidden" {
			t.Fatalf("%s %s: code=%s", w.method, w.path, got)
		}
	}
	for i, path := range reads {
		if got := admin.do("GET", path, "", 200); string(got) != string(adminReads[i]) {
			t.Fatalf("%s changed after the operator's attempts: %s\nbefore %s", path, got, adminReads[i])
		}
	}
}

// findCategory returns the body of the expense category called name, archived or not.
func findCategory(t *testing.T, c configurationClient, name string) []byte {
	t.Helper()
	for _, cat := range decodeInto[struct{ Items []json.RawMessage }](t, c.do("GET", "/api/v1/categories?include_archived=true", "", 200)).Items {
		if b := decodeInto[categoryBody](t, cat); b.Name == name && b.Type == "expense" {
			return cat
		}
	}
	t.Fatalf("no expense category %q", name)
	return nil
}

func problemCodeOf(t *testing.T, raw []byte) string {
	t.Helper()
	return decodeInto[struct{ Code string }](t, raw).Code
}

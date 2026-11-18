//go:build integration

package app_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/testsupport/fixture/e2e"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
)

// stagesAPI drives the real router over the isolation scenario: company A is "generic" (Presupuesto, Aprobado,
// En curso, Finalizado, Perdido) and company B is "aluminum_carpentry". Every request is also validated against
// the contract by isolationRequest.
type stagesAPI struct {
	t *testing.T
	f *e2e.Scenario
	h http.Handler
}

func newStagesAPI(t *testing.T) *stagesAPI {
	t.Helper()
	f := e2e.New(t, pgtest.AppPool(t))
	root := app.NewRootHandler(app.RootDeps{API: app.BuildAPIRouter(f.Users, f.Companies, f.Settings, f.Clock, f.Logger),
		Liveness: app.LivenessHandler(), Readiness: app.ReadinessPlaceholder(), SPA: app.SPAUnavailableHandler()},
		app.NewCommonMiddleware(f.Logger, true, nil))
	return &stagesAPI{t: t, f: f, h: root}
}

type stageBody struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Color    string `json:"color"`
	Position int    `json:"position"`
}

func (a *stagesAPI) do(who, method, path, ct, body string, want int) *httptest.ResponseRecorder {
	a.t.Helper()
	var cookie *http.Cookie
	if who != "" {
		cookie = a.f.A.Users[who].Cookie
	}
	rec := isolationRequest(a.t, a.h, context.Background(), cookie, method, path, ct, []byte(body), "", want)
	if rec.Header().Get("Cache-Control") != "no-store" {
		a.t.Fatalf("%s %s: Cache-Control=%q, want no-store", method, path, rec.Header().Get("Cache-Control"))
	}
	return rec
}

func (a *stagesAPI) json(who, method, path, body string, want int) *httptest.ResponseRecorder {
	a.t.Helper()
	return a.do(who, method, path, "application/json", body, want)
}

func (a *stagesAPI) list(who string) []stageBody {
	a.t.Helper()
	var out struct {
		Items []stageBody `json:"items"`
	}
	if err := json.Unmarshal(a.do(who, "GET", "/api/v1/stages", "", "", 200).Body.Bytes(), &out); err != nil {
		a.t.Fatal(err)
	}
	return out.Items
}

// order is the pipeline as names in position order; it also checks that positions are dense from 1.
func order(t *testing.T, stages []stageBody) []string {
	t.Helper()
	names := make([]string, len(stages))
	for i, s := range stages {
		if s.Position != i+1 {
			t.Fatalf("stage %q has position %d, want %d", s.Name, s.Position, i+1)
		}
		names[i] = s.Name
	}
	return names
}

func (a *stagesAPI) id(name string) string {
	a.t.Helper()
	for _, s := range a.list("admin") {
		if s.Name == name {
			return s.ID
		}
	}
	a.t.Fatalf("no stage %q", name)
	return ""
}

func decodeStage(t *testing.T, rec *httptest.ResponseRecorder) stageBody {
	t.Helper()
	var s stageBody
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func problemCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var p struct {
		Code   string                         `json:"code"`
		Errors []struct{ Field, Code string } `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if len(p.Errors) > 0 {
		return p.Code + ":" + p.Errors[0].Field + "/" + p.Errors[0].Code
	}
	return p.Code
}

// T-002-B403 (Historia 1, 002/DD-18): the five operations walk the independent test of the phase and every
// response matches the contract.
func TestStagesHTTPPipelineScenario(t *testing.T) {
	a := newStagesAPI(t)
	for _, who := range []string{"admin", "operator"} {
		if got := order(t, a.list(who)); !reflect.DeepEqual(got, []string{"Presupuesto", "Aprobado", "En curso", "Finalizado", "Perdido"}) {
			t.Fatalf("%s sees %v", who, got)
		}
	}
	// Move "En curso" to 1; the answer is the whole list.
	var moved struct{ Items []stageBody }
	if err := json.Unmarshal(a.json("admin", "POST", "/api/v1/stages/"+a.id("En curso")+"/move", `{"position":1}`, 200).Body.Bytes(), &moved); err != nil {
		t.Fatal(err)
	}
	if got := order(t, moved.Items); !reflect.DeepEqual(got, []string{"En curso", "Presupuesto", "Aprobado", "Finalizado", "Perdido"}) {
		t.Fatalf("move returned %v", got)
	}
	// Create "Instalación" before "Finalizado" (position 4 of 5).
	created := decodeStage(t, a.json("admin", "POST", "/api/v1/stages", `{"name":"  Instalación ","type":"open","color":"teal","position":4}`, 201))
	if created.Name != "Instalación" || created.Type != "open" || created.Color != "teal" || created.Position != 4 || created.ID == "" {
		t.Fatalf("created %+v", created)
	}
	// "Finalizado" is the only won stage.
	if got := problemCode(t, a.json("admin", "DELETE", "/api/v1/stages/"+a.id("Finalizado"), "", 409)); got != "invalid_state" {
		t.Fatalf("code=%s", got)
	}
	// PATCH changes the present fields; the same values again are a 200 with the stage.
	patched := decodeStage(t, a.json("admin", "PATCH", "/api/v1/stages/"+created.ID, `{"name":"Obra","color":"orange"}`, 200))
	if patched.ID != created.ID || patched.Name != "Obra" || patched.Color != "orange" || patched.Type != "open" || patched.Position != 4 {
		t.Fatalf("patched %+v", patched)
	}
	if again := decodeStage(t, a.json("admin", "PATCH", "/api/v1/stages/"+created.ID, `{"name":"Obra","color":"orange"}`, 200)); again != patched {
		t.Fatalf("no-op patch returned %+v, want %+v", again, patched)
	}
	if got := order(t, a.list("operator")); !reflect.DeepEqual(got, []string{"En curso", "Presupuesto", "Aprobado", "Obra", "Finalizado", "Perdido"}) {
		t.Fatalf("final order %v", got)
	}
	a.json("admin", "DELETE", "/api/v1/stages/"+created.ID, "", 204)
	if got := order(t, a.list("operator")); !reflect.DeepEqual(got, []string{"En curso", "Presupuesto", "Aprobado", "Finalizado", "Perdido"}) {
		t.Fatalf("after delete %v", got)
	}
}

// T-002-B403 (002/DD-12, DD-18, INV-11): validation, malformed bodies and pipeline conflicts of each operation.
func TestStagesHTTPErrors(t *testing.T) {
	a := newStagesAPI(t)
	missing := uuid.NewString()
	for _, tc := range []struct {
		name, method, path, body string
		want                     int
		code                     string
	}{
		{"create empty name", "POST", "/api/v1/stages", `{"name":"","type":"open","color":"gray"}`, 422, "validation_failed:name/required"},
		{"create bad type", "POST", "/api/v1/stages", `{"name":"Nueva","type":"closed","color":"gray"}`, 422, "validation_failed:type/invalid_value"},
		{"create bad color", "POST", "/api/v1/stages", `{"name":"Nueva","type":"open","color":"pink"}`, 422, "validation_failed:color/invalid_value"},
		{"create position 0", "POST", "/api/v1/stages", `{"name":"Nueva","type":"open","color":"gray","position":0}`, 422, "validation_failed:position/invalid_value"},
		{"create position n+2", "POST", "/api/v1/stages", `{"name":"Nueva","type":"open","color":"gray","position":7}`, 422, "validation_failed:position/invalid_value"},
		{"create unknown field", "POST", "/api/v1/stages", `{"name":"Nueva","type":"open","color":"gray","extra":1}`, 400, "malformed_request"},
		{"create position as string", "POST", "/api/v1/stages", `{"name":"Nueva","type":"open","color":"gray","position":"2"}`, 400, "malformed_request"},
		{"create position with decimals", "POST", "/api/v1/stages", `{"name":"Nueva","type":"open","color":"gray","position":2.5}`, 400, "malformed_request"},
		{"create null name", "POST", "/api/v1/stages", `{"name":null,"type":"open","color":"gray"}`, 400, "malformed_request"},
		{"create null position", "POST", "/api/v1/stages", `{"name":"Nueva","type":"open","color":"gray","position":null}`, 400, "malformed_request"},
		{"create broken json", "POST", "/api/v1/stages", `{"name":`, 400, "malformed_request"},
		{"create repeated name", "POST", "/api/v1/stages", `{"name":"presupuesto","type":"open","color":"gray"}`, 409, "name_taken"},
		{"create won first", "POST", "/api/v1/stages", `{"name":"Nueva","type":"won","color":"green","position":1}`, 409, "invalid_state"},
		{"create lost first", "POST", "/api/v1/stages", `{"name":"Nueva","type":"lost","color":"red","position":1}`, 409, "invalid_state"},
		{"update empty", "PATCH", "/api/v1/stages/{id}", `{}`, 400, "malformed_request"},
		{"update null name", "PATCH", "/api/v1/stages/{id}", `{"name":null}`, 400, "malformed_request"},
		{"update null color with a valid name", "PATCH", "/api/v1/stages/{id}", `{"name":"Otra","color":null}`, 400, "malformed_request"},
		{"update unknown field", "PATCH", "/api/v1/stages/{id}", `{"position":2}`, 400, "malformed_request"},
		{"update bad type", "PATCH", "/api/v1/stages/{id}", `{"type":"closed"}`, 422, "validation_failed:type/invalid_value"},
		{"update empty name", "PATCH", "/api/v1/stages/{id}", `{"name":"   "}`, 422, "validation_failed:name/required"},
		// Fields are validated before the stage is looked up: an invalid body on a missing stage is a 422.
		{"update invalid field of a missing stage", "PATCH", "/api/v1/stages/" + missing, `{"color":"pink"}`, 422, "validation_failed:color/invalid_value"},
		{"update repeated name", "PATCH", "/api/v1/stages/{id}", `{"name":"aprobado"}`, 409, "name_taken"},
		{"update only won to open", "PATCH", "/api/v1/stages/{won}", `{"type":"open"}`, 409, "invalid_state"},
		{"update first to won", "PATCH", "/api/v1/stages/{id}", `{"type":"won"}`, 409, "invalid_state"},
		{"delete only won", "DELETE", "/api/v1/stages/{won}", "", 409, "invalid_state"},
		{"move to 0", "POST", "/api/v1/stages/{id}/move", `{"position":0}`, 422, "validation_failed:position/invalid_value"},
		{"move to 99", "POST", "/api/v1/stages/{id}/move", `{"position":99}`, 422, "validation_failed:position/invalid_value"},
		{"move to n+1", "POST", "/api/v1/stages/{id}/move", `{"position":6}`, 422, "validation_failed:position/invalid_value"},
		{"move without position", "POST", "/api/v1/stages/{id}/move", `{}`, 422, "validation_failed:position/required"},
		{"move position as string", "POST", "/api/v1/stages/{id}/move", `{"position":"1"}`, 400, "malformed_request"},
		{"move null position", "POST", "/api/v1/stages/{id}/move", `{"position":null}`, 400, "malformed_request"},
		{"move unknown field", "POST", "/api/v1/stages/{id}/move", `{"position":1,"to":2}`, 400, "malformed_request"},
		{"move lost to first", "POST", "/api/v1/stages/{lost}/move", `{"position":1}`, 409, "invalid_state"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := strings.NewReplacer("{id}", a.id("Presupuesto"), "{won}", a.id("Finalizado"), "{lost}", a.id("Perdido")).Replace(tc.path)
			before := a.f.Snapshot(t, a.f.A.ID)
			rec := a.json("admin", tc.method, path, tc.body, tc.want)
			if got := problemCode(t, rec); got != tc.code {
				t.Fatalf("problem=%s, want %s", rec.Body.String(), tc.code)
			}
			assertUnchanged(t, a.f, a.f.A.ID, before)
		})
	}
	if got := order(t, a.list("admin")); !reflect.DeepEqual(got, []string{"Presupuesto", "Aprobado", "En curso", "Finalizado", "Perdido"}) {
		t.Fatalf("a rejected request changed the pipeline: %v", got)
	}
}

// T-002-B403 (002/DD-15, INV-11): another company's stage, a missing one, a malformed id and a deleted one are
// indistinguishable 404s, and company B does not change.
func TestStagesHTTPUnknownStageIs404(t *testing.T) {
	a := newStagesAPI(t)
	foreign, err := a.f.Settings.ListStages(context.Background(), a.f.B.Users["admin"].Principal)
	if err != nil || len(foreign) == 0 {
		t.Fatalf("stages of B: %v %v", foreign, err)
	}
	deleted := a.id("Aprobado")
	a.json("admin", "DELETE", "/api/v1/stages/"+deleted, "", 204)
	bBefore := a.f.Snapshot(t, a.f.B.ID)
	for _, id := range []string{foreign[0].ID.String(), uuid.NewString(), "no-uuid", deleted} {
		for _, tc := range []struct{ method, suffix, body string }{
			{"PATCH", "", `{"name":"Otra"}`},
			{"DELETE", "", ""},
			{"POST", "/move", `{"position":1}`},
		} {
			rec := a.json("admin", tc.method, "/api/v1/stages/"+id+tc.suffix, tc.body, 404)
			if got := problemCode(t, rec); got != "not_found" {
				t.Fatalf("%s %s: code=%s", tc.method, id, got)
			}
			assertNoCompanyResponseData(t, rec, a.f.B, false)
		}
	}
	assertUnchanged(t, a.f, a.f.B.ID, bBefore)
	if got := order(t, a.list("admin")); !reflect.DeepEqual(got, []string{"Presupuesto", "En curso", "Finalizado", "Perdido"}) {
		t.Fatalf("pipeline of A: %v", got)
	}
}

// T-002-B403 (FR-007 de 001, INV-11): the permission is decided before the body is read and before the stage is
// looked up; an operator reads but never writes.
func TestStagesHTTPOperatorCannotWrite(t *testing.T) {
	a := newStagesAPI(t)
	id := a.id("Aprobado")
	before := a.f.Snapshot(t, a.f.A.ID)
	for _, tc := range []struct{ method, path string }{
		{"POST", "/api/v1/stages"},
		{"PATCH", "/api/v1/stages/" + id},
		{"DELETE", "/api/v1/stages/" + id},
		{"POST", "/api/v1/stages/" + id + "/move"},
		{"PATCH", "/api/v1/stages/no-uuid"},
		{"DELETE", "/api/v1/stages/" + uuid.NewString()},
	} {
		for _, body := range []string{`{"name":"Nueva","type":"open","color":"gray","position":1}`, `{"broken":`, `{}`} {
			rec := a.json("operator", tc.method, tc.path, body, 403)
			if got := problemCode(t, rec); got != "forbidden" {
				t.Fatalf("%s %s: code=%s", tc.method, tc.path, got)
			}
		}
	}
	assertUnchanged(t, a.f, a.f.A.ID, before)
	a.do("operator", "GET", "/api/v1/stages", "", "", 200)
	a.do("", "GET", "/api/v1/stages", "", "", 401)
	a.json("", "POST", "/api/v1/stages", `{"name":"Nueva","type":"open","color":"gray"}`, 401)
}

// T-002-B403 (DD-30 and DD-17 de 001): body limit, media type and cross-origin protection apply to the stage writes.
func TestStagesHTTPTransportRules(t *testing.T) {
	a := newStagesAPI(t)
	id := a.id("Aprobado")
	oversize := `{"name":"Nueva","type":"open","color":"gray","pad":"` + strings.Repeat("x", 64*1024) + `"}`
	for _, tc := range []struct{ method, path, body string }{
		{"POST", "/api/v1/stages", `{"name":"Nueva","type":"open","color":"gray"}`},
		{"PATCH", "/api/v1/stages/" + id, `{"name":"Otra"}`},
		{"POST", "/api/v1/stages/" + id + "/move", `{"position":1}`},
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
	if got := order(t, a.list("admin")); len(got) != 5 {
		t.Fatalf("pipeline changed: %v", got)
	}
}

// T-002-B403 (002/INV-1): a company that lost its settings row is a bug, never a 404 that would look like a
// missing stage.
func TestStagesHTTPMissingSettingsRowIs500(t *testing.T) {
	a := newStagesAPI(t)
	if _, err := pgtest.SuperuserPool(t).Exec(context.Background(), `DELETE FROM app.tenant_settings WHERE tenant_id=$1`, a.f.A.ID); err != nil {
		t.Fatal(err)
	}
	rec := a.json("admin", "POST", "/api/v1/stages", `{"name":"Nueva","type":"open","color":"gray"}`, 500)
	if got := problemCode(t, rec); got != "internal" {
		t.Fatalf("code=%s", got)
	}
}

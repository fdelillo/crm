package contract

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

// Independent of the contracts: the exact operationId set the validator must expose.
var operationsOf001 = []string{
	"listIndustryTemplates", "signup", "login", "logout", "requestPasswordReset", "confirmPasswordReset",
	"confirmEmailVerification", "resendEmailVerification", "previewInvitation", "acceptInvitation",
	"getMe", "getTenant", "updateTenant", "getTenantLogo", "putTenantLogo", "deleteTenantLogo",
	"listUsers", "inviteUser", "changeUserRole", "deactivateUser", "reactivateUser", "liveness", "readiness",
}

var operationsOf002 = []string{
	"listStages", "createStage", "updateStage", "deleteStage", "moveStage",
	"listCategories", "createCategory", "renameCategory", "archiveCategory", "restoreCategory",
	"listCatalogItems", "createCatalogItem", "getCatalogItem", "updateCatalogItem", "archiveCatalogItem",
	"restoreCatalogItem", "getLabels", "replaceLabels", "getQuoteDefaults", "replaceQuoteDefaults",
}

// T-002-B203 (DD-13): Load reads every document of Documents; each one passes ValidateDocument.
func TestLoadReadsEveryDocument(t *testing.T) {
	if len(Documents) != 2 {
		t.Fatalf("Documents=%v, want the contracts of 001 and 002", Documents)
	}
	v, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, op := range v.Operations() {
		seen[op.Document] = true
	}
	for _, doc := range Documents {
		if !seen[doc] {
			t.Errorf("no operation comes from %s; documents seen: %v", doc, seen)
		}
	}
}

// T-002-B203 (DD-13, S-4, R-3): every $ref of the 002 contract, internal and into 001, resolves.
func TestEveryReferenceOfTheSecondContractResolves(t *testing.T) {
	v := Default(t)
	var own []Reference
	for _, ref := range v.References() {
		if ref.Document == Documents[1] {
			own = append(own, ref)
		}
	}
	if len(own) == 0 {
		t.Fatal("no references listed for the 002 contract: the walk would pass on nothing")
	}
	for _, want := range []string{
		"#/components/schemas/Money", "#/components/schemas/Problem", "#/components/securitySchemes/sessionCookie",
		"#/components/responses/Forbidden", "#/components/responses/ValidationFailed", "#/components/schemas/StageType",
	} {
		if !slices.ContainsFunc(own, func(r Reference) bool { return strings.HasSuffix(r.Ref, want) }) {
			t.Errorf("reference %s not walked", want)
		}
	}
	if !slices.ContainsFunc(own, func(r Reference) bool {
		return strings.Contains(r.Ref, "001-empresas-usuarios/contracts/openapi.yaml#")
	}) {
		t.Error("no reference into the 001 contract was walked")
	}
	if unresolved := v.UnresolvedReferences(); len(unresolved) != 0 {
		t.Errorf("unresolved references: %+v", unresolved)
	}
}

// T-002-B203 (R-3): a 001 contract without Money (as v0.5.0) makes Load fail naming the reference.
func TestLoadFailsNamingTheUnresolvedReference(t *testing.T) {
	_, err := load(".", []string{"testdata/unresolved_ref/base.yaml", "testdata/unresolved_ref/dependent.yaml"})
	if err == nil {
		t.Fatal("Load accepted a reference to a component that does not exist")
	}
	if !strings.Contains(err.Error(), "#/components/schemas/Money") {
		t.Fatalf("error does not name the unresolved reference: %v", err)
	}
}

// T-002-B203: a $ref to a file that does not exist makes Load fail naming the file.
func TestLoadFailsOnReferenceToMissingFile(t *testing.T) {
	_, err := load(".", []string{"testdata/missing_file/dependent.yaml"})
	if err == nil {
		t.Fatal("Load accepted a reference to a file that does not exist")
	}
	if !strings.Contains(err.Error(), "does-not-exist.yaml") {
		t.Fatalf("error does not name the missing file: %v", err)
	}
}

// T-002-B203 (DD-13): two contracts declaring the same method and path make Load fail naming both.
func TestLoadFailsOnDuplicateRoute(t *testing.T) {
	_, err := load(".", []string{"testdata/duplicate_route/first.yaml", "testdata/duplicate_route/second.yaml"})
	if err == nil {
		t.Fatal("Load accepted the same route in two contracts")
	}
	for _, want := range []string{"GET /things", "first.yaml", "second.yaml"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q: %v", want, err)
		}
	}
}

// T-002-B203 (DD-13): the union of operations is 001's plus the 20 of 002, by operationId.
func TestOperationsAreTheUnionOfAllContracts(t *testing.T) {
	var got []string
	for _, op := range Default(t).Operations() {
		got = append(got, op.ID)
	}
	want := append(slices.Clone(operationsOf001), operationsOf002...)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("operations differ\n got: %v\nwant: %v", got, want)
	}
}

// T-002-B203 (S-4): the enum and Money of 001 apply through the $ref of the 002 contract.
func TestExternalReferencesApplyTo002Responses(t *testing.T) {
	v := Default(t)
	stages := httptest.NewRequest(http.MethodPost, "https://crm.example/api/v1/stages", nil)
	conflict := func(code string) string {
		return `{"type":"/problems/` + code + `","title":"Conflicto","status":409,"code":"` + code + `"}`
	}
	if err := v.CheckResponse(stages, response(409, "application/problem+json", conflict("name_taken"))); err != nil {
		t.Fatalf("409 name_taken: %v", err)
	}
	err := v.CheckResponse(stages, response(409, "application/problem+json", conflict("foo")))
	if err == nil {
		t.Fatal("409 with code foo accepted: the ErrorCode enum of 001 is not applied through the $ref")
	}
	if !strings.Contains(err.Error(), "$.code") {
		t.Fatalf("error is not about the code field: %v", err)
	}

	items := httptest.NewRequest(http.MethodGet, "https://crm.example/api/v1/catalog-items", nil)
	list := func(price string) string {
		return `{"items":[{"id":"6f1c2f2e-0c1e-4c58-9a3a-2f6a7a1f9b11","name":"Pintura","description":null,"unit":"other","unit_label":"balde","reference_price":` + price + `,"archived":false,"created_at":"2026-10-23T12:00:00Z","updated_at":"2026-10-23T12:00:00Z"}]}`
	}
	if err := v.CheckResponse(items, response(200, "application/json", list(`{"amount_cents":1500,"currency":"ARS"}`))); err != nil {
		t.Fatalf("valid catalog item: %v", err)
	}
	if err := v.CheckResponse(items, response(200, "application/json", list(`null`))); err != nil {
		t.Fatalf("catalog item without price: %v", err)
	}
	err = v.CheckResponse(items, response(200, "application/json", list(`{"amount_cents":1.5,"currency":"ARS"}`)))
	if err == nil {
		t.Fatal("amount_cents 1.5 accepted: Money of 001 is not applied through the $ref")
	}
	if !strings.Contains(err.Error(), "amount_cents") {
		t.Fatalf("error is not about amount_cents: %v", err)
	}
}

// T-002-B203: requests of 002 routes are validated against the contract that declares them.
func TestRequestsAreValidatedAgainstTheDeclaringContract(t *testing.T) {
	v := Default(t)
	post := func(body string) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "https://crm.example/api/v1/stages", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "__Host-crm_session", Value: "opaque-session-value"})
		return req
	}
	if err := v.CheckRequest(post(`{"name":"Contactado","type":"open","color":"blue"}`)); err != nil {
		t.Fatalf("valid createStage request: %v", err)
	}
	err := v.CheckRequest(post(`{"name":"Contactado","type":"open","color":"blue","extra":1}`))
	if err == nil {
		t.Fatal("createStage request with an unknown property accepted")
	}
	if !strings.Contains(err.Error(), "extra") && !strings.Contains(err.Error(), "additional") {
		t.Fatalf("error is not about the unknown property: %v", err)
	}
}

// T-002-B203: a route in no contract behaves as before: global 404 and 405 against Problem.
func TestRoutesOutsideEveryContractKeepTheGlobalResponses(t *testing.T) {
	v := Default(t)
	missing := httptest.NewRequest(http.MethodGet, "https://crm.example/api/v1/no-existe", nil)
	if err := v.CheckResponse(missing, response(404, "application/problem+json", problem(404, "not_found"))); err != nil {
		t.Fatalf("global 404: %v", err)
	}
	if err := v.CheckResponse(missing, response(200, "application/json", `{}`)); err == nil {
		t.Fatal("200 on a route outside every contract accepted")
	}
	wrongMethod := httptest.NewRequest(http.MethodDelete, "https://crm.example/api/v1/stages", nil)
	resp := response(405, "application/problem+json", problem(405, "method_not_allowed"))
	resp.Header.Set("Allow", "GET, POST")
	if err := v.CheckResponse(wrongMethod, resp); err != nil {
		t.Fatalf("global 405 on a 002 path: %v", err)
	}
	if err := v.CheckResponse(wrongMethod, response(405, "application/problem+json", problem(405, "method_not_allowed"))); err == nil {
		t.Fatal("405 without Allow accepted")
	}
}

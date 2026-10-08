//go:build integration

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/httpx"
	"github.com/fdelillo/crm/internal/testsupport/apitest"
	"github.com/fdelillo/crm/internal/testsupport/contract"
	"github.com/fdelillo/crm/internal/testsupport/fixture/e2e"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func routeCoverage(t *testing.T, api chi.Router) int {
	t.Helper()
	seen := map[string]bool{}
	if err := chi.Walk(api, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		key := method + " " + route
		if seen[key] {
			t.Errorf("duplicate API route: %s", key)
		}
		seen[key] = true
		if _, ok := isolationCases[key]; !ok {
			t.Errorf("missing isolation case: %s", key)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	declared := map[string]bool{}
	for _, op := range contract.Default(t).Operations() {
		if op.Path == "/healthz" || op.Path == "/readyz" {
			continue
		}
		key := op.Method + " /api/v1" + op.Path
		declared[key] = true
		if !seen[key] {
			t.Errorf("contract route not implemented: %s", key)
		}
	}
	for key := range seen {
		if !declared[key] {
			t.Errorf("implemented route outside contract: %s", key)
		}
	}
	for key := range isolationCases {
		if !seen[key] {
			t.Errorf("isolation case without API route: %s", key)
		}
	}
	t.Logf("API routes=%d; contract API operations=%d", len(seen), len(declared))
	return len(seen)
}

func TestIsolationRouteCoverage(t *testing.T) {
	routeCoverage(t, app.BuildAPIRouter(nil, nil, clock.Real{}, slog.New(slog.NewTextHandler(io.Discard, nil))))
}

// Expectations are declared independently of both the router and the contract.
var isolationCases = map[string]string{
	"GET /api/v1/industry-templates":               "catalog",
	"POST /api/v1/auth/signup":                     "duplicate_signup",
	"POST /api/v1/auth/login":                      "login",
	"POST /api/v1/auth/logout":                     "logout",
	"POST /api/v1/auth/password-reset":             "reset_request",
	"POST /api/v1/auth/password-reset/confirm":     "reset_confirm",
	"POST /api/v1/auth/email-verification/confirm": "verification_confirm",
	"POST /api/v1/auth/email-verification/resend":  "verification_resend",
	"POST /api/v1/auth/invitations/preview":        "invitation_preview",
	"POST /api/v1/auth/invitations/accept":         "invitation_accept",
	"GET /api/v1/me":                               "read",
	"GET /api/v1/tenant":                           "read",
	"PATCH /api/v1/tenant":                         "patch",
	"GET /api/v1/tenant/logo":                      "logo_read",
	"PUT /api/v1/tenant/logo":                      "logo_put",
	"DELETE /api/v1/tenant/logo":                   "logo_delete",
	"GET /api/v1/users":                            "read",
	"POST /api/v1/users/invitations":               "invite",
	"PUT /api/v1/users/{userId}/role":              "foreign_id",
	"POST /api/v1/users/{userId}/deactivate":       "foreign_id",
	"POST /api/v1/users/{userId}/reactivate":       "foreign_id",
}

func isolationRoot(t *testing.T, f *e2e.Scenario, api chi.Router) *apitest.Server {
	t.Helper()
	root := app.NewRootHandler(app.RootDeps{API: api, Liveness: app.LivenessHandler(), Readiness: app.ReadinessPlaceholder(), SPA: app.SPAUnavailableHandler()}, app.NewCommonMiddleware(f.Logger, true, nil))
	server := apitest.NewServer(t, root)
	// Each request explicitly selects its actor; a previous response cookie must not replace it.
	server.Client.Jar = nil
	return server
}

func isolationRequest(t *testing.T, target any, ctx context.Context, cookie *http.Cookie, method, path, ct string, body []byte, etag string, want int, checks ...func(*httptest.ResponseRecorder)) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(ctx, method, path, bytes.NewReader(body))
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	rec := httptest.NewRecorder()
	switch h := target.(type) {
	case http.Handler:
		h.ServeHTTP(rec, req)
	case *apitest.Server:
		origin, err := url.Parse(h.URL)
		if err != nil {
			t.Fatal(err)
		}
		req.URL.Scheme, req.URL.Host = origin.Scheme, origin.Host
		req.RequestURI = ""
		response, err := h.Client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		for name, values := range response.Header {
			rec.Header()[name] = values
		}
		rec.WriteHeader(response.StatusCode)
		_, err = io.Copy(rec, response.Body)
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("unsupported isolation request target %T", target)
	}
	for _, check := range checks {
		check(rec)
	}
	if rec.Code != want {
		t.Fatalf("%s %s: status=%d want=%d body=%s", method, path, rec.Code, want, rec.Body.String())
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	contract.Default(t).RequireRecorded(t, req, rec)
	return rec
}

func jsonBody(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// Only T-B801 supplies a counter. Other phases retain the same assertions without
// contributing to its report; no global state or extra connection is involved.
type isolationCount struct{ detections, verifications int }

func counterFor(counters []*isolationCount) *isolationCount {
	if len(counters) == 0 {
		return nil
	}
	return counters[0]
}
func (c *isolationCount) verify(t *testing.T, isolated bool, format string, args ...any) {
	t.Helper()
	if c != nil {
		c.verifications++
		if !isolated {
			c.detections++
		}
	}
	if !isolated {
		t.Errorf(format, args...)
	}
}

func assertUnchanged(t *testing.T, f *e2e.Scenario, id uuid.UUID, before map[string]string, counters ...*isolationCount) {
	t.Helper()
	after := f.Snapshot(t, id)
	for table, want := range before {
		counterFor(counters).verify(t, after[table] == want, "company %s changed %s across isolation boundary\nbefore=%s\nafter=%s", id, table, want, after[table])
	}
}
func assertChanged(t *testing.T, f *e2e.Scenario, id uuid.UUID, before map[string]string, tables ...string) {
	t.Helper()
	after := f.Snapshot(t, id)
	for _, table := range tables {
		if after[table] == before[table] {
			t.Errorf("%s: expected observable change in %s", id, table)
		}
	}
}

// Sentinels are independent of the response schema. Only the public catalog's template
// code is exempt; all other company data is forbidden in every body and header.
func companySentinels(c e2e.Company, catalog bool) []string {
	values := []string{c.ID.String(), c.Details.Name, c.Details.Timezone, c.Details.BaseCurrency,
		c.Details.CreatedAt.Format(time.RFC3339Nano), c.Details.UpdatedAt.Format(time.RFC3339Nano),
		c.LogoKey, c.ETag, strings.Trim(c.ETag, `"`), c.Reset, c.Verification, c.Invitation, c.Suffix}
	if !catalog {
		values = append(values, c.Details.IndustryTemplateCode)
	}
	for _, v := range []*string{c.Details.LegalName, c.Details.TaxID, c.Details.Address, c.Details.Phone, c.Details.Email} {
		if v != nil {
			values = append(values, *v)
		}
	}
	for _, u := range c.Users {
		values = append(values, u.ID.String(), u.Email, u.Name)
		if u.Cookie != nil {
			values = append(values, u.Cookie.Value)
		}
	}
	unique := map[string]bool{}
	for _, value := range values {
		if value != "" {
			unique[value] = true
		}
	}
	values = values[:0]
	for value := range unique {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

type sentinelCheck struct {
	location, value string
	found           bool
}

func responseSentinelChecks(rec *httptest.ResponseRecorder, c e2e.Company, catalog bool) []sentinelCheck {
	type part struct {
		location string
		data     []byte
	}
	parts := []part{{"raw body", rec.Body.Bytes()}}
	names := make([]string, 0, len(rec.Header()))
	for name := range rec.Header() {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		parts = append(parts, part{"header name " + name, []byte(name)})
		for _, value := range rec.Header()[name] {
			parts = append(parts, part{"header value " + name, []byte(value)})
		}
	}
	var checks []sentinelCheck
	for _, value := range companySentinels(c, catalog) {
		for _, p := range parts {
			checks = append(checks, sentinelCheck{p.location, value, bytes.Contains(p.data, []byte(value))})
		}
	}
	return checks
}

func assertNoCompanyResponseData(t *testing.T, rec *httptest.ResponseRecorder, c e2e.Company, catalog bool, counters ...*isolationCount) {
	t.Helper()
	for _, check := range responseSentinelChecks(rec, c, catalog) {
		counterFor(counters).verify(t, !check.found, "cross-company data in %s: %q", check.location, check.value)
	}
}

func assertNoCompanyData(t *testing.T, body []byte, c e2e.Company) {
	t.Helper()
	rec := httptest.NewRecorder()
	rec.Body.Write(body)
	assertNoCompanyResponseData(t, rec, c, false)
}

func TestIsolationResponseSentinels(t *testing.T) {
	c := e2e.Company{Suffix: "foreign-suffix", Reset: "raw-reset", Verification: "raw-verification", Invitation: "raw-invitation",
		ETag: `"018fdcca-06d0-7b20-8c58-0842a9900239"`, Users: map[string]e2e.User{"operator": {Cookie: &http.Cookie{Value: "raw-cookie"}}}}
	c.Details.IndustryTemplateCode = "foreign-template"
	for _, tc := range []struct {
		name, header, value string
		status              int
	}{
		{"body", "", c.Reset, 200}, {"header-value", "Location", c.Verification, 202},
		{"header-name", "X-" + c.Invitation, "safe", 204}, {"cookie", "Set-Cookie", c.Users["operator"].Cookie.Value, 204},
		{"unquoted-logo", "ETag", strings.Trim(c.ETag, `"`), 304}, {"suffix", "", c.Suffix, 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			if tc.header == "" {
				rec.Body.WriteString(tc.value)
			} else {
				rec.Header()[tc.header] = []string{tc.value}
			}
			rec.WriteHeader(tc.status)
			found := 0
			for _, check := range responseSentinelChecks(rec, c, false) {
				if check.found {
					found++
				}
			}
			if found != 1 {
				t.Fatalf("detections=%d want=1", found)
			}
		})
	}
	rec := httptest.NewRecorder()
	rec.Body.WriteString(c.Details.IndustryTemplateCode)
	for _, check := range responseSentinelChecks(rec, c, true) {
		if check.found {
			t.Fatalf("catalog exception: %+v", check)
		}
	}
	rec.Header().Set("Location", c.Reset)
	found := false
	for _, check := range responseSentinelChecks(rec, c, true) {
		found = found || check.found
	}
	if !found {
		t.Fatal("catalog exception also hid a forbidden token")
	}
}

func TestIsolationHTTPMatrix(t *testing.T) {
	counts := &isolationCount{}
	covered, total := 0, 0
	t.Cleanup(func() {
		t.Logf("Isolation: covered routes=%d/%d; cross-company accesses=%d; verifications=%d", covered, total, counts.detections, counts.verifications)
		if counts.detections != 0 {
			t.Errorf("detected %d cross-company isolation violations", counts.detections)
		}
		if counts.verifications == 0 {
			t.Error("no isolation verifications executed")
		}
	})
	total = routeCoverage(t, app.BuildAPIRouter(nil, nil, clock.Real{}, slog.New(slog.NewTextHandler(io.Discard, nil))))
	keys := make([]string, 0, len(isolationCases))
	for k := range isolationCases {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			f := e2e.New(t, pgtest.AppPool(t))
			api := app.BuildAPIRouter(f.Users, f.Companies, f.Clock, f.Logger)
			h := isolationRoot(t, f, api)
			parts := strings.SplitN(key, " ", 2)
			method, path := parts[0], parts[1]
			aBefore, bBefore := f.Snapshot(t, f.A.ID), f.Snapshot(t, f.B.ID)
			foreignCompany := f.B
			switch isolationCases[key] {
			case "login", "reset_request", "reset_confirm", "verification_confirm", "invitation_preview", "invitation_accept":
				foreignCompany = f.A
			}
			checkResponse := func(rec *httptest.ResponseRecorder) {
				assertNoCompanyResponseData(t, rec, foreignCompany, isolationCases[key] == "catalog", counts)
			}
			call := func(path, ct string, body []byte, etag string, want int) *httptest.ResponseRecorder {
				return isolationRequest(t, h, context.Background(), f.A.Users["admin"].Cookie, method, path, ct, body, etag, want, func(rec *httptest.ResponseRecorder) {
					checkResponse(rec)
					if isolationCases[key] == "logo_read" && want == 200 {
						counts.verify(t, rec.Code == 200 && bytes.Equal(rec.Body.Bytes(), f.A.Logo) && rec.Header().Get("ETag") == f.A.ETag, "wrong company logo/ETag: status=%d ETag=%s", rec.Code, rec.Header().Get("ETag"))
					}
				})
			}
			switch isolationCases[key] {
			case "foreign_id":
				if !strings.Contains(path, "{userId}") {
					t.Fatalf("%s: user-id isolation case requires {userId}; other resources need their own case", key)
				}
				// RequestID is server-generated. Reuse one genuine request context for both API calls,
				// so every byte (including instance) is compared without stripping/rewriting the body.
				httpx.RequestID(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
					var payload []byte
					ct := ""
					if method == "PUT" {
						payload = []byte(`{"role":"operator"}`)
						ct = "application/json"
					}
					missing := isolationRequest(t, api, req.Context(), f.A.Users["admin"].Cookie, method, strings.ReplaceAll(path, "{userId}", uuid.NewString()), ct, payload, "", 404, checkResponse)
					for _, state := range []string{"admin", "operator", "invited", "disabled_password", "disabled_no_password"} {
						foreign := isolationRequest(t, api, req.Context(), f.A.Users["admin"].Cookie, method, strings.ReplaceAll(path, "{userId}", f.B.Users[state].ID.String()), ct, payload, "", 404, checkResponse)
						counts.verify(t, bytes.Equal(missing.Body.Bytes(), foreign.Body.Bytes()), "%s: foreign 404 differs byte for byte from absent id: %s != %s", state, foreign.Body.String(), missing.Body.String())
						counts.verify(t, reflect.DeepEqual(missing.Header(), foreign.Header()), "%s: foreign 404 headers differ from absent id: %v != %v", state, foreign.Header(), missing.Header())
						assertUnchanged(t, f, f.B.ID, bBefore, counts)
					}
				})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
				assertUnchanged(t, f, f.A.ID, aBefore, counts)
			case "read", "logo_read", "catalog":
				etag := ""
				if isolationCases[key] == "logo_read" {
					etag = f.B.ETag
				}
				rec := call(path, "", nil, etag, 200)
				switch path {
				case "/api/v1/me", "/api/v1/tenant":
					if !bytes.Contains(rec.Body.Bytes(), []byte(f.A.ID.String())) {
						t.Fatal("own company missing")
					}
				case "/api/v1/users":
					for _, u := range f.A.Users {
						if !bytes.Contains(rec.Body.Bytes(), []byte(u.ID.String())) {
							t.Errorf("own user missing: %s", u.ID)
						}
					}
				case "/api/v1/tenant/logo":
					call(path, "", nil, f.A.ETag, 304)
				}
				assertUnchanged(t, f, f.A.ID, aBefore, counts)
				assertUnchanged(t, f, f.B.ID, bBefore, counts)
			case "patch":
				call(path, "application/json", []byte(`{"name":"Updated A"}`), "", 200)
				company, err := f.Companies.Get(context.Background(), f.A.Users["admin"].Principal)
				if err != nil || company.Name != "Updated A" {
					t.Fatalf("patch not persisted: %+v %v", company, err)
				}
				assertChanged(t, f, f.A.ID, aBefore, "tenants", "audit_log")
				assertUnchanged(t, f, f.B.ID, bBefore, counts)
			case "logo_put":
				data, ct := logoMultipart(t, f.B.Logo, "file")
				call(path, ct, data, "", 200)
				var newKey string
				err := f.Runner.InTenantTx(context.Background(), f.A.ID, func(ctx context.Context, tx db.Tx) error {
					return tx.QueryRow(ctx, `SELECT logo_object_key FROM app.tenants WHERE id=$1`, f.A.ID).Scan(&newKey)
				})
				if err != nil {
					t.Fatal(err)
				}
				counts.verify(t, strings.HasPrefix(newKey, "tenants/"+f.A.ID.String()+"/"), "wrong company logo key: %s", newKey)
				if newKey == f.A.LogoKey {
					t.Fatal("logo key was not replaced")
				}
				logo, err := f.Companies.GetLogo(context.Background(), f.A.Users["admin"].Principal, "")
				if err != nil {
					t.Fatal(err)
				}
				actual, err := io.ReadAll(logo.Body)
				logo.Body.Close()
				if err != nil || !bytes.Equal(actual, f.B.Logo) {
					t.Fatal("uploaded logo not persisted")
				}
				assertChanged(t, f, f.A.ID, aBefore, "tenants", "audit_log")
				assertUnchanged(t, f, f.B.ID, bBefore, counts)
			case "logo_delete":
				call(path, "", nil, "", 204)
				company, err := f.Companies.Get(context.Background(), f.A.Users["admin"].Principal)
				if err != nil || company.HasLogo {
					t.Fatal("logo not removed")
				}
				assertChanged(t, f, f.A.ID, aBefore, "tenants", "audit_log")
				assertUnchanged(t, f, f.B.ID, bBefore, counts)
			case "invite":
				email := "new-" + uuid.NewString() + "@example.test"
				call(path, "application/json", jsonBody(map[string]string{"email": email, "role": "operator"}), "", 201)
				list, err := f.Users.ListUsers(context.Background(), f.A.Users["admin"].Principal)
				if err != nil {
					t.Fatal(err)
				}
				if len(list) != 6 {
					t.Fatalf("invitation not persisted: %d users", len(list))
				}
				assertChanged(t, f, f.A.ID, aBefore, "users", "user_tokens", "outbox_messages", "audit_log")
				assertUnchanged(t, f, f.B.ID, bBefore, counts)
			case "duplicate_signup":
				// Exactly five requests: one per user state, matching the signup limiter burst of five.
				for _, state := range []string{"admin", "operator", "invited", "disabled_password", "disabled_no_password"} {
					rec := call(path, "application/json", signupBody(f.B.Users[state].Email), "", 409)
					if !bytes.Contains(rec.Body.Bytes(), []byte(`"code":"email_already_registered"`)) {
						t.Fatal("wrong duplicate error")
					}
				}
				assertUnchanged(t, f, f.A.ID, aBefore, counts)
				assertUnchanged(t, f, f.B.ID, bBefore, counts)
			case "login":
				rec := call(path, "application/json", jsonBody(map[string]string{"email": f.B.Users["operator"].Email, "password": e2e.Password}), "", 200)
				assertSessionCompany(t, f, rec, f.B.ID, counts)
				assertUnchanged(t, f, f.A.ID, aBefore, counts)
				assertChanged(t, f, f.B.ID, bBefore, "sessions", "audit_log")
			case "logout":
				call(path, "", nil, "", 204)
				assertChanged(t, f, f.A.ID, aBefore, "sessions", "audit_log")
				assertUnchanged(t, f, f.B.ID, bBefore, counts)
			case "reset_request":
				call(path, "application/json", jsonBody(map[string]string{"email": f.B.Users["operator"].Email}), "", 202)
				assertUnchanged(t, f, f.A.ID, aBefore, counts)
				assertChanged(t, f, f.B.ID, bBefore, "user_tokens", "outbox_messages", "audit_log")
			case "reset_confirm":
				call(path, "application/json", jsonBody(map[string]string{"token": f.B.Reset, "password": e2e.NewPassword}), "", 204)
				assertUnchanged(t, f, f.A.ID, aBefore, counts)
				assertChanged(t, f, f.B.ID, bBefore, "users", "sessions", "user_tokens", "audit_log")
				// Exactly the token's user changes, including password and revoked sessions.
				bAfter := f.Snapshot(t, f.B.ID)
				assertOtherUsersUnchanged(t, bBefore["users"], bAfter["users"], f.B.Users["operator"].ID, counts)
				_, resolveErr := f.Users.ResolveSession(context.Background(), f.A.Users["operator"].Cookie.Value)
				counts.verify(t, resolveErr == nil, "A session revoked by B reset: %v", resolveErr)
				session, err := f.Users.Login(context.Background(), f.B.Users["operator"].Email, e2e.NewPassword, identity.RequestMeta{})
				counts.verify(t, err == nil && session.Principal.TenantID == f.B.ID, "reset login belongs to %s want B: %v", session.Principal.TenantID, err)
			case "verification_confirm":
				call(path, "application/json", jsonBody(map[string]string{"token": f.B.Verification}), "", 204)
				assertUnchanged(t, f, f.A.ID, aBefore, counts)
				assertChanged(t, f, f.B.ID, bBefore, "users", "user_tokens", "audit_log")
				current, err := f.Users.Me(context.Background(), f.B.Users["admin"].Principal)
				if err != nil || !current.EmailVerified {
					t.Fatalf("B not verified: %+v %v", current, err)
				}
			case "verification_resend":
				call(path, "", nil, "", 202)
				assertChanged(t, f, f.A.ID, aBefore, "user_tokens", "outbox_messages")
				assertUnchanged(t, f, f.B.ID, bBefore, counts)
			case "invitation_preview":
				rec := call(path, "application/json", jsonBody(map[string]string{"token": f.B.Invitation}), "", 200)
				if !bytes.Contains(rec.Body.Bytes(), []byte(f.B.Users["invited"].Email)) || !bytes.Contains(rec.Body.Bytes(), []byte(f.B.Details.Name)) {
					t.Fatal("preview did not resolve B")
				}
				assertUnchanged(t, f, f.A.ID, aBefore, counts)
				assertUnchanged(t, f, f.B.ID, bBefore, counts)
			case "invitation_accept":
				rec := call(path, "application/json", jsonBody(map[string]string{"token": f.B.Invitation, "name": "Accepted B", "password": e2e.NewPassword}), "", 201)
				assertSessionCompany(t, f, rec, f.B.ID, counts)
				assertUnchanged(t, f, f.A.ID, aBefore, counts)
				assertChanged(t, f, f.B.ID, bBefore, "users", "sessions", "user_tokens", "audit_log")
			default:
				t.Fatalf("unimplemented isolation case for %s", key)
			}
			if !t.Failed() {
				covered++
			}
		})
	}
	// Derive every successful operator GET from the independent permission expectations.
	operatorReads := []string{}
	for key, wants := range isolationPermissions {
		if strings.HasPrefix(key, "GET ") && wants[1] >= 200 && wants[1] < 300 {
			operatorReads = append(operatorReads, key)
		}
	}
	sort.Strings(operatorReads)
	if len(operatorReads) == 0 {
		t.Fatal("permission table has no successful operator reads")
	}
	for _, key := range operatorReads {
		t.Run("operator/"+key, func(t *testing.T) {
			f := e2e.New(t, pgtest.AppPool(t))
			api := app.BuildAPIRouter(f.Users, f.Companies, f.Clock, f.Logger)
			h := isolationRoot(t, f, api)
			path := strings.SplitN(key, " ", 2)[1]
			beforeA, beforeB := f.Snapshot(t, f.A.ID), f.Snapshot(t, f.B.ID)
			etag := ""
			if path == "/api/v1/tenant/logo" {
				etag = f.B.ETag
			}
			rec := isolationRequest(t, h, context.Background(), f.A.Users["operator"].Cookie, "GET", path, "", nil, etag, isolationPermissions[key][1], func(rec *httptest.ResponseRecorder) {
				assertNoCompanyResponseData(t, rec, f.B, isolationCases[key] == "catalog", counts)
				if path == "/api/v1/tenant/logo" {
					counts.verify(t, rec.Code == 200 && bytes.Equal(rec.Body.Bytes(), f.A.Logo) && rec.Header().Get("ETag") == f.A.ETag, "operator received wrong company logo/ETag: status=%d ETag=%s", rec.Code, rec.Header().Get("ETag"))
				}
			})
			if (path == "/api/v1/me" || path == "/api/v1/tenant") && !bytes.Contains(rec.Body.Bytes(), []byte(f.A.ID.String())) {
				t.Fatal("operator's own company missing")
			}
			assertUnchanged(t, f, f.A.ID, beforeA, counts)
			assertUnchanged(t, f, f.B.ID, beforeB, counts)
		})
	}
}

func signupBody(email string) []byte {
	return jsonBody(map[string]string{"name": "New Admin", "email": email, "password": e2e.Password, "company_name": "New Company", "base_currency": "ARS", "industry_template_code": "generic"})
}
func assertSessionCompany(t *testing.T, f *e2e.Scenario, rec *httptest.ResponseRecorder, id uuid.UUID, counters ...*isolationCount) {
	t.Helper()
	res := rec.Result()
	defer res.Body.Close()
	for _, cookie := range res.Cookies() {
		if cookie.Name != identity.SessionCookieName {
			continue
		}
		p, err := f.Users.ResolveSession(context.Background(), cookie.Value)
		counterFor(counters).verify(t, err == nil && p.TenantID == id, "session belongs to %s want %s: %v", p.TenantID, id, err)
		return
	}
	counterFor(counters).verify(t, false, "missing session cookie")
}
func assertOtherUsersUnchanged(t *testing.T, before, after string, changed uuid.UUID, counters ...*isolationCount) {
	t.Helper()
	var a, b []map[string]any
	if err := json.Unmarshal([]byte(before), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(after), &b); err != nil {
		t.Fatal(err)
	}
	counterFor(counters).verify(t, len(a) == len(b), "reset changed number of users")
	if len(a) != len(b) {
		return
	}
	for i := range a {
		if a[i]["id"] != changed.String() {
			counterFor(counters).verify(t, reflect.DeepEqual(a[i], b[i]), "reset changed unrelated user %v", a[i]["id"])
		}
	}
}

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

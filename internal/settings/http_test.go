package settings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// T-002-B405 (002/DD-19, plan 9.2): the error map answers the settings errors and sends everything else the way
// 001 does. It does not go through the router, so the contract is validated by the HTTP tests of app.
func TestWriteSettingsErrorMap(t *testing.T) {
	for _, tc := range []struct {
		name       string
		err        error
		status     int
		code       string
		rowMissing bool   // a settings_row_missing line at ERROR
		wantLog    string // other substring that the log must carry
	}{
		{"tenant settings missing", fmt.Errorf("settings: lock: %w", ErrTenantSettingsMissing), 500, "internal", true, ""},
		{"name taken", fmt.Errorf("category: %w", ErrNameTaken), 409, "name_taken", false, ""},
		{"system category", fmt.Errorf("category: %w", ErrSystemCategory), 409, "invalid_state", false, ""},
		{"malformed changes", fmt.Errorf("category: %w", ErrMalformedChanges), 400, "malformed_request", false, ""},
		{"settings not found", fmt.Errorf("stage: %w", ErrNotFound), 404, "not_found", false, ""},
		{"untranslated db not found", fmt.Errorf("stage: %w", db.ErrNotFound), 404, "not_found", false, ""},
		{"db privilege", fmt.Errorf("stage: %w", db.ErrPrivilege), 500, "internal", false, `"security_event":"rls_violation"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, nil))
			var rec *httptest.ResponseRecorder
			httpx.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				rec = httptest.NewRecorder()
				if !writeSettingsError(rec, r, tc.err, logger) {
					t.Fatal("writeSettingsError reported no error")
				}
			})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/v1/stages", nil))
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
			var p struct{ Code string }
			if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || p.Code != tc.code {
				t.Fatalf("code = %q (%v), want %q", p.Code, err, tc.code)
			}
			rowMissing := 0
			for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
				if line == "" {
					continue
				}
				var entry map[string]any
				if err := json.Unmarshal([]byte(line), &entry); err != nil {
					t.Fatalf("log line %q: %v", line, err)
				}
				if entry["event"] == "settings_row_missing" {
					rowMissing++
					if entry["level"] != "ERROR" {
						t.Fatalf("level = %v, want ERROR", entry["level"])
					}
					if id, _ := entry["request_id"].(string); id == "" {
						t.Fatalf("settings_row_missing line without request_id: %s", line)
					}
				}
			}
			if want := map[bool]int{true: 1, false: 0}[tc.rowMissing]; rowMissing != want {
				t.Fatalf("settings_row_missing lines = %d, want %d; log: %s", rowMissing, want, logs.String())
			}
			if tc.wantLog != "" && !strings.Contains(logs.String(), tc.wantLog) {
				t.Fatalf("log %q lacks %s", logs.String(), tc.wantLog)
			}
		})
	}
}

// T-002-B603 (002/DD-9, ADR-029, plan 9.1): a catalog body or query the contract does not admit is a 400 before
// the service is reached (the router below has no service: reaching it would panic), so no amount is ever read
// as a float, no null is accepted where the contract has none and no unknown field slips into the price.
func TestCatalogItemRequestsRejectMalformedInput(t *testing.T) {
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			admin := authz.Principal{TenantID: uuid.New(), UserID: uuid.New(), Role: authz.RoleAdmin}
			next.ServeHTTP(w, r.WithContext(authz.WithPrincipal(r.Context(), admin)))
		})
	})
	RegisterRoutes(router, nil, slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)))
	id := uuid.NewString()
	for _, tc := range []struct{ name, method, path, body string }{
		{"list include_archived maybe", "GET", "/api/v1/catalog-items?include_archived=maybe", ""},
		{"list include_archived empty", "GET", "/api/v1/catalog-items?include_archived=", ""},
		{"create amount with decimals", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"amount_cents":1.5,"currency":"ARS"}}`},
		{"create amount with a zero decimal", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"amount_cents":100.0,"currency":"ARS"}}`},
		{"create amount as exponent", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"amount_cents":1e3,"currency":"ARS"}}`},
		{"create amount as string", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"amount_cents":"100","currency":"ARS"}}`},
		{"create amount beyond int64", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"amount_cents":9223372036854775808,"currency":"ARS"}}`},
		{"create amount null", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"amount_cents":null,"currency":"ARS"}}`},
		{"create currency null", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"amount_cents":1,"currency":null}}`},
		{"create currency as number", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"amount_cents":1,"currency":1}}`},
		{"create price unknown field", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"amount_cents":1,"currency":"ARS","extra":1}}`},
		{"create price as number", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":5}`},
		{"create null name", "POST", "/api/v1/catalog-items", `{"name":null,"unit":"unit"}`},
		{"create null unit", "POST", "/api/v1/catalog-items", `{"name":"A","unit":null}`},
		{"create description as number", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","description":1}`},
		{"create unknown field", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","extra":1}`},
		{"update null name", "PATCH", "/api/v1/catalog-items/" + id, `{"name":null}`},
		{"update null unit", "PATCH", "/api/v1/catalog-items/" + id, `{"unit":null}`},
		{"update amount with decimals", "PATCH", "/api/v1/catalog-items/" + id, `{"reference_price":{"amount_cents":1.5,"currency":"ARS"}}`},
		{"update amount beyond int64", "PATCH", "/api/v1/catalog-items/" + id, `{"reference_price":{"amount_cents":9223372036854775808,"currency":"ARS"}}`},
		{"update price unknown field", "PATCH", "/api/v1/catalog-items/" + id, `{"reference_price":{"amount_cents":1,"currency":"ARS","extra":1}}`},
		{"update unknown field", "PATCH", "/api/v1/catalog-items/" + id, `{"archived":true}`},
		{"update broken json", "PATCH", "/api/v1/catalog-items/" + id, `{"name":`},
		{"create repeated amount_cents 100 then 900", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"amount_cents":100,"amount_cents":900,"currency":"ARS"}}`},
		{"update repeated amount_cents 100 then 900", "PATCH", "/api/v1/catalog-items/" + id, `{"reference_price":{"amount_cents":100,"amount_cents":900,"currency":"ARS"}}`},
		{"create repeated amount_cents 900 then 100", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"amount_cents":900,"amount_cents":100,"currency":"ARS"}}`},
		{"update repeated amount_cents 900 then 100", "PATCH", "/api/v1/catalog-items/" + id, `{"reference_price":{"amount_cents":900,"amount_cents":100,"currency":"ARS"}}`},
		{"create repeated currency", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"amount_cents":1,"currency":"ARS","currency":"USD"}}`},
		{"update repeated currency", "PATCH", "/api/v1/catalog-items/" + id, `{"reference_price":{"amount_cents":1,"currency":"ARS","currency":"USD"}}`},
		{"create repeated amount_cents with an escaped key", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":{"amount_cents":1,"amount_c\u0065nts":2,"currency":"ARS"}}`},
		{"update repeated amount_cents with an escaped key", "PATCH", "/api/v1/catalog-items/" + id, `{"reference_price":{"amount_cents":1,"amount_c\u0065nts":2,"currency":"ARS"}}`},
		{"create repeated name", "POST", "/api/v1/catalog-items", `{"name":"A","name":"B","unit":"unit"}`},
		{"create name repeated in another case", "POST", "/api/v1/catalog-items", `{"name":"A","Name":"B","unit":"unit"}`},
		{"update amount_cents repeated in another case", "PATCH", "/api/v1/catalog-items/" + id, `{"reference_price":{"amount_cents":1,"AMOUNT_CENTS":2,"currency":"ARS"}}`},
		{"update amount_cents repeated with a folding rune", "PATCH", "/api/v1/catalog-items/" + id, `{"reference_price":{"amount_cents":1,"amount_cent\u017f":2,"currency":"ARS"}}`},
		{"update repeated name", "PATCH", "/api/v1/catalog-items/" + id, `{"name":"A","name":"B"}`},
		{"create repeated reference_price", "POST", "/api/v1/catalog-items", `{"name":"A","unit":"unit","reference_price":null,"reference_price":{"amount_cents":1,"currency":"ARS"}}`},
		{"update repeated reference_price", "PATCH", "/api/v1/catalog-items/" + id, `{"reference_price":null,"reference_price":{"amount_cents":1,"currency":"ARS"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			var rec *httptest.ResponseRecorder
			httpx.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				rec = httptest.NewRecorder()
				router.ServeHTTP(rec, r)
			})).ServeHTTP(httptest.NewRecorder(), req)
			var problem struct{ Code string }
			if err := json.Unmarshal(rec.Body.Bytes(), &problem); rec.Code != 400 || err != nil || problem.Code != "malformed_request" {
				t.Fatalf("status = %d, body = %s; want 400 malformed_request", rec.Code, rec.Body.String())
			}
		})
	}
}

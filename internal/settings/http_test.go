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

	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/httpx"
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

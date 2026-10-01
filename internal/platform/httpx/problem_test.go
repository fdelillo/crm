package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/platform/db"
)

func TestProblemCodes(t *testing.T) {
	cases := []struct {
		code   Code
		status int
	}{
		{CodeMalformedRequest, 400}, {CodeValidationFailed, 422}, {CodeUnsupportedMediaType, 415},
		{CodePayloadTooLarge, 413}, {CodeUnauthenticated, 401}, {CodeInvalidCredentials, 401},
		{CodeAccountDisabled, 403}, {CodeForbidden, 403}, {CodeNotFound, 404},
		{CodeMethodNotAllowed, 405}, {CodeEmailAlreadyRegistered, 409}, {CodeEmailTaken, 409},
		{CodeLastAdmin, 409}, {CodeInvalidState, 409}, {CodeTokenInvalid, 400},
		{CodeLoginLocked, 429}, {CodeRateLimited, 429}, {CodeServiceUnavailable, 503}, {CodeInternal, 500},
	}
	for _, tc := range cases {
		t.Run(string(tc.code), func(t *testing.T) {
			w := httptest.NewRecorder()
			// RequestID (not a bare httptest.NewRequest) so instance can be checked against the
			// same id the response header carries (T-B201).
			RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				WriteProblem(w, r, tc.code)
			})).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/test", nil))
			if w.Code != tc.status || w.Header().Get("Content-Type") != "application/problem+json" {
				t.Fatalf("status=%d content-type=%q", w.Code, w.Header().Get("Content-Type"))
			}
			var raw map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
				t.Fatal(err)
			}
			var p struct {
				Type, Title, Instance string
				Status                int
				Code                  Code
			}
			if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
				t.Fatal(err)
			}
			if p.Type != "/problems/"+string(tc.code) || p.Status != tc.status || p.Code != tc.code || p.Title == "" {
				t.Fatalf("problem=%+v", p)
			}
			if p.Instance == "" || p.Instance != w.Header().Get("X-Request-Id") {
				t.Fatalf("instance=%q header=%q", p.Instance, w.Header().Get("X-Request-Id"))
			}
			if _, present := raw["suggested_action"]; present {
				t.Fatalf("suggested_action present without the option: %v", raw)
			}
		})
	}
}

func TestProblemOptions(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	WriteProblem(w, r, CodeEmailAlreadyRegistered, WithSuggestedAction(SuggestedPasswordReset))
	var p map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p["title"] != "Ya existe un usuario con ese email" || p["detail"] != "Ya existe un usuario con ese email. ¿Querés recuperar la contraseña?" || p["suggested_action"] != "password_reset" {
		t.Fatalf("problem=%v", p)
	}
	w = httptest.NewRecorder()
	WriteProblem(w, r, CodeServiceUnavailable, RetryAfter(2*time.Second))
	if got := w.Header().Get("Retry-After"); got != "2" {
		t.Fatalf("Retry-After=%q", got)
	}
	w = httptest.NewRecorder()
	WriteProblem(w, r, CodeServiceUnavailable)
	if got := w.Header().Get("Retry-After"); got != "" {
		t.Fatalf("unexpected Retry-After=%q", got)
	}
}

func TestValidationErrorSorted(t *testing.T) {
	w := httptest.NewRecorder()
	ValidationError(w, httptest.NewRequest(http.MethodPost, "/", nil), map[string]string{"z": "required", "a": "invalid_value"})
	var p struct {
		Code   Code
		Errors []struct{ Field, Code string }
	}
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if w.Code != 422 || p.Code != CodeValidationFailed || len(p.Errors) != 2 || p.Errors[0].Field != "a" || p.Errors[1].Field != "z" {
		t.Fatalf("response=%d %+v", w.Code, p)
	}
}

func TestWriteDBError(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{"not found", db.ErrNotFound, 404},
		{"unavailable", db.ErrUnavailable, 503},
		{"privilege", db.ErrPrivilege, 500},
		{"unclassified", errors.New("boom"), 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, nil))
			w := httptest.NewRecorder()
			WriteDBError(w, httptest.NewRequest(http.MethodGet, "/api/v1/test", nil), tc.err, logger)
			if w.Code != tc.status {
				t.Fatalf("status=%d want=%d", w.Code, tc.status)
			}
			if tc.name == "privilege" {
				if !strings.Contains(logs.String(), `"security_event":"rls_violation"`) {
					t.Fatalf("missing security event: %s", logs.String())
				}
				if !strings.Contains(logs.String(), `"err":"db: insufficient privilege or RLS violation"`) {
					t.Fatalf("missing err in log: %s", logs.String())
				}
			} else if logs.Len() != 0 {
				t.Fatalf("unexpected log for %s: %s", tc.name, logs.String())
			}
		})
	}
}

// A nil logger must panic instead of silently falling back to slog.Default() (N2 of the second PR
// #8 review, replacing I3's fallback): a text-handler default or one nobody reads would still lose
// INV-19's ErrPrivilege log, just less visibly. logger is a caller bug when missing, not a runtime
// condition to degrade from.
func TestWriteDBErrorPanicsOnNilLogger(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic with a nil logger")
		}
	}()
	w := httptest.NewRecorder()
	WriteDBError(w, httptest.NewRequest(http.MethodGet, "/api/v1/test", nil), db.ErrPrivilege, nil)
}

func TestWriteDBErrorCanceledByClientWritesNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	WriteDBError(w, r, db.ErrCanceled, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if w.Body.Len() != 0 {
		t.Fatalf("canceled response has a body: %q", w.Body.String())
	}
}

func TestDecodeJSON(t *testing.T) {
	for _, tc := range []struct {
		name, body, contentType string
		status                  int
	}{
		{"valid", `{"name":"Ana"}`, "application/json", 0},
		{"charset", `{"name":"Ana"}`, "application/json; charset=utf-8", 0},
		{"unknown", `{"other":1}`, "application/json", 400},
		{"concatenated", `{"name":"Ana"}{"name":"B"}`, "application/json", 400},
		{"oversize", `{"name":"` + strings.Repeat("a", 65536) + `"}`, "application/json", 413},
		{"missing type", `{"name":"Ana"}`, "", 415},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
			if tc.contentType != "" {
				r.Header.Set("Content-Type", tc.contentType)
			}
			w := httptest.NewRecorder()
			var dst struct {
				Name string `json:"name"`
			}
			err := DecodeJSON(w, r, &dst)
			if tc.status == 0 && (err != nil || dst.Name != "Ana") {
				t.Fatalf("err=%v dst=%+v", err, dst)
			}
			if tc.status != 0 && (err == nil || w.Code != tc.status) {
				t.Fatalf("err=%v status=%d", err, w.Code)
			}
		})
	}
}

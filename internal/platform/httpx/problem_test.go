package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
			WriteProblem(w, httptest.NewRequest(http.MethodGet, "/api/v1/test", nil), tc.code)
			if w.Code != tc.status || w.Header().Get("Content-Type") != "application/problem+json" {
				t.Fatalf("status=%d content-type=%q", w.Code, w.Header().Get("Content-Type"))
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

package httpx

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// requestLine returns the single "request" record Logging wrote.
func requestLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var found []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("log line is not JSON: %q: %v", l, err)
		}
		if m["msg"] == "request" {
			found = append(found, m)
		}
	}
	if len(found) != 1 {
		t.Fatalf("request lines = %d, want 1; logs:\n%s", len(found), buf.String())
	}
	return found[0]
}

// T-F011 (DD-50): WithStatusLogLevel only changes the level of the final status it names, and
// Logging keeps the precedence canceled > panic > SetLogLevel > WithStatusLogLevel > default.
func TestWithStatusLogLevel(t *testing.T) {
	respond := func(code int) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) })
	}
	cases := []struct {
		name       string
		h          http.Handler
		wantStatus int
		wantLevel  string
	}{
		{"503 is WARN", respond(503), 503, "WARN"},
		{"500 stays ERROR", respond(500), 500, "ERROR"},
		{"200 stays INFO", respond(200), 200, "INFO"},
		{"404 stays INFO", respond(404), 404, "INFO"},
		{"implicit 200 stays INFO", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }), 200, "INFO"},
		{"panic is 500 ERROR", http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }), 500, "ERROR"},
		{"SetLogLevel prevails over the status rule", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			SetLogLevel(r, slog.LevelInfo)
			w.WriteHeader(503)
		}), 503, "INFO"},
		{"client canceled is INFO 499", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			markClientCanceled(r)
			w.WriteHeader(503)
		}), 499, "INFO"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))
			h := Recover(logger)(Logging(logger)(WithStatusLogLevel(503, slog.LevelWarn, tc.h)))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
			if rec.Code != tc.wantStatus && tc.wantStatus != 499 {
				t.Errorf("response status = %d, want %d", rec.Code, tc.wantStatus)
			}
			line := requestLine(t, &buf)
			if line["level"] != tc.wantLevel || line["status"] != float64(tc.wantStatus) {
				t.Errorf("level = %v status = %v, want %s %d", line["level"], line["status"], tc.wantLevel, tc.wantStatus)
			}
		})
	}
}

// T-F011 (DD-50): outside Logging the rule is a no-op: same response, no panic.
func TestWithStatusLogLevelWithoutLogging(t *testing.T) {
	h := WithStatusLogLevel(503, slog.LevelWarn, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(503)
		_, _ = w.Write([]byte("down"))
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != 503 || rec.Body.String() != "down" {
		t.Errorf("status %d body %q, want 503 down", rec.Code, rec.Body.String())
	}
}

// T-F011 (DD-50): a panic is answered 500 by Recover and still logs ERROR even if a rule names 500.
func TestWithStatusLogLevelDoesNotDowngradePanic(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	h := Recover(logger)(Logging(logger)(WithStatusLogLevel(500, slog.LevelWarn,
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	line := requestLine(t, &buf)
	if rec.Code != 500 || line["level"] != "ERROR" || line["status"] != float64(500) {
		t.Errorf("status %d level %v, want 500 ERROR", rec.Code, line["level"])
	}
}

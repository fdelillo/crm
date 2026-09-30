package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"expvar"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/httpx"
)

func phase2Root(t *testing.T, local bool, trusted []netip.Prefix, logs *bytes.Buffer, called *int) http.Handler {
	t.Helper()
	api := app.NewAPIRouter()
	api.Post("/api/v1/test", func(w http.ResponseWriter, r *http.Request) { *called++; w.WriteHeader(201) })
	api.Get("/api/v1/test", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("{}")) })
	api.Get("/api/v1/tenant/logo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-cache")
		w.WriteHeader(200)
	})
	api.Get("/api/v1/error", func(w http.ResponseWriter, r *http.Request) { httpx.WriteDBError(w, r, db.ErrCanceled, nil) })
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	return app.NewRootHandler(app.RootDeps{
		API:       api,
		Liveness:  http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }),
		Readiness: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }),
		SPA:       http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("spa")) }),
	}, app.NewCommonMiddleware(logger, local, trusted))
}

func TestPhase2CommonMiddleware(t *testing.T) {
	var logs bytes.Buffer
	called := 0
	trusted := []netip.Prefix{netip.MustParsePrefix("198.51.100.0/24")}
	root := phase2Root(t, false, trusted, &logs, &called)
	cases := []struct {
		name, method, path, fetch, origin string
		status                            int
		noStore                           bool
	}{
		{"cross-site", "POST", "/api/v1/test", "cross-site", "", 403, true},
		{"other origin", "POST", "/api/v1/test", "", "https://other.example", 403, true},
		{"same origin", "POST", "/api/v1/test", "same-origin", "", 201, true},
		{"safe cross-site", "GET", "/api/v1/test", "cross-site", "", 200, true},
		{"not found", "GET", "/api/v1/missing", "", "", 404, true},
		{"method not allowed", "PUT", "/api/v1/test", "", "", 405, true},
		{"spa", "GET", "/", "", "", 200, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.path, nil)
			r.RemoteAddr = "198.51.100.10:4711"
			r.Header.Set("X-Forwarded-For", "192.0.2.66")
			if tc.fetch != "" {
				r.Header.Set("Sec-Fetch-Site", tc.fetch)
			}
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			w := httptest.NewRecorder()
			root.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.status, w.Body.String())
			}
			if (w.Header().Get("Cache-Control") == "no-store") != tc.noStore {
				t.Fatalf("cache=%q", w.Header().Get("Cache-Control"))
			}
			for _, key := range []string{"Strict-Transport-Security", "X-Content-Type-Options", "Referrer-Policy"} {
				if w.Header().Get(key) == "" {
					t.Errorf("%s missing", key)
				}
			}
			if tc.status == 403 {
				var p struct{ Code string }
				if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil || p.Code != "forbidden" || w.Header().Get("Content-Type") != "application/problem+json" {
					t.Fatalf("csrf response=%q code=%q err=%v", w.Body.String(), p.Code, err)
				}
			}
		})
	}
	if called != 1 {
		t.Fatalf("handler called %d times", called)
	}
	if !strings.Contains(logs.String(), `"security_event":"csrf_rejected"`) || !strings.Contains(logs.String(), `"ip":"192.0.2.66"`) {
		t.Fatalf("missing security event or client ip: %s", logs.String())
	}
	if v := expvar.Get("csrf_rejected_total"); v == nil || v.String() == "0" {
		t.Fatal("CSRF metric missing")
	}
	w := httptest.NewRecorder()
	root.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/tenant/logo", nil))
	if w.Header().Get("Cache-Control") != "private, no-cache" {
		t.Fatalf("logo cache=%q", w.Header().Get("Cache-Control"))
	}
	local := phase2Root(t, true, nil, &bytes.Buffer{}, new(int))
	w = httptest.NewRecorder()
	local.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Header().Get("Strict-Transport-Security") != "" {
		t.Fatal("HSTS on local")
	}
}

func TestCanceledRequestLog(t *testing.T) {
	var logs bytes.Buffer
	root := phase2Root(t, true, nil, &logs, new(int))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequest("GET", "/api/v1/error", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	root.ServeHTTP(w, r)
	if w.Body.Len() != 0 {
		t.Fatalf("canceled response has body %q", w.Body.String())
	}
	if !strings.Contains(logs.String(), `"status":499`) || !strings.Contains(logs.String(), `"event":"client_canceled"`) || strings.Contains(logs.String(), `"level":"ERROR"`) {
		t.Fatalf("canceled log=%s", logs.String())
	}
	if v := expvar.Get("http_client_canceled_total"); v == nil || v.String() == "0" {
		t.Fatal("canceled metric missing")
	}
	live := httptest.NewRecorder()
	root.ServeHTTP(live, httptest.NewRequest("GET", "/api/v1/error", nil))
	if live.Code != 503 {
		t.Fatalf("live cancellation status=%d", live.Code)
	}
}

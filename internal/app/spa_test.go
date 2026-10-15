package app_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/testsupport/contract"
	"github.com/fdelillo/crm/web"
	"github.com/go-chi/chi/v5"
)

// T-F007 (INV-22, DD-24, DD-29): the real SPA shares common middleware without
// changing the API's responses, compression policy or route inventory.
func TestRootEmbeddedSPA(t *testing.T) {
	for _, local := range []bool{false, true} {
		name := "https://crm.example"
		if local {
			name = "https://localhost:8443"
		}
		t.Run(name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			api := app.BuildAPIRouter(nil, nil, clock.Real{}, logger)
			var before []string
			walk := func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
				before = append(before, method+" "+route)
				if !strings.HasPrefix(route, "/api/") {
					t.Errorf("API includes SPA or ops route %s %s", method, route)
				}
				return nil
			}
			if err := chi.Walk(api, walk); err != nil {
				t.Fatal(err)
			}
			original := append([]string(nil), before...)
			root := app.NewRootHandler(app.RootDeps{
				API: api, Liveness: app.LivenessHandler(), Readiness: app.ReadinessPlaceholder(),
				SPA: web.NewHandler(fstest.MapFS{
					"index.html":             {Data: []byte("<!doctype html><html>CRM</html>")},
					"assets/index-abc123.js": {Data: []byte(strings.Repeat("export default 'crm';\n", 100))},
				}),
			}, app.NewCommonMiddleware(logger, local, nil))
			before = nil
			if err := chi.Walk(api, walk); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(original, before) || len(before) == 0 {
				t.Fatalf("API route inventory changed: before %v, after %v", original, before)
			}
			for _, tt := range []struct {
				method, path string
				status       int
			}{
				{http.MethodGet, "/", 200},
				{http.MethodGet, "/settings/users", 200},
				{http.MethodGet, "/assets/index-abc123.js", 200},
				{http.MethodGet, "/assets/viejo-def456.js", 404},
				{http.MethodPost, "/login", 405},
				{http.MethodGet, "/api/v1/no-existe", 404},
				{http.MethodGet, "/api/v1/me", 401},
				{http.MethodGet, "/healthz", 200},
			} {
				t.Run(tt.method+" "+tt.path, func(t *testing.T) {
					req := httptest.NewRequest(tt.method, tt.path, nil)
					req.Header.Set("Accept-Encoding", "gzip")
					rec := httptest.NewRecorder()
					root.ServeHTTP(rec, req)
					if rec.Code != tt.status {
						t.Fatalf("status = %d, want %d; body %q", rec.Code, tt.status, rec.Body.String())
					}
					if rec.Header().Get("X-Request-Id") == "" || rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("Referrer-Policy") != "no-referrer" {
						t.Errorf("missing common headers: %v", rec.Header())
					}
					if got := rec.Header().Get("Strict-Transport-Security"); local && got != "" || !local && got == "" {
						t.Errorf("HSTS = %q for local=%t", got, local)
					}
					if strings.HasPrefix(tt.path, "/api/") {
						contract.Default(t).RequireRecorded(t, req, rec)
						if rec.Header().Get("Content-Encoding") != "" || rec.Header().Get("Content-Type") != "application/problem+json" || strings.Contains(rec.Body.String(), "<html") {
							t.Errorf("API response was changed by SPA: headers %v body %q", rec.Header(), rec.Body.String())
						}
					}
					if tt.path == "/healthz" && strings.TrimSpace(rec.Body.String()) != `{"status":"ok"}` {
						t.Errorf("liveness body = %q", rec.Body.String())
					}
				})
			}
		})
	}
}

// T-F007 (DD-29): even API bodies above the compression threshold stay uncompressed.
func TestRootDoesNotCompressLargeAPIResponse(t *testing.T) {
	api := app.NewAPIRouter()
	body := `{"name":"` + strings.Repeat("CRM ", 500) + `"}`
	api.Get("/api/v1/example", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	})
	root := app.NewRootHandler(app.RootDeps{
		API: api, Liveness: app.LivenessHandler(), Readiness: app.ReadinessPlaceholder(),
		SPA: web.NewHandler(fstest.MapFS{"index.html": {Data: []byte("<html>CRM</html>")}}),
	}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/example", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	root.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != body || rec.Header().Get("Content-Encoding") != "" {
		t.Fatalf("API was compressed: status %d headers %v body %q", rec.Code, rec.Header(), rec.Body.String())
	}
}

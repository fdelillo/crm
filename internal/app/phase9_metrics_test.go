package app_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/config"
	"github.com/go-chi/chi/v5"
)

func TestMetricsServer(t *testing.T) {
	cfg := config.Config{HTTPAddr: "127.0.0.1:8080", MetricsAddr: "127.0.0.1:9091"}
	srv := app.NewMetricsServer(cfg)
	if srv.Addr != cfg.MetricsAddr || srv.Handler == nil || srv.Handler == http.DefaultServeMux || srv.TLSConfig != nil {
		t.Fatalf("metrics server: %#v", srv)
	}
	if srv.ReadHeaderTimeout != 5*time.Second || srv.ReadTimeout != 15*time.Second || srv.WriteTimeout != 30*time.Second || srv.IdleTimeout <= 0 {
		t.Fatal("metrics server timeouts")
	}
	request := func(method, path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		srv.Handler.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
		return rec
	}
	rec := request("GET", "/debug/vars")
	var vars map[string]json.RawMessage
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &vars) != nil || vars["cmdline"] == nil || vars["memstats"] == nil {
		t.Fatalf("debug vars: %d %s", rec.Code, rec.Body.String())
	}
	if got := request("POST", "/debug/vars").Code; got != 405 {
		t.Fatalf("POST vars=%d", got)
	}
	api := app.BuildAPIRouter(nil, nil, clock.Real{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	parameter := regexp.MustCompile(`\{[^}]+\}`)
	if err := chi.Walk(api, func(method, path string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		rec := request(method, parameter.ReplaceAllString(path, "018fdcca-06d0-7b20-8c58-0842a9900239"))
		if rec.Code != 404 || strings.HasPrefix(rec.Header().Get("Content-Type"), "application/problem+json") {
			t.Errorf("metrics exposed %s %s: %d %s", method, path, rec.Code, rec.Body.String())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/healthz", "/readyz", "/"} {
		if rec := request("GET", path); rec.Code != 404 {
			t.Errorf("metrics exposed %s: %d", path, rec.Code)
		}
	}
	old := http.DefaultServeMux
	http.DefaultServeMux = http.NewServeMux()
	t.Cleanup(func() { http.DefaultServeMux = old })
	http.DefaultServeMux.HandleFunc("/global-probe", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	if got := request("GET", "/global-probe").Code; got != 404 {
		t.Fatalf("global mux probe=%d", got)
	}
	spaCalled := false
	root := app.NewRootHandler(app.RootDeps{API: api, Liveness: app.LivenessHandler(), Readiness: app.ReadinessPlaceholder(), SPA: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { spaCalled = true; _, _ = io.WriteString(w, "spa") })}, nil)
	rec = httptest.NewRecorder()
	root.ServeHTTP(rec, httptest.NewRequest("GET", "/debug/vars", nil))
	if !spaCalled || strings.Contains(rec.Body.String(), `"memstats"`) || strings.Contains(rec.Body.String(), `"cmdline"`) {
		t.Fatalf("root exposed vars: %s", rec.Body.String())
	}
}

func TestNewServerRejectsNilHandler(t *testing.T) {
	if srv, err := app.NewServer(config.Config{}, nil); err == nil || srv != nil {
		t.Fatalf("nil handler accepted: %v %v", srv, err)
	}
}

package app_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/web"
)

// requestLineFor returns the "request" record of the response with that X-Request-Id.
func requestLineFor(t *testing.T, logs *syncBuffer, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	id := rec.Header().Get("X-Request-Id")
	for _, l := range logs.lines(t) {
		if l["msg"] == "request" && l["request_id"] == id {
			return l
		}
	}
	t.Fatalf("no request line for %q; logs:\n%s", id, logs.String())
	return nil
}

func statusHandler(code int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) })
}

// T-F011 (DD-50, INV-22): the WARN rule for 503 applies to the SPA branch of the root mux only.
func TestRootSPALogLevel(t *testing.T) {
	unbuilt := func() http.Handler { return web.NewHandler(fstest.MapFS{}) }
	built := func() http.Handler {
		return web.NewHandler(fstest.MapFS{"index.html": {Data: []byte("<html>CRM</html>")}})
	}
	// A plain handler, not chi: the escaped path would be a chi 404, and what matters is that the
	// request reaches the API and not the SPA rule.
	apiWith := statusHandler
	panicking := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("spa boom") })

	cases := []struct {
		name       string
		spa        http.Handler
		api        http.Handler
		method     string
		target     string
		gzip       bool
		wantStatus int
		wantLevel  string
		wantRoute  string
	}{
		{"unbuilt SPA GET /login", unbuilt(), nil, "GET", "/login", false, 503, "WARN", "spa"},
		{"unbuilt SPA with gzip", unbuilt(), nil, "GET", "/login", true, 503, "WARN", "spa"},
		{"unbuilt SPA HEAD /", unbuilt(), nil, "HEAD", "/", false, 503, "WARN", "spa"},
		{"unbuilt SPA POST /login", unbuilt(), nil, "POST", "/login", false, 405, "INFO", "spa"},
		{"SPA answering 500", statusHandler(500), nil, "GET", "/x", false, 500, "ERROR", "spa"},
		{"SPA panic", panicking, nil, "GET", "/x", false, 500, "ERROR", "spa"},
		{"built SPA GET /", built(), nil, "GET", "/", false, 200, "INFO", "spa"},
		{"SPAUnavailableHandler", app.SPAUnavailableHandler(), nil, "GET", "/login", false, 503, "WARN", "spa"},
		{"API 503 on /api/v1/x", unbuilt(), apiWith(503), "GET", "/api/v1/x", false, 503, "ERROR", ""},
		{"API 503 on escaped /api%2Fv1%2Fx", unbuilt(), apiWith(503), "GET", "/api%2Fv1%2Fx", false, 503, "ERROR", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logs := &syncBuffer{}
			logger := slog.New(slog.NewJSONHandler(logs, nil))
			api := tc.api
			if api == nil {
				api = app.NewAPIRouter()
			}
			root := app.NewRootHandler(app.RootDeps{
				API: api, Liveness: app.LivenessHandler(), Readiness: app.ReadinessPlaceholder(), SPA: tc.spa,
			}, app.NewCommonMiddleware(logger, true, nil))
			req := httptest.NewRequest(tc.method, tc.target, nil)
			if tc.gzip {
				req.Header.Set("Accept-Encoding", "gzip")
			}
			rec := httptest.NewRecorder()
			root.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.method == "HEAD" && rec.Body.Len() != 0 {
				t.Errorf("HEAD body = %q, want empty", rec.Body.String())
			}
			line := requestLineFor(t, logs, rec)
			if line["level"] != tc.wantLevel || line["status"] != float64(tc.wantStatus) {
				t.Errorf("level = %v status = %v, want %s %d", line["level"], line["status"], tc.wantLevel, tc.wantStatus)
			}
			if tc.wantRoute != "" && line["route"] != tc.wantRoute {
				t.Errorf("route = %v, want %q", line["route"], tc.wantRoute)
			}
		})
	}
}

// T-F011 (DD-50): the level rule does not change the 503 of the unbuilt SPA (T-F007): headers and
// body are the same as web.NewHandler's own.
func TestRootUnbuiltSPAResponseUnchangedByLogLevel(t *testing.T) {
	spa := web.NewHandler(fstest.MapFS{})
	direct := httptest.NewRecorder()
	spa.ServeHTTP(direct, httptest.NewRequest(http.MethodGet, "/login", nil))

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	root := app.NewRootHandler(app.RootDeps{
		API: app.NewAPIRouter(), Liveness: app.LivenessHandler(), Readiness: app.ReadinessPlaceholder(), SPA: spa,
	}, app.NewCommonMiddleware(logger, true, nil))
	rec := httptest.NewRecorder()
	root.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/login", nil))

	if rec.Body.String() != direct.Body.String() || rec.Body.Len() == 0 {
		t.Errorf("body = %q, want %q", rec.Body.String(), direct.Body.String())
	}
	for k, want := range direct.Header() {
		if got := rec.Header()[k]; len(got) != len(want) || got[0] != want[0] {
			t.Errorf("header %s = %v, want %v", k, got, want)
		}
	}
}

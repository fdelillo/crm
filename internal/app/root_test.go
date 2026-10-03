package app_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/testsupport/contract"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// syncBuffer is a bytes.Buffer safe for the concurrent writes of the server goroutines.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// lines returns the JSON log records written so far.
func (b *syncBuffer) lines(t testing.TB) []map[string]any {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(b.buf.String()), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("log line is not JSON: %q: %v", l, err)
		}
		out = append(out, m)
	}
	return out
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// spaStub is the stand-in for RootDeps.SPA: it answers 200 text/plain "spa" and records the
// paths it receives.
type spaStub struct {
	mu    sync.Mutex
	paths []string
	panic bool
}

func (s *spaStub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.paths = append(s.paths, r.URL.Path)
	s.mu.Unlock()
	if s.panic {
		panic("spa boom")
	}
	w.Header().Set("Content-Type", "text/plain")
	_, _ = io.WriteString(w, "spa")
}

func (s *spaStub) received() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.paths...)
}

type harness struct {
	handler http.Handler
	spa     *spaStub
	logs    *syncBuffer
}

// testAPI is a chi router with the routes the tests need.
func testAPI() *chi.Mux {
	api := app.NewAPIRouter()
	api.Get("/api/v1/ping", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"pong":true}`)
	})
	api.Get("/api/v1/industry-templates", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"items":[]}`)
	})
	api.Get("/api/v1/things/{id}", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "thing")
	})
	api.Get("/api/v1/boom", func(http.ResponseWriter, *http.Request) { panic("api boom") })
	return api
}

// newHarness builds the real root handler with a test chi router and the SPA stub.
func newHarness(t *testing.T, local bool) *harness {
	t.Helper()
	return newHarnessWithAPI(t, local, testAPI())
}

func newHarnessWithAPI(t *testing.T, local bool, api http.Handler) *harness {
	t.Helper()
	logs := &syncBuffer{}
	logger := slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	spa := &spaStub{}
	root := app.NewRootHandler(app.RootDeps{
		API:       api,
		Liveness:  app.LivenessHandler(),
		Readiness: app.ReadinessPlaceholder(),
		SPA:       spa,
	}, app.NewCommonMiddleware(logger, local, nil))
	return &harness{handler: root, spa: spa, logs: logs}
}

func (h *harness) do(method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

type problem struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Instance string `json:"instance"`
	Code     string `json:"code"`
}

func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder) problem {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json (body %q)", ct, rec.Body.String())
	}
	var p problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("problem body is not JSON: %v", err)
	}
	return p
}

func TestRoot_Healthz(t *testing.T) {
	h := newHarness(t, false)
	rec := h.do(http.MethodGet, "/healthz")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"status":"ok"}` {
		t.Errorf("body = %q, want {\"status\":\"ok\"}", got)
	}
	if got := h.spa.received(); len(got) != 0 {
		t.Errorf("SPA received %v, want nothing", got)
	}
}

// A non-GET request to an ops path is a 405 and never falls through to the SPA's catch-all.
func TestRoot_OpsRejectOtherMethods(t *testing.T) {
	for _, path := range []string{"/healthz", "/readyz"} {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
			t.Run(method+" "+path, func(t *testing.T) {
				h := newHarness(t, false)
				rec := h.do(method, path)
				if rec.Code != http.StatusMethodNotAllowed {
					t.Errorf("status = %d, want 405", rec.Code)
				}
				if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
					t.Errorf("Allow = %q, want it to list GET", allow)
				}
				if got := h.spa.received(); len(got) != 0 {
					t.Errorf("SPA received %v, want nothing", got)
				}
			})
		}
	}
}

func TestRoot_HealthzAcceptsHEAD(t *testing.T) {
	h := newHarness(t, false)
	if rec := h.do(http.MethodHead, "/healthz"); rec.Code != http.StatusOK {
		t.Errorf("HEAD /healthz status = %d, want 200", rec.Code)
	}
}

// Readiness is a placeholder until T-B903: it must at least never fall through to the SPA.
func TestRoot_ReadyzNeverReachesSPA(t *testing.T) {
	h := newHarness(t, false)
	rec := h.do(http.MethodGet, "/readyz")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("GET /readyz status = %d, want 503 (placeholder)", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	if got := h.spa.received(); len(got) != 0 {
		t.Errorf("SPA received %v, want nothing", got)
	}
}

// Everything under /api/ belongs to chi: unknown paths are problem+json 404, never the SPA (INV-22).
func TestRoot_APINotFound(t *testing.T) {
	for _, target := range []string{
		"/api/v1/no-existe", "/api/v2/cualquier", "/api/", "/api/v1/things",
		// The ServeMux routes on the escaped path, so these would reach the SPA's catch-all with an
		// unescaped URL.Path under /api/ (INV-22).
		"/api%2Fv1%2Fno-existe", "/api%2f", "/%61pi/v1/no-existe",
	} {
		t.Run(target, func(t *testing.T) {
			h := newHarness(t, false)
			req := httptest.NewRequest(http.MethodGet, target, nil)
			rec := httptest.NewRecorder()
			h.handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", rec.Code)
			}
			contract.Default(t).RequireRecorded(t, req, rec)
			p := decodeProblem(t, rec)
			if p.Code != "not_found" || p.Status != 404 || p.Type != "/problems/not_found" || p.Title == "" {
				t.Errorf("problem = %+v", p)
			}
			if id := rec.Header().Get("X-Request-Id"); id == "" || p.Instance != id {
				t.Errorf("instance = %q, X-Request-Id = %q; want equal and non-empty", p.Instance, id)
			}
			if got := h.spa.received(); len(got) != 0 {
				t.Errorf("SPA received %v, want nothing (INV-22)", got)
			}
		})
	}
}

func TestRoot_APIMethodNotAllowed(t *testing.T) {
	h := newHarness(t, false)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/industry-templates", nil)
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	contract.Default(t).RequireRecorded(t, req, rec)
	p := decodeProblem(t, rec)
	if p.Status != 405 || p.Code != "method_not_allowed" || p.Type != "/problems/method_not_allowed" || p.Instance == "" {
		t.Errorf("problem = %+v, want status 405 and code method_not_allowed", p)
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
		t.Errorf("Allow = %q, want it to list GET", allow)
	}
	if got := h.spa.received(); len(got) != 0 {
		t.Errorf("SPA received %v, want nothing", got)
	}
}

// A method chi does not know still gets the same 405 with Allow.
func TestRoot_APIUnknownMethodIsMethodNotAllowed(t *testing.T) {
	h := newHarness(t, false)
	req := httptest.NewRequest("FOO", "/api/v1/industry-templates", nil)
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	contract.Default(t).RequireRecorded(t, req, rec)
	if p := decodeProblem(t, rec); p.Code != "method_not_allowed" {
		t.Errorf("code = %q, want method_not_allowed", p.Code)
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
		t.Errorf("Allow = %q, want it to list GET", allow)
	}
}

// "/api" without the trailing slash is redirected by the ServeMux and never reaches the SPA.
func TestRoot_APIWithoutSlashRedirects(t *testing.T) {
	h := newHarness(t, false)
	rec := h.do(http.MethodGet, "/api")
	switch rec.Code {
	case http.StatusMovedPermanently, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
	default:
		t.Fatalf("status = %d, want a redirect", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/api/" {
		t.Errorf("Location = %q, want /api/", loc)
	}
	if got := h.spa.received(); len(got) != 0 {
		t.Errorf("SPA received %v, want nothing", got)
	}
}

func TestRoot_SPAReceivesEverythingElse(t *testing.T) {
	for _, target := range []string{"/", "/login", "/settings/users", "/assets/x.js"} {
		t.Run(target, func(t *testing.T) {
			h := newHarness(t, false)
			rec := h.do(http.MethodGet, target)
			if rec.Code != http.StatusOK || rec.Body.String() != "spa" {
				t.Fatalf("status = %d body = %q, want the stub's 200 \"spa\"", rec.Code, rec.Body.String())
			}
			got := h.spa.received()
			if len(got) != 1 || got[0] != target {
				t.Errorf("SPA received %v, want [%s]", got, target)
			}
		})
	}
}

func TestRoot_APIHandlerRoutesToChi(t *testing.T) {
	h := newHarness(t, false)
	rec := h.do(http.MethodGet, "/api/v1/ping")
	if rec.Code != http.StatusOK || rec.Body.String() != `{"pong":true}` {
		t.Errorf("status = %d body = %q", rec.Code, rec.Body.String())
	}
}

func TestRoot_PanicInAPIHandler(t *testing.T) {
	h := newHarness(t, false)
	rec := h.do(http.MethodGet, "/api/v1/boom")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	p := decodeProblem(t, rec)
	if p.Code != "internal" {
		t.Errorf("code = %q, want internal", p.Code)
	}
	assertErrorLogWithRequestID(t, h.logs, rec.Header().Get("X-Request-Id"))
	// The process is still alive: the next request is served normally.
	if again := h.do(http.MethodGet, "/api/v1/ping"); again.Code != http.StatusOK {
		t.Errorf("request after the panic: status = %d, want 200", again.Code)
	}
}

func TestRoot_PanicInSPA(t *testing.T) {
	h := newHarness(t, false)
	h.spa.panic = true
	rec := h.do(http.MethodGet, "/login")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (recover is common to all destinations)", rec.Code)
	}
	assertErrorLogWithRequestID(t, h.logs, rec.Header().Get("X-Request-Id"))
	if again := h.do(http.MethodGet, "/healthz"); again.Code != http.StatusOK {
		t.Errorf("request after the panic: status = %d, want 200", again.Code)
	}
}

func assertErrorLogWithRequestID(t *testing.T, logs *syncBuffer, requestID string) {
	t.Helper()
	if requestID == "" {
		t.Fatal("response has no X-Request-Id")
	}
	for _, rec := range logs.lines(t) {
		if rec["level"] == "ERROR" && rec["request_id"] == requestID {
			return
		}
	}
	t.Errorf("no ERROR log with request_id=%s in:\n%s", requestID, logs.String())
}

// Every response (API, ops, SPA, errors) carries the common headers; HSTS depends on the mode (DD-24).
func TestRoot_CommonHeaders(t *testing.T) {
	targets := []struct{ name, method, path string }{
		{"api ok", http.MethodGet, "/api/v1/ping"},
		{"api 404", http.MethodGet, "/api/v1/no-existe"},
		{"api 405", http.MethodDelete, "/api/v1/ping"},
		{"ops", http.MethodGet, "/healthz"},
		{"spa", http.MethodGet, "/login"},
	}
	for _, mode := range []struct {
		name  string
		local bool
	}{{"non-local", false}, {"local", true}} {
		for _, tg := range targets {
			t.Run(mode.name+"/"+tg.name, func(t *testing.T) {
				h := newHarness(t, mode.local)
				rec := h.do(tg.method, tg.path)
				hd := rec.Header()
				if hd.Get("X-Request-Id") == "" {
					t.Error("missing X-Request-Id")
				}
				if got := hd.Get("X-Content-Type-Options"); got != "nosniff" {
					t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
				}
				if got := hd.Get("Referrer-Policy"); got != "no-referrer" {
					t.Errorf("Referrer-Policy = %q, want no-referrer", got)
				}
				hsts := hd.Get("Strict-Transport-Security")
				switch {
				case mode.local && hsts != "":
					t.Errorf("HSTS = %q in local mode, want absent (DD-24)", hsts)
				case !mode.local && !strings.HasPrefix(hsts, "max-age="):
					t.Errorf("HSTS = %q, want a max-age policy", hsts)
				}
			})
		}
	}
}

func TestRoot_RequestIDsAreUnique(t *testing.T) {
	h := newHarness(t, false)
	seen := map[string]bool{}
	for range 50 {
		id := h.do(http.MethodGet, "/healthz").Header().Get("X-Request-Id")
		if id == "" || seen[id] {
			t.Fatalf("request id %q is empty or repeated", id)
		}
		seen[id] = true
	}
}

// The client cannot choose the request id (no log injection, no spoofed correlation).
func TestRoot_IncomingRequestIDIsIgnored(t *testing.T) {
	h := newHarness(t, false)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-Id", "attacker-chosen\nfake=1")
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	if id := rec.Header().Get("X-Request-Id"); id == "" || strings.Contains(id, "attacker") {
		t.Errorf("X-Request-Id = %q, want a server-generated id", id)
	}
}

// The log line carries the route label, never the raw URL (DD-12).
func TestRoot_LogRoute(t *testing.T) {
	tests := []struct {
		name, method, path, wantRoute, rawMustNotAppear string
		wantStatus                                      float64
	}{
		{"spa", http.MethodGet, "/settings/users/secret-segment", "spa", "secret-segment", 200},
		{"ops", http.MethodGet, "/healthz", "ops", "", 200},
		{"api pattern", http.MethodGet, "/api/v1/things/12345", "/api/v1/things/{id}", "12345", 200},
		{"api not found", http.MethodGet, "/api/v1/no-existe-raw", "unmatched", "no-existe-raw", 404},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, false)
			rec := h.do(tt.method, tt.path)
			reqID := rec.Header().Get("X-Request-Id")
			var found map[string]any
			for _, l := range h.logs.lines(t) {
				if l["request_id"] == reqID && l["route"] != nil {
					found = l
				}
			}
			if found == nil {
				t.Fatalf("no request log with request_id=%s in:\n%s", reqID, h.logs.String())
			}
			if found["route"] != tt.wantRoute {
				t.Errorf("route = %v, want %q", found["route"], tt.wantRoute)
			}
			if found["status"] != tt.wantStatus {
				t.Errorf("status = %v, want %v", found["status"], tt.wantStatus)
			}
			if found["method"] != tt.method {
				t.Errorf("method = %v, want %s", found["method"], tt.method)
			}
			if _, ok := found["duration_ms"]; !ok {
				t.Error("log has no duration_ms")
			}
			if tt.rawMustNotAppear != "" && strings.Contains(h.logs.String(), tt.rawMustNotAppear) {
				t.Errorf("log contains the raw URL segment %q:\n%s", tt.rawMustNotAppear, h.logs.String())
			}
		})
	}
}

// A missing destination is a programming error caught when the handler is built, not a nil-pointer
// panic on the first request.
func TestNewRootHandler_PanicsOnMissingDependency(t *testing.T) {
	full := app.RootDeps{
		API:       app.NewAPIRouter(),
		Liveness:  app.LivenessHandler(),
		Readiness: app.ReadinessPlaceholder(),
		SPA:       app.SPAUnavailableHandler(),
	}
	tests := map[string]func(*app.RootDeps){
		"API":       func(d *app.RootDeps) { d.API = nil },
		"Liveness":  func(d *app.RootDeps) { d.Liveness = nil },
		"Readiness": func(d *app.RootDeps) { d.Readiness = nil },
		"SPA":       func(d *app.RootDeps) { d.SPA = nil },
	}
	for name, breakIt := range tests {
		t.Run(name, func(t *testing.T) {
			deps := full
			breakIt(&deps)
			defer func() {
				r := recover()
				msg, _ := r.(string)
				if !strings.Contains(msg, "RootDeps."+name) {
					t.Errorf("panic = %v, want a message naming RootDeps.%s", r, name)
				}
			}()
			app.NewRootHandler(deps, nil)
		})
	}
}

// middleware.GetHead (and any chi middleware that inspects the route context) must work behind the
// root mux, and the request log still carries the chi pattern.
func TestRoot_ChiMiddlewareThatNeedsTheRouteContext(t *testing.T) {
	api := app.NewAPIRouter()
	api.Use(middleware.GetHead)
	api.Get("/api/v1/things/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "thing")
	})
	h := newHarnessWithAPI(t, false, api)

	rec := h.do(http.MethodHead, "/api/v1/things/12345")
	if rec.Code != http.StatusOK {
		t.Fatalf("HEAD status = %d, want 200 (GetHead routes it to the GET handler)", rec.Code)
	}
	reqID := rec.Header().Get("X-Request-Id")
	for _, l := range h.logs.lines(t) {
		if l["request_id"] == reqID && l["route"] != nil {
			if l["route"] != "/api/v1/things/{id}" || l["method"] != http.MethodHead {
				t.Errorf("log route = %v method = %v, want the chi pattern for a HEAD", l["route"], l["method"])
			}
			return
		}
	}
	t.Errorf("no request log for %s:\n%s", reqID, h.logs.String())
}

// A panic after part of the response went out cannot become a 500: the client must see the
// connection cut (never a "complete" 200), and the log must say ERROR with the request id (and the
// request line must not claim status 200 at INFO).
func TestRoot_PanicAfterTheResponseStarted(t *testing.T) {
	api := app.NewAPIRouter()
	api.Get("/api/v1/partial", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "partial")
		w.(http.Flusher).Flush()
		panic("boom after writing")
	})
	h := newHarnessWithAPI(t, false, api)
	srv := httptest.NewServer(h.handler)
	defer srv.Close()

	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	resp, err := client.Get(srv.URL + "/api/v1/partial")
	if err != nil {
		return // the connection died before the headers arrived: also acceptable
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(resp.Body)
	if readErr == nil {
		t.Fatalf("client read the whole body %q without error: a corrupted response looked complete", body)
	}
	reqID := resp.Header.Get("X-Request-Id")
	if reqID == "" {
		t.Fatal("response has no X-Request-Id")
	}

	deadline := time.Now().Add(3 * time.Second)
	var sawPanicError, sawRequestLine bool
	for time.Now().Before(deadline) && (!sawPanicError || !sawRequestLine) {
		for _, l := range h.logs.lines(t) {
			if l["request_id"] != reqID {
				continue
			}
			if l["level"] == "ERROR" && l["msg"] == "panic recovered" {
				sawPanicError = true
			}
			if l["msg"] == "request" {
				sawRequestLine = true
				if l["status"] != float64(500) || l["level"] != "ERROR" {
					t.Errorf("request line status = %v level = %v, want 500 ERROR", l["status"], l["level"])
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !sawPanicError || !sawRequestLine {
		t.Errorf("panic ERROR logged = %v, request line logged = %v; logs:\n%s", sawPanicError, sawRequestLine, h.logs.String())
	}
}

// The 503 of the readiness placeholder and of the "SPA not built" handler are expected while those
// pieces do not exist yet: they are logged as WARN, not as ERROR.
func TestRoot_ExpectedPlaceholders503AreWarnings(t *testing.T) {
	logs := &syncBuffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	root := app.NewRootHandler(app.RootDeps{
		API:       app.NewAPIRouter(),
		Liveness:  app.LivenessHandler(),
		Readiness: app.ReadinessPlaceholder(),
		SPA:       app.SPAUnavailableHandler(),
	}, app.NewCommonMiddleware(logger, true, nil))

	for _, path := range []string{"/readyz", "/login"} {
		rec := httptest.NewRecorder()
		root.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s status = %d, want 503", path, rec.Code)
		}
		reqID := rec.Header().Get("X-Request-Id")
		found := false
		for _, l := range logs.lines(t) {
			if l["request_id"] == reqID && l["msg"] == "request" {
				found = true
				if l["level"] != "WARN" || l["status"] != float64(503) {
					t.Errorf("%s: level = %v status = %v, want WARN 503", path, l["level"], l["status"])
				}
			}
		}
		if !found {
			t.Errorf("%s: no request line", path)
		}
	}
}

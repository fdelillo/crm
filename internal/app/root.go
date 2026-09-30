package app

import (
	"log/slog"
	"net/http"
	"net/netip"
	"slices"
	"strings"

	"github.com/fdelillo/crm/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
)

// RootDeps are the four destinations of the root mux (DD-22).
type RootDeps struct {
	API       http.Handler // chi router serving everything under /api/ (INV-22)
	Liveness  http.Handler // GET /healthz
	Readiness http.Handler // GET /readyz
	SPA       http.Handler // everything else; web.NewHandler(dist) in production, a stub in backend tests
}

// CommonMiddleware wraps the whole root mux, so it applies to the API, ops and the SPA alike.
type CommonMiddleware func(http.Handler) http.Handler

// NewCommonMiddleware builds the middlewares shared by every destination: request id, recover,
// logging and security headers, outermost first (T-B204). local omits HSTS (DD-24).
// CrossOriginProtection joins this chain in T-B204.
func NewCommonMiddleware(logger *slog.Logger, local bool, trustedProxies ...[]netip.Prefix) CommonMiddleware {
	var trusted []netip.Prefix
	if len(trustedProxies) > 0 {
		trusted = trustedProxies[0]
	}
	csrf := http.NewCrossOriginProtection()
	csrf.SetDenyHandler(httpx.CSRFDenyHandler(logger))
	chain := []func(http.Handler) http.Handler{
		httpx.ClientIP(trusted),
		httpx.RequestID,
		httpx.Recover(logger),
		httpx.Logging(logger),
		httpx.SecurityHeaders(!local),
		csrf.Handler,
	}
	return func(next http.Handler) http.Handler {
		for _, mw := range slices.Backward(chain) {
			next = mw(next)
		}
		return next
	}
}

// NewRootHandler builds the root mux: /api/ → chi, /healthz and /readyz (GET only) → ops, and
// everything else → the SPA. The SPA is never registered in chi, so an unknown /api/... path is a
// 404 problem+json and never index.html (DD-22, INV-22).
func NewRootHandler(deps RootDeps, common CommonMiddleware) http.Handler {
	for name, h := range map[string]http.Handler{
		"API": deps.API, "Liveness": deps.Liveness, "Readiness": deps.Readiness, "SPA": deps.SPA,
	} {
		if h == nil {
			panic("app: RootDeps." + name + " is nil")
		}
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", deps.API)
	// Not "GET /healthz": with the catch-all "/" below, a POST would match "/" and reach the SPA
	// instead of getting a 405 from the mux. The method check is done here.
	mux.Handle("/healthz", httpx.WithRoute("ops", getOnly(deps.Liveness)))
	mux.Handle("/readyz", httpx.WithRoute("ops", getOnly(deps.Readiness)))
	mux.Handle("/", spaUnlessAPI(deps.API, deps.SPA))
	if common == nil {
		return mux
	}
	return common(mux)
}

// getOnly answers 405 (with Allow) to anything but GET and HEAD.
func getOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// spaUnlessAPI serves the SPA, except for anything that is under /api/ once decoded. The ServeMux
// routes on the escaped path, so "/api%2Fv1%2Fx" would otherwise fall into the SPA's catch-all with
// URL.Path "/api/v1/x" (INV-22): it goes to the API, which answers 404 problem+json.
func spaUnlessAPI(api, spa http.Handler) http.Handler {
	spa = httpx.WithRoute("spa", spa)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			api.ServeHTTP(w, r)
			return
		}
		spa.ServeHTTP(w, r)
	})
}

// NewAPIRouter returns the chi router of the API with its own 404 and 405 in problem+json.
// Modules register their routes on it; the SPA is never registered here.
func NewAPIRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Use(httpx.NoStore)
	// First middleware: once the request has been routed (or not), report the chi pattern to the
	// request log. It runs on 404 and 405 too, and lets chi build its own route context.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			defer func() {
				if pattern := chi.RouteContext(req.Context()).RoutePattern(); pattern != "" {
					httpx.SetRoute(req, pattern)
				}
			}()
			next.ServeHTTP(w, req)
		})
	})
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		httpx.WriteProblem(w, req, httpx.CodeNotFound)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		// chi's default 405 sets Allow, a custom handler does not (RFC 9110 requires it).
		for _, m := range []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
			http.MethodPatch, http.MethodDelete, http.MethodOptions} {
			if r.Match(chi.NewRouteContext(), m, req.URL.Path) {
				w.Header().Add("Allow", m)
			}
		}
		httpx.MethodNotAllowed(w, req)
	})
	return r
}

package app

import (
	"io"
	"net/http"
)

// LivenessHandler answers GET /healthz: 200 while the process serves requests. It never touches
// the database (plan §12.2).
func LivenessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	})
}

// ReadinessPlaceholder answers GET /readyz with 503 until T-B903 implements the real check
// (database reachable and migrations up to date). It exists so /readyz is never served by the SPA.
func ReadinessPlaceholder() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{"status":"unavailable"}`)
	})
}

// spaUnavailableMessage is the same text the web package will use without index.html (ADR-019).
const spaUnavailableMessage = "La interfaz no está compilada (correr `make web-build`)"

// SPAUnavailableHandler is the RootDeps.SPA of `crm serve` until T-F008 replaces it with
// web.NewHandler(web.DistFS()).
func SPAUnavailableHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, spaUnavailableMessage)
	})
}

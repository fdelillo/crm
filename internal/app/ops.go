package app

import (
	"io"
	"log/slog"
	"net/http"

	"github.com/fdelillo/crm/internal/platform/httpx"
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

// ReadinessPlaceholder is the unavailable handler used by composition tests without a database.
func ReadinessPlaceholder() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		httpx.SetLogLevel(r, slog.LevelWarn)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{"status":"unavailable"}`)
	})
}

// spaUnavailableMessage is the same text the web package will use without index.html (ADR-019).
const spaUnavailableMessage = "La interfaz no está compilada (correr `make web-build`)"

// SPAUnavailableHandler is the RootDeps.SPA of `crm serve` until T-F008 replaces it with
// web.NewHandler(web.DistFS()).
func SPAUnavailableHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		httpx.SetLogLevel(r, slog.LevelWarn) // expected until T-F008
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, spaUnavailableMessage)
	})
}

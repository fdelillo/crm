package orders

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Registering the SPA in the API router breaks INV-22.
func router(spa http.Handler) http.Handler {
	r := chi.NewRouter()
	r.Mount("/", spa)
	r.NotFound(spa.ServeHTTP)
	r.Get("/*", func(http.ResponseWriter, *http.Request) {})
	return r
}

package orders

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Constant SQL is fine, and so are messages that merely start with an SQL verb.
const listOrders = "SELECT id, tenant_id FROM app.orders WHERE tenant_id = $1"

func failed(err error) error { return errors.New("update failed: " + err.Error()) }

func describe(n int) string { return fmt.Sprintf("selected %d rows", n) }

func label(name string) string { return "delete requested by " + name }

// Routes relative to a sub-router, the "/" of a group, a 404 in problem+json and a role name that
// only mentions the word are all fine.
func router(notFound http.HandlerFunc) http.Handler {
	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/", func(http.ResponseWriter, *http.Request) {})
		r.Get("/orders/{id}", func(http.ResponseWriter, *http.Request) {})
		r.Mount("/admin", chi.NewRouter())
	})
	r.NotFound(notFound)
	return r
}

const roleDoc = "the role of the user"

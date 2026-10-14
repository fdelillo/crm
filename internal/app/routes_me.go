package app

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/platform/httpx"
	"github.com/fdelillo/crm/internal/tenant"
	"github.com/go-chi/chi/v5"
)

// RegisterMeRoute assembles the user and company views under their own modules' transactions.
func RegisterMeRoute(r chi.Router, users *identity.Service, companies *tenant.Service, logger *slog.Logger) {
	r.With(identity.Authenticate(users, logger)).Get("/api/v1/me", func(w http.ResponseWriter, req *http.Request) {
		principal, ok := authz.PrincipalFrom(req.Context())
		if !ok {
			httpx.WriteProblem(w, req, httpx.CodeUnauthenticated)
			return
		}
		user, err := users.Me(req.Context(), principal)
		if err != nil {
			httpx.WriteDBError(w, req, err, logger)
			return
		}
		company, err := companies.SummaryFor(req.Context(), principal.TenantID)
		if err != nil {
			httpx.WriteDBError(w, req, err, logger)
			return
		}
		body, err := json.Marshal(struct {
			User        identity.CurrentUser `json:"user"`
			Tenant      tenant.Summary       `json:"tenant"`
			Permissions []authz.Permission   `json:"permissions"`
		}{User: user, Tenant: company, Permissions: authz.Permissions(principal.Role)})
		if err != nil {
			panic(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})
}

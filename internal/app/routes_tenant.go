package app

import (
	"log/slog"

	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/tenant"
	"github.com/go-chi/chi/v5"
)

func RegisterTenantRoutes(r chi.Router, users *identity.Service, companies *tenant.Service, logger *slog.Logger) {
	r.Group(func(r chi.Router) {
		r.Use(identity.Authenticate(users, logger))
		tenant.RegisterCompanyRoutes(r, companies, logger)
	})
}

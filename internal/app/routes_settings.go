package app

import (
	"log/slog"

	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/settings"
	"github.com/go-chi/chi/v5"
)

// T-002-B205 (DD-1): the group exists so each operation only adds its route in settings.RegisterRoutes.
func RegisterSettingsRoutes(r chi.Router, users *identity.Service, configuration *settings.Service, logger *slog.Logger) {
	r.Group(func(r chi.Router) {
		r.Use(identity.Authenticate(users, logger))
		settings.RegisterRoutes(r, configuration, logger)
	})
}

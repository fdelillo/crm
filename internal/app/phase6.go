package app

import (
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/platform/ratelimit"
	"github.com/fdelillo/crm/internal/tenant"
	"github.com/go-chi/chi/v5"
	"log/slog"
)

func RegisterUserRoutes(r chi.Router, users *identity.Service, companies *tenant.Service, invitationsLimiter *ratelimit.Limiter, logger *slog.Logger) {
}

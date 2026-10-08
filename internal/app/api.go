package app

import (
	"log/slog"
	"time"

	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/industrytemplate"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/ratelimit"
	"github.com/fdelillo/crm/internal/tenant"
	"github.com/go-chi/chi/v5"
	"golang.org/x/time/rate"
)

// BuildAPIRouter is the composition root for the API served by crm serve.
// Isolation tests walk this exact router (ADR-002, T-B801): new routes belong here.
// Rate limits, middleware order and registrations are preserved from serve.go.
func BuildAPIRouter(users *identity.Service, companies *tenant.Service, c clock.Clock, logger *slog.Logger) chi.Router {
	api := NewAPIRouter()
	industrytemplate.RegisterRoutes(api)
	tenant.RegisterRoutes(api, companies, ratelimit.NewLimiter(rate.Every(12*time.Minute), 5, c, time.Hour), logger)
	RegisterAuthRoutes(api, users, companies, ratelimit.NewLimiter(rate.Every(3*time.Second), 20, c, time.Minute), logger)
	RegisterMeRoute(api, users, companies, logger)
	RegisterRecoveryRoutes(api, users, ratelimit.NewLimiter(rate.Every(12*time.Minute), 5, c, time.Hour),
		ratelimit.NewLimiter(rate.Every(3*time.Minute), 20, c, time.Hour), logger)
	RegisterUserRoutes(api, users, companies, ratelimit.NewLimiter(rate.Every(3*time.Minute), 20, c, time.Hour), logger)
	RegisterTenantRoutes(api, users, companies, logger)
	return api
}

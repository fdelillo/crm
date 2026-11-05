package settings

import (
	"log/slog"

	"github.com/go-chi/chi/v5"
)

// RegisterRoutes expects Authenticate outside the group it receives.
// T-002-B205 (DD-1): registers nothing yet; each operation adds its route with its own task.
func RegisterRoutes(r chi.Router, service *Service, logger *slog.Logger) {}

package tenant

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync/atomic"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/httpx"
	"github.com/fdelillo/crm/internal/platform/ratelimit"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var signupEmailExistsCount, signupLockTimeoutCount atomic.Int64

func SignupEmailExistsCount() int64 { return signupEmailExistsCount.Load() }
func SignupLockTimeoutCount() int64 { return signupLockTimeoutCount.Load() }

// RegisterRoutes registers the public signup endpoint. The limiter spends a token before decoding
// or validating the body, so rejected attempts also consume the hourly budget.
func RegisterRoutes(r chi.Router, service *Service, limiter *ratelimit.Limiter, logger *slog.Logger) {
	r.With(ratelimit.Middleware(limiter)).Post("/api/v1/auth/signup", func(w http.ResponseWriter, req *http.Request) {
		var input Signup
		if err := httpx.DecodeJSON(w, req, &input); err != nil {
			return
		}
		meta := identity.RequestMeta{IP: httpx.ClientIPFrom(req.Context()), UserAgent: req.UserAgent(),
			RequestID: httpx.RequestIDFrom(req.Context())}
		result, err := service.Register(req.Context(), input, meta)
		if err != nil {
			var fields FieldErrors
			if errors.As(err, &fields) {
				httpx.ValidationError(w, req, fields)
				return
			}
			if errors.Is(err, identity.ErrEmailTaken) {
				signupEmailExistsCount.Add(1)
				logger.WarnContext(req.Context(), "signup email exists", "security_event", "signup_email_exists", "ip", meta.IP.String())
				httpx.WriteProblem(w, req, httpx.CodeEmailAlreadyRegistered,
					httpx.WithSuggestedAction(httpx.SuggestedPasswordReset))
				return
			}
			var pgErr *pgconn.PgError
			if errors.Is(err, db.ErrUnavailable) && errors.As(err, &pgErr) && pgErr.Code == "55P03" {
				signupLockTimeoutCount.Add(1)
				logger.WarnContext(req.Context(), "signup lock timeout", "event", "signup_lock_timeout")
				httpx.WriteProblem(w, req, httpx.CodeServiceUnavailable, httpx.RetryAfter(SignupLockTimeout))
				return
			}
			httpx.WriteDBError(w, req, err, logger)
			return
		}
		identity.SetSessionCookie(w, result.Session.RawToken)
		body, err := json.Marshal(struct {
			User        identity.CurrentUser `json:"user"`
			Tenant      Summary              `json:"tenant"`
			Permissions []authz.Permission   `json:"permissions"`
		}{User: result.User, Tenant: result.Tenant, Permissions: authz.Permissions(result.User.Role)})
		if err != nil {
			panic(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(body)
	})
}

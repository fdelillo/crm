package app

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/platform/httpx"
	"github.com/fdelillo/crm/internal/platform/ratelimit"
	"github.com/go-chi/chi/v5"
)

// RegisterRecoveryRoutes: the resend of the verification has, after the IP quota shared with confirm
// (DD-9) and after Authenticate, a quota per user (DD-46 (3), amendment H-16). It counts every request
// that reaches it and answers 429 without calling the service, so a refusal queues nothing and revokes
// no link.
func RegisterRecoveryRoutes(r chi.Router, users *identity.Service, resetLimiter, verificationLimiter, resendLimiter *ratelimit.Limiter, logger *slog.Logger) {
	r.With(ratelimit.Middleware(resetLimiter)).Post("/api/v1/auth/password-reset", func(w http.ResponseWriter, req *http.Request) {
		var input struct {
			Email string `json:"email"`
		}
		if err := httpx.DecodeJSON(w, req, &input); err != nil {
			return
		}
		email := strings.ToLower(strings.TrimSpace(input.Email))
		if !identity.ValidEmail(email) {
			httpx.ValidationError(w, req, map[string]string{"email": "invalid_format"})
			return
		}
		if err := users.RequestPasswordReset(req.Context(), email, requestMeta(req)); err != nil {
			httpx.WriteDBError(w, req, err, logger)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	r.With(ratelimit.Middleware(resetLimiter)).Post("/api/v1/auth/password-reset/confirm", func(w http.ResponseWriter, req *http.Request) {
		var input struct {
			Token    string `json:"token"`
			Password string `json:"password"`
		}
		if err := httpx.DecodeJSON(w, req, &input); err != nil {
			return
		}
		err := users.ConfirmPasswordReset(req.Context(), input.Token, input.Password, requestMeta(req))
		var invalid *identity.PasswordValidationError
		switch {
		case errors.Is(err, identity.ErrTokenInvalid):
			httpx.WriteProblem(w, req, httpx.CodeTokenInvalid)
		case errors.As(err, &invalid):
			httpx.ValidationError(w, req, map[string]string{"password": invalid.Code})
		case err != nil:
			httpx.WriteDBError(w, req, err, logger)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	})
	r.With(ratelimit.Middleware(verificationLimiter)).Post("/api/v1/auth/email-verification/confirm", func(w http.ResponseWriter, req *http.Request) {
		var input struct {
			Token string `json:"token"`
		}
		if err := httpx.DecodeJSON(w, req, &input); err != nil {
			return
		}
		err := users.ConfirmEmailVerification(req.Context(), input.Token, requestMeta(req))
		switch {
		case errors.Is(err, identity.ErrTokenInvalid):
			httpx.WriteProblem(w, req, httpx.CodeTokenInvalid)
		case err != nil:
			httpx.WriteDBError(w, req, err, logger)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	})
	r.With(ratelimit.Middleware(verificationLimiter), identity.Authenticate(users, logger), ratelimit.MiddlewareBy(resendLimiter, userKey)).Post("/api/v1/auth/email-verification/resend", func(w http.ResponseWriter, req *http.Request) {
		principal, ok := authz.PrincipalFrom(req.Context())
		if !ok {
			httpx.WriteProblem(w, req, httpx.CodeUnauthenticated)
			return
		}
		if err := users.ResendEmailVerification(req.Context(), principal); err != nil {
			httpx.WriteDBError(w, req, err, logger)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
}

func requestMeta(req *http.Request) identity.RequestMeta {
	return identity.RequestMeta{IP: httpx.ClientIPFrom(req.Context()), UserAgent: req.UserAgent(), RequestID: httpx.RequestIDFrom(req.Context())}
}

// userKey is the quota key of the resend: the user of the session. It runs after Authenticate; without
// a Principal the route is wired wrong, and the middleware answers 500.
func userKey(r *http.Request) (string, bool) {
	p, ok := authz.PrincipalFrom(r.Context())
	if !ok {
		return "", false
	}
	return p.UserID.String(), true
}

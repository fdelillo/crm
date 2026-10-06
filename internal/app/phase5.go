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

func RegisterRecoveryRoutes(r chi.Router, users *identity.Service, resetLimiter, verificationLimiter *ratelimit.Limiter, logger *slog.Logger) {
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
	r.With(ratelimit.Middleware(verificationLimiter), identity.Authenticate(users, logger)).Post("/api/v1/auth/email-verification/resend", func(w http.ResponseWriter, req *http.Request) {
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

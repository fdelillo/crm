package app

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/platform/httpx"
	"github.com/fdelillo/crm/internal/platform/ratelimit"
	"github.com/fdelillo/crm/internal/tenant"
	"github.com/go-chi/chi/v5"
)

// RegisterRoutes exposes login and idempotent logout. Login spends an IP token before decoding.
func RegisterAuthRoutes(r chi.Router, users *identity.Service, companies *tenant.Service, limiter *ratelimit.Limiter, logger *slog.Logger) {
	r.With(ratelimit.Middleware(limiter)).Post("/api/v1/auth/login", func(w http.ResponseWriter, req *http.Request) {
		var input struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := httpx.DecodeJSON(w, req, &input); err != nil {
			return
		}
		fields := map[string]string{}
		email := strings.ToLower(strings.TrimSpace(input.Email))
		address, parseErr := mail.ParseAddress(email)
		if email == "" || len(email) > 254 || parseErr != nil || address.Address != email || strings.ContainsRune(email, '\x00') {
			fields["email"] = "invalid_format"
		}
		if input.Password == "" || utf8.RuneCountInString(input.Password) > 128 {
			fields["password"] = "invalid_value"
		}
		if len(fields) > 0 {
			httpx.ValidationError(w, req, fields)
			return
		}
		meta := identity.RequestMeta{IP: httpx.ClientIPFrom(req.Context()), UserAgent: req.UserAgent(), RequestID: httpx.RequestIDFrom(req.Context())}
		session, err := users.Login(req.Context(), input.Email, input.Password, meta)
		if err != nil {
			var locked *identity.LockedError
			switch {
			case errors.Is(err, identity.ErrInvalidCredentials):
				httpx.WriteProblem(w, req, httpx.CodeInvalidCredentials)
			case errors.Is(err, identity.ErrAccountDisabled):
				httpx.WriteProblem(w, req, httpx.CodeAccountDisabled)
			case errors.As(err, &locked):
				httpx.WriteProblem(w, req, httpx.CodeLoginLocked, httpx.RetryAfter(locked.RetryAfter))
			default:
				httpx.WriteDBError(w, req, err, logger)
			}
			return
		}
		user, err := users.Me(req.Context(), session.Principal)
		if err != nil {
			httpx.WriteDBError(w, req, err, logger)
			return
		}
		company, err := companies.SummaryFor(req.Context(), session.Principal.TenantID)
		if err != nil {
			httpx.WriteDBError(w, req, err, logger)
			return
		}
		body, err := json.Marshal(struct {
			User        identity.CurrentUser `json:"user"`
			Tenant      tenant.Summary       `json:"tenant"`
			Permissions []authz.Permission   `json:"permissions"`
		}{user, company, authz.Permissions(session.Principal.Role)})
		if err != nil {
			panic(err)
		}
		identity.SetSessionCookie(w, session)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})
	r.Post("/api/v1/auth/logout", func(w http.ResponseWriter, req *http.Request) {
		var raw string
		if cookie, err := req.Cookie(identity.SessionCookieName); err == nil {
			raw = cookie.Value
		}
		meta := identity.RequestMeta{IP: httpx.ClientIPFrom(req.Context()), UserAgent: req.UserAgent(), RequestID: httpx.RequestIDFrom(req.Context())}
		if err := users.Logout(req.Context(), raw, meta); err != nil {
			httpx.WriteDBError(w, req, err, logger)
			return
		}
		identity.ClearSessionCookie(w)
		w.WriteHeader(http.StatusNoContent)
	})
}

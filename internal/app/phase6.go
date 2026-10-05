package app

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/platform/httpx"
	"github.com/fdelillo/crm/internal/platform/ratelimit"
	"github.com/fdelillo/crm/internal/tenant"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// RegisterUserRoutes shares one IP quota between preview and accept (DD-9).
// Management permission is checked before parsing or searching any target ID (INV-12).
func RegisterUserRoutes(r chi.Router, users *identity.Service, companies *tenant.Service, invitationsLimiter *ratelimit.Limiter, logger *slog.Logger) {
	r.With(ratelimit.Middleware(invitationsLimiter)).Post("/api/v1/auth/invitations/preview", func(w http.ResponseWriter, req *http.Request) {
		var input struct {
			Token string `json:"token"`
		}
		if err := httpx.DecodeJSON(w, req, &input); err != nil {
			return
		}
		preview, err := users.PreviewInvitation(req.Context(), input.Token)
		if writeUserError(w, req, err, logger) {
			return
		}
		writeUsersJSON(w, http.StatusOK, preview)
	})
	r.With(ratelimit.Middleware(invitationsLimiter)).Post("/api/v1/auth/invitations/accept", func(w http.ResponseWriter, req *http.Request) {
		var input struct {
			Token    string `json:"token"`
			Name     string `json:"name"`
			Password string `json:"password"`
		}
		if err := httpx.DecodeJSON(w, req, &input); err != nil {
			return
		}
		session, err := users.AcceptInvitation(req.Context(), input.Token, input.Name, input.Password, requestMeta(req))
		if writeUserError(w, req, err, logger) {
			return
		}
		user, err := users.Me(req.Context(), session.Principal)
		if writeUserError(w, req, err, logger) {
			return
		}
		company, err := companies.SummaryFor(req.Context(), session.Principal.TenantID)
		if writeUserError(w, req, err, logger) {
			return
		}
		identity.SetSessionCookie(w, session)
		writeUsersJSON(w, http.StatusCreated, struct {
			User        identity.CurrentUser `json:"user"`
			Tenant      tenant.Summary       `json:"tenant"`
			Permissions []authz.Permission   `json:"permissions"`
		}{user, company, authz.Permissions(session.Principal.Role)})
	})
	r.Group(func(r chi.Router) {
		r.Use(identity.Authenticate(users, logger), authz.RequirePermission(authz.SettingsManage))
		r.Get("/api/v1/users", func(w http.ResponseWriter, req *http.Request) {
			p, _ := authz.PrincipalFrom(req.Context())
			items, err := users.ListUsers(req.Context(), p)
			if writeUserError(w, req, err, logger) {
				return
			}
			writeUsersJSON(w, http.StatusOK, struct {
				Items []identity.User `json:"items"`
			}{items})
		})
		r.Post("/api/v1/users/invitations", func(w http.ResponseWriter, req *http.Request) {
			var input struct {
				Email string     `json:"email"`
				Role  authz.Role `json:"role"`
			}
			if err := httpx.DecodeJSON(w, req, &input); err != nil {
				return
			}
			p, _ := authz.PrincipalFrom(req.Context())
			user, reissued, err := users.Invite(req.Context(), p, input.Email, input.Role, requestMeta(req))
			if writeUserError(w, req, err, logger) {
				return
			}
			status := http.StatusCreated
			if reissued {
				status = http.StatusOK
			}
			writeUsersJSON(w, status, user)
		})
		r.Put("/api/v1/users/{userId}/role", func(w http.ResponseWriter, req *http.Request) {
			id, err := uuid.Parse(chi.URLParam(req, "userId"))
			if err != nil {
				httpx.WriteProblem(w, req, httpx.CodeNotFound)
				return
			}
			var input struct {
				Role authz.Role `json:"role"`
			}
			if err := httpx.DecodeJSON(w, req, &input); err != nil {
				return
			}
			p, _ := authz.PrincipalFrom(req.Context())
			user, err := users.ChangeRole(req.Context(), p, id, input.Role, requestMeta(req))
			if writeUserError(w, req, err, logger) {
				return
			}
			writeUsersJSON(w, http.StatusOK, user)
		})
		r.Post("/api/v1/users/{userId}/deactivate", func(w http.ResponseWriter, req *http.Request) {
			id, err := uuid.Parse(chi.URLParam(req, "userId"))
			if err != nil {
				httpx.WriteProblem(w, req, httpx.CodeNotFound)
				return
			}
			p, _ := authz.PrincipalFrom(req.Context())
			user, err := users.Deactivate(req.Context(), p, id, requestMeta(req))
			if writeUserError(w, req, err, logger) {
				return
			}
			writeUsersJSON(w, http.StatusOK, user)
		})
		r.Post("/api/v1/users/{userId}/reactivate", func(w http.ResponseWriter, req *http.Request) {
			id, err := uuid.Parse(chi.URLParam(req, "userId"))
			if err != nil {
				httpx.WriteProblem(w, req, httpx.CodeNotFound)
				return
			}
			p, _ := authz.PrincipalFrom(req.Context())
			user, err := users.Reactivate(req.Context(), p, id, requestMeta(req))
			if writeUserError(w, req, err, logger) {
				return
			}
			writeUsersJSON(w, http.StatusOK, user)
		})
	})
}

func writeUserError(w http.ResponseWriter, req *http.Request, err error, logger *slog.Logger) bool {
	if err == nil {
		return false
	}
	var invalid *identity.UserValidationError
	var invalidPassword *identity.PasswordValidationError
	switch {
	case errors.Is(err, identity.ErrUserNotFound):
		httpx.WriteProblem(w, req, httpx.CodeNotFound)
	case errors.Is(err, identity.ErrLastAdmin):
		httpx.WriteProblem(w, req, httpx.CodeLastAdmin)
	case errors.Is(err, identity.ErrInvalidTransition):
		httpx.WriteProblem(w, req, httpx.CodeInvalidState)
	case errors.Is(err, identity.ErrEmailTaken):
		httpx.WriteProblem(w, req, httpx.CodeEmailTaken)
	case errors.Is(err, identity.ErrTokenInvalid):
		httpx.WriteProblem(w, req, httpx.CodeTokenInvalid)
	case errors.As(err, &invalid):
		httpx.ValidationError(w, req, invalid.Fields)
	case errors.As(err, &invalidPassword):
		httpx.ValidationError(w, req, map[string]string{"password": invalidPassword.Code})
	default:
		httpx.WriteDBError(w, req, err, logger)
	}
	return true
}

func writeUsersJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

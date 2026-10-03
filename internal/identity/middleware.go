package identity

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/platform/httpx"
)

const SessionCookieName = "__Host-crm_session"

// Authenticate resolves the cookie on every protected request and injects the current Principal.
func Authenticate(service *Service, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil {
				httpx.WriteProblem(w, r, httpx.CodeUnauthenticated)
				return
			}
			principal, err := service.ResolveSession(r.Context(), cookie.Value)
			if errors.Is(err, ErrUnauthenticated) {
				ClearSessionCookie(w)
				httpx.WriteProblem(w, r, httpx.CodeUnauthenticated)
				return
			}
			if err != nil {
				httpx.WriteDBError(w, r, err, logger)
				return
			}
			next.ServeHTTP(w, r.WithContext(authz.WithPrincipal(r.Context(), principal)))
		})
	}
}

func SetSessionCookie(w http.ResponseWriter, raw string) {
	http.SetCookie(w, &http.Cookie{Name: SessionCookieName, Value: raw, Path: "/", MaxAge: 7 * 24 * 60 * 60,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
}

func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: SessionCookieName, Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
}

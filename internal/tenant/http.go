package tenant

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"sync/atomic"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/httpx"
	"github.com/fdelillo/crm/internal/platform/objectstore"
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
		identity.SetSessionCookie(w, result.Session)
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

// RegisterCompanyRoutes expects Authenticate outside this group. Permission checks
// precede decoding multipart/JSON or looking up resources (INV-12).
func RegisterCompanyRoutes(r chi.Router, service *Service, logger *slog.Logger) {
	r.Get("/api/v1/tenant", func(w http.ResponseWriter, req *http.Request) {
		p, _ := authz.PrincipalFrom(req.Context())
		out, err := service.Get(req.Context(), p)
		if writeCompanyError(w, req, err, logger) {
			return
		}
		writeCompanyJSON(w, out)
	})
	r.Get("/api/v1/tenant/logo", func(w http.ResponseWriter, req *http.Request) {
		p, _ := authz.PrincipalFrom(req.Context())
		out, err := service.GetLogo(req.Context(), p, req.Header.Get("If-None-Match"))
		if writeCompanyError(w, req, err, logger) {
			return
		}
		w.Header().Set("Cache-Control", "private, no-cache")
		w.Header().Set("ETag", out.ETag)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Type", out.ContentType)
		if out.NotModified {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		defer func() { _ = out.Body.Close() }()
		w.Header().Set("Content-Length", strconv.FormatInt(out.Size, 10))
		w.WriteHeader(http.StatusOK)
		if _, err := io.Copy(w, out.Body); err != nil {
			logger.WarnContext(req.Context(), "logo response transfer failed", "event", "logo_transfer_failed", "error", err)
		}
	})
	r.Group(func(r chi.Router) {
		r.Use(authz.RequirePermission(authz.SettingsManage))
		r.Patch("/api/v1/tenant", func(w http.ResponseWriter, req *http.Request) {
			var input Update
			if err := httpx.DecodeJSON(w, req, &input); err != nil {
				return
			}
			p, _ := authz.PrincipalFrom(req.Context())
			out, err := service.Update(req.Context(), p, input, companyRequestMeta(req))
			if writeCompanyError(w, req, err, logger) {
				return
			}
			writeCompanyJSON(w, out)
		})
		r.Put("/api/v1/tenant/logo", func(w http.ResponseWriter, req *http.Request) {
			data, err := readLogoMultipart(w, req)
			if writeCompanyError(w, req, err, logger) {
				return
			}
			p, _ := authz.PrincipalFrom(req.Context())
			out, err := service.SetLogo(req.Context(), p, data, companyRequestMeta(req))
			if writeCompanyError(w, req, err, logger) {
				return
			}
			writeCompanyJSON(w, out)
		})
		r.Delete("/api/v1/tenant/logo", func(w http.ResponseWriter, req *http.Request) {
			p, _ := authz.PrincipalFrom(req.Context())
			if writeCompanyError(w, req, service.RemoveLogo(req.Context(), p, companyRequestMeta(req)), logger) {
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
	})
}

var errMalformedMultipart = errors.New("tenant: malformed multipart")

func readLogoMultipart(w http.ResponseWriter, req *http.Request) ([]byte, error) {
	mediaType, _, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" {
		return nil, ErrLogoUnsupportedType
	}
	req.Body = http.MaxBytesReader(w, req.Body, LogoMaxBodyBytes)
	reader, err := req.MultipartReader()
	if err != nil {
		return nil, errMalformedMultipart
	}
	part, err := reader.NextPart()
	if errors.Is(err, io.EOF) {
		return nil, FieldErrors{"file": "required"}
	}
	if err != nil {
		return nil, multipartError(err)
	}
	if part.FormName() != "file" {
		return nil, errMalformedMultipart
	}
	// Do not Close/drain an oversize part: Close would read its unread remainder.
	data, err := io.ReadAll(io.LimitReader(part, LogoMaxBytes+1))
	if len(data) > LogoMaxBytes {
		return nil, ErrLogoTooLarge
	}
	if err != nil {
		return nil, multipartError(err)
	}
	if _, err = reader.NextPart(); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errMalformedMultipart
		}
		return nil, multipartError(err)
	}
	// NextPart ignores an epilogue. Still enforce the limit on the complete body.
	if _, err = io.Copy(io.Discard, req.Body); err != nil {
		return nil, multipartError(err)
	}
	if len(data) == 0 {
		return nil, FieldErrors{"file": "required"}
	}
	return data, nil
}
func multipartError(err error) error {
	var max *http.MaxBytesError
	if errors.As(err, &max) {
		return ErrLogoTooLarge
	}
	return errMalformedMultipart
}
func companyRequestMeta(req *http.Request) identity.RequestMeta {
	return identity.RequestMeta{IP: httpx.ClientIPFrom(req.Context()), UserAgent: req.UserAgent(), RequestID: httpx.RequestIDFrom(req.Context())}
}
func writeCompanyJSON(w http.ResponseWriter, value Tenant) {
	body, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
func writeCompanyError(w http.ResponseWriter, req *http.Request, err error, logger *slog.Logger) bool {
	if err == nil {
		return false
	}
	var fields FieldErrors
	switch {
	case errors.As(err, &fields):
		httpx.ValidationError(w, req, fields)
	case errors.Is(err, ErrMalformedUpdate), errors.Is(err, errMalformedMultipart):
		httpx.WriteProblem(w, req, httpx.CodeMalformedRequest)
	case errors.Is(err, ErrLogoTooLarge):
		httpx.WriteProblem(w, req, httpx.CodePayloadTooLarge)
	case errors.Is(err, ErrLogoUnsupportedType):
		httpx.WriteProblem(w, req, httpx.CodeUnsupportedMediaType)
	case errors.Is(err, ErrLogoInvalidImage):
		httpx.ValidationError(w, req, map[string]string{"file": "invalid_value"})
	case errors.Is(err, authz.ErrForbidden):
		httpx.WriteProblem(w, req, httpx.CodeForbidden)
	case errors.Is(err, objectstore.ErrNotFound):
		httpx.WriteProblem(w, req, httpx.CodeNotFound)
	case errors.Is(err, objectstore.ErrUnavailable):
		if req.Context().Err() != nil {
			httpx.WriteDBError(w, req, db.MapError(req.Context().Err()), logger)
		} else {
			logger.WarnContext(req.Context(), "logo storage unavailable", "event", "logo_storage_unavailable", "error", err)
			httpx.WriteProblem(w, req, httpx.CodeServiceUnavailable)
		}
	default:
		httpx.WriteDBError(w, req, db.MapError(err), logger)
	}
	return true
}

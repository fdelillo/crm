package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/fdelillo/crm/internal/platform/db"
)

// Code is the closed ErrorCode enum in the HTTP contract.
type Code string

const (
	CodeMalformedRequest       Code = "malformed_request"
	CodeValidationFailed       Code = "validation_failed"
	CodeUnsupportedMediaType   Code = "unsupported_media_type"
	CodePayloadTooLarge        Code = "payload_too_large"
	CodeUnauthenticated        Code = "unauthenticated"
	CodeInvalidCredentials     Code = "invalid_credentials"
	CodeAccountDisabled        Code = "account_disabled"
	CodeForbidden              Code = "forbidden"
	CodeNotFound               Code = "not_found"
	CodeMethodNotAllowed       Code = "method_not_allowed"
	CodeEmailAlreadyRegistered Code = "email_already_registered"
	CodeEmailTaken             Code = "email_taken"
	CodeLastAdmin              Code = "last_admin"
	CodeInvalidState           Code = "invalid_state"
	CodeTokenInvalid           Code = "token_invalid"
	CodeLoginLocked            Code = "login_locked"
	CodeRateLimited            Code = "rate_limited"
	CodeServiceUnavailable     Code = "service_unavailable"
	CodeInternal               Code = "internal"
)

type spec struct {
	status        int
	title, detail string
}

var problems = map[Code]spec{
	CodeMalformedRequest:       {400, "Solicitud incorrecta", ""},
	CodeValidationFailed:       {422, "Los datos ingresados no son válidos", ""},
	CodeUnsupportedMediaType:   {415, "Tipo de contenido no admitido", ""},
	CodePayloadTooLarge:        {413, "El contenido es demasiado grande", ""},
	CodeUnauthenticated:        {401, "Iniciá sesión para continuar", ""},
	CodeInvalidCredentials:     {401, "Email o contraseña incorrectos", ""},
	CodeAccountDisabled:        {403, "La cuenta está desactivada", ""},
	CodeForbidden:              {403, "No tenés permiso para realizar esta acción", ""},
	CodeNotFound:               {404, "Recurso no encontrado", ""},
	CodeMethodNotAllowed:       {405, "Método no permitido", ""},
	CodeEmailAlreadyRegistered: {409, "Ya existe un usuario con ese email", "Ya existe un usuario con ese email. ¿Querés recuperar la contraseña?"},
	CodeEmailTaken:             {409, "Ese email ya está en uso", ""},
	CodeLastAdmin:              {409, "La empresa necesita al menos un administrador activo", ""},
	CodeInvalidState:           {409, "La operación no es válida en el estado actual", ""},
	CodeTokenInvalid:           {400, "El enlace no es válido o venció", ""},
	CodeLoginLocked:            {429, "Demasiados intentos de inicio de sesión", ""},
	CodeRateLimited:            {429, "Demasiadas solicitudes", ""},
	CodeServiceUnavailable:     {503, "El servicio no está disponible", ""},
	CodeInternal:               {500, "Error interno del servidor", ""},
}

type SuggestedAction string

const SuggestedPasswordReset SuggestedAction = "password_reset"

type FieldError struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}
type body struct {
	Type            string          `json:"type"`
	Title           string          `json:"title"`
	Status          int             `json:"status"`
	Detail          string          `json:"detail,omitempty"`
	Instance        string          `json:"instance,omitempty"`
	Code            Code            `json:"code"`
	SuggestedAction SuggestedAction `json:"suggested_action,omitempty"`
	Errors          []FieldError    `json:"errors,omitempty"`
}
type ProblemOption func(*problemOptions)
type problemOptions struct {
	action     SuggestedAction
	retryAfter time.Duration
}

func WithSuggestedAction(action SuggestedAction) ProblemOption {
	return func(o *problemOptions) { o.action = action }
}
func RetryAfter(delay time.Duration) ProblemOption {
	return func(o *problemOptions) { o.retryAfter = delay }
}

// WriteProblem never includes internal error details in a response.
func WriteProblem(w http.ResponseWriter, r *http.Request, code Code, options ...ProblemOption) {
	s, ok := problems[code]
	if !ok {
		code, s = CodeInternal, problems[CodeInternal]
	}
	opts := problemOptions{}
	for _, apply := range options {
		apply(&opts)
	}
	if opts.retryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(opts.retryAfter.Seconds()))))
	}
	writeBody(w, body{Type: "/problems/" + string(code), Title: s.title, Status: s.status, Detail: s.detail,
		Instance: RequestIDFrom(r.Context()), Code: code, SuggestedAction: opts.action})
}
func ValidationError(w http.ResponseWriter, r *http.Request, fields map[string]string) {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	list := make([]FieldError, 0, len(keys))
	for _, key := range keys {
		list = append(list, FieldError{Field: key, Code: fields[key]})
	}
	s := problems[CodeValidationFailed]
	writeBody(w, body{Type: "/problems/" + string(CodeValidationFailed), Title: s.title, Status: s.status,
		Instance: RequestIDFrom(r.Context()), Code: CodeValidationFailed, Errors: list})
}
func writeBody(w http.ResponseWriter, p body) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}
func MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	WriteProblem(w, r, CodeMethodNotAllowed)
}

// WriteDBError maps platform database errors. Endpoint-specific errors belong to their HTTP modules.
func WriteDBError(w http.ResponseWriter, r *http.Request, err error, logger *slog.Logger) {
	switch {
	case errors.Is(err, db.ErrCanceled):
		if r.Context().Err() != nil {
			markClientCanceled(r)
			return
		}
		WriteProblem(w, r, CodeServiceUnavailable)
	case errors.Is(err, db.ErrNotFound):
		WriteProblem(w, r, CodeNotFound)
	case errors.Is(err, db.ErrUnavailable):
		WriteProblem(w, r, CodeServiceUnavailable)
	case errors.Is(err, db.ErrPrivilege):
		if logger != nil {
			logger.ErrorContext(r.Context(), "rls violation", "security_event", "rls_violation", "request_id", RequestIDFrom(r.Context()))
		}
		WriteProblem(w, r, CodeInternal)
	default:
		WriteProblem(w, r, CodeInternal)
	}
}

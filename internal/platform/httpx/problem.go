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

// Code is the stable error code of a problem+json response (plan §9.1; the contract's ErrorCode
// enum is the closed list). The frontend maps each code to a Spanish text.
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

// spec is the status and generic Spanish title (and, for some codes, a longer detail) of a Code.
type spec struct {
	status        int
	title, detail string
}

// problems holds every Code's spec (ADR-009, contract v0.4.0, plan §9.1).
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

// SuggestedAction hints the frontend at a next step for some problems (e.g. offering a password
// reset when registration finds the email already taken).
type SuggestedAction string

const SuggestedPasswordReset SuggestedAction = "password_reset"

// FieldError is one entry of a ValidationError's "errors" array: which field, which code.
type FieldError struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

// body is the RFC 9457 document (ADR-009).
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

// ProblemOption configures one optional field of a WriteProblem response.
type ProblemOption func(*problemOptions)
type problemOptions struct {
	action     SuggestedAction
	retryAfter time.Duration
}

// WithSuggestedAction sets the response's suggested_action.
func WithSuggestedAction(action SuggestedAction) ProblemOption {
	return func(o *problemOptions) { o.action = action }
}

// RetryAfter sets the response's Retry-After header, rounded up to whole seconds.
func RetryAfter(delay time.Duration) ProblemOption {
	return func(o *problemOptions) { o.retryAfter = delay }
}

// WriteProblem writes an application/problem+json response for code. `instance` is the request id.
// It never includes internal error details in the response (no SQL, no stack): an unknown code
// falls back to CodeInternal rather than describing the mistake to the client.
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
// ValidationError writes a 422 problem+json with one FieldError per invalid field, sorted by field
// name so the response is deterministic.
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
// writeBody encodes p as the response: every problem+json response goes through here, so the
// content type and status line are set in exactly one place.
func writeBody(w http.ResponseWriter, p body) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}

// MethodNotAllowed writes a 405 problem+json with code method_not_allowed (contract v0.4.0). The
// caller sets the Allow header first (the API router asks chi which methods the path accepts).
func MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	WriteProblem(w, r, CodeMethodNotAllowed)
}

// WriteDBError maps a platform/db error to its HTTP response (plan §9.2); endpoint-specific errors
// belong to their own HTTP modules, not here. logger is required: INV-19 needs the ErrPrivilege
// case logged every time a bug of this severity happens, so a nil logger falls back to
// slog.Default() instead of silently dropping the log (I3 of the PR #8 review).
func WriteDBError(w http.ResponseWriter, r *http.Request, err error, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
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
		// A set-role-denied or an RLS violation (DD-34, INV-27): never a 403 or a 404, always a bug
		// to investigate. err is logged (not shown to the client) so the message (which names the
		// step and the role) tells the two apart.
		logger.ErrorContext(r.Context(), "rls violation", "security_event", "rls_violation",
			"request_id", RequestIDFrom(r.Context()), "err", err)
		WriteProblem(w, r, CodeInternal)
	default:
		WriteProblem(w, r, CodeInternal)
	}
}

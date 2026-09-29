package httpx

import (
	"encoding/json"
	"net/http"
)

// Code is the stable error code of a problem+json response (plan §9.1; the contract's
// ErrorCode enum is the closed list). The frontend maps each code to a Spanish text.
type Code string

const (
	CodeMalformedRequest Code = "malformed_request"
	CodeNotFound         Code = "not_found"
	CodeInternal         Code = "internal"
)

type spec struct {
	status int
	title  string
}

// problems holds the status and generic Spanish title of each code. Phase 0 only needs the
// codes below; T-B202 completes the taxonomy of plan §9.1.
var problems = map[Code]spec{
	CodeNotFound: {http.StatusNotFound, "Recurso no encontrado"},
	CodeInternal: {http.StatusInternalServerError, "Error interno del servidor"},
}

// body is the RFC 9457 document (ADR-009).
type body struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Instance string `json:"instance,omitempty"`
	Code     Code   `json:"code"`
}

// WriteProblem writes an application/problem+json response for code. `instance` is the request id.
// It never includes internal detail (no SQL, no stack).
func WriteProblem(w http.ResponseWriter, r *http.Request, code Code) {
	s, ok := problems[code]
	if !ok {
		s = problems[CodeInternal]
		code = CodeInternal
	}
	write(w, r, s.status, s.title, code)
}

// MethodNotAllowed writes a 405 problem+json. The contract's ErrorCode has no dedicated code for
// it, so it reuses malformed_request; see the open question reported for T-B004.
func MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	write(w, r, http.StatusMethodNotAllowed, "Método no permitido", CodeMalformedRequest)
}

func write(w http.ResponseWriter, r *http.Request, status int, title string, code Code) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body{
		Type:     "/problems/" + string(code),
		Title:    title,
		Status:   status,
		Instance: RequestIDFrom(r.Context()),
		Code:     code,
	})
}

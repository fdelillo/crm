package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
)

type ctxKey int

const (
	requestIDKey ctxKey = iota
	routeKey
)

// hstsValue is deliberately conservative: one year, no includeSubDomains and no preload (the
// hosting domain is not decided yet, and preload cannot be undone).
const hstsValue = "max-age=31536000"

// unmatchedRoute labels requests that no route handled (chi 404/405, mux redirects). The raw URL
// is never logged (DD-12).
const unmatchedRoute = "unmatched"

// RequestIDFrom returns the request id set by RequestID, or "" outside a request.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// RequestID assigns a server-generated id to every request and returns it in X-Request-Id.
// An incoming X-Request-Id is ignored on purpose: the client cannot choose what ends up in logs.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.Must(uuid.NewV7()).String()
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

// Recover turns a panic in any handler into a 500 problem+json and an ERROR log with the request id,
// so the process survives. http.ErrAbortHandler keeps its net/http meaning and is re-panicked.
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(rec)
				}
				logger.ErrorContext(r.Context(), "panic recovered",
					"request_id", RequestIDFrom(r.Context()),
					"panic", fmt.Sprint(rec),
					"stack", string(debug.Stack()))
				WriteProblem(w, r, CodeInternal)
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// routeHolder lets handlers deeper in the chain tell Logging which route label to record.
type routeHolder struct{ label string }

// SetRoute records the route label for the request log. The root mux calls it through WithRoute
// and the API wrapper with the chi pattern. It is a no-op outside Logging.
func SetRoute(r *http.Request, label string) {
	if h, ok := r.Context().Value(routeKey).(*routeHolder); ok {
		h.label = label
	}
}

// WithRoute labels every request handled by next, e.g. "ops" or "spa".
func WithRoute(label string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SetRoute(r, label)
		next.ServeHTTP(w, r)
	})
}

// Logging writes one structured line per request: request_id, method, route, status and
// duration_ms (DD-12). It logs from a defer so a panic that Recover (outside) turns into a 500 is
// still logged. Bodies, cookies and headers are never logged.
func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			holder := &routeHolder{}
			r = r.WithContext(context.WithValue(r.Context(), routeKey, holder))
			sw := &statusWriter{ResponseWriter: w}
			completed := false
			defer func() {
				status := sw.statusCode()
				if !completed && !sw.wroteHeader {
					status = http.StatusInternalServerError // panicking: Recover will answer 500
				}
				route := holder.label
				if route == "" {
					route = unmatchedRoute
				}
				level := slog.LevelInfo
				if status >= http.StatusInternalServerError {
					level = slog.LevelError
				}
				logger.Log(r.Context(), level, "request",
					"request_id", RequestIDFrom(r.Context()),
					"method", r.Method,
					"route", route,
					"status", status,
					"duration_ms", time.Since(start).Milliseconds())
			}()
			next.ServeHTTP(sw, r)
			completed = true
		})
	}
}

// SecurityHeaders sets the headers common to every response (plan §10.7). hsts is false in local
// mode so a developer's browser never records HSTS for localhost (DD-24).
func SecurityHeaders(hsts bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Referrer-Policy", "no-referrer")
			if hsts {
				h.Set("Strict-Transport-Security", hstsValue)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// statusWriter records the status code. Unwrap keeps http.ResponseController working
// (Flush, deadlines) and Flush covers code that asserts http.Flusher directly.
type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusWriter) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status, w.wroteHeader = code, true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	w.wroteHeader = true
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) Flush() { _ = http.NewResponseController(w.ResponseWriter).Flush() }

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

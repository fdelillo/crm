package httpx

import (
	"context"
	"errors"
	"expvar"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
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

var requestsTotal = expvar.NewMap("http_requests_total")

var clientCanceledTotal = expvar.NewInt("http_client_canceled_total")
var csrfRejectedTotal = expvar.NewInt("csrf_rejected_total")

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
// so the process survives. If the response already started, a 500 is impossible and writing more
// would corrupt it: it logs and re-panics with http.ErrAbortHandler so net/http cuts the connection
// and the client sees a failed read instead of a "complete" 200. A panic that already is
// http.ErrAbortHandler keeps its meaning and is re-panicked untouched.
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sw := trackWriter(w)
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
					"response_started", sw.wroteHeader,
					"stack", string(debug.Stack()))
				if sw.wroteHeader {
					panic(http.ErrAbortHandler)
				}
				WriteProblem(sw, r, CodeInternal)
			}()
			next.ServeHTTP(sw, r)
		})
	}
}

// trackWriter wraps w in a statusWriter unless it already is one.
func trackWriter(w http.ResponseWriter) *statusWriter {
	if sw, ok := w.(*statusWriter); ok {
		return sw
	}
	return &statusWriter{ResponseWriter: w}
}

// routeHolder lets handlers deeper in the chain tell Logging which route label to record.
type routeHolder struct {
	label    string
	level    *slog.Level
	canceled bool
}

func markClientCanceled(r *http.Request) {
	if h, ok := r.Context().Value(routeKey).(*routeHolder); ok {
		h.canceled = true
	}
	clientCanceledTotal.Add(1)
}

// SetLogLevel overrides the level of this request's log line (by default INFO, or ERROR for 5xx).
// For responses that are expected even though they are 5xx, such as a placeholder answering 503.
// A panic still logs at ERROR. It is a no-op outside Logging.
func SetLogLevel(r *http.Request, level slog.Level) {
	if h, ok := r.Context().Value(routeKey).(*routeHolder); ok {
		h.level = &level
	}
}

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
			sw := trackWriter(w)
			completed := false
			defer func() {
				status := sw.statusCode()
				if holder.canceled {
					status = 499
				}
				if !completed {
					// A panic is in flight: Recover answers 500 or, if the response had started, cuts
					// the connection. Either way 200 would be a lie.
					status = http.StatusInternalServerError
				}
				route := holder.label
				if route == "" {
					route = unmatchedRoute
				}
				level := slog.LevelInfo
				switch {
				case holder.canceled:
					level = slog.LevelInfo
				case !completed || (status >= http.StatusInternalServerError && holder.level == nil):
					level = slog.LevelError
				case holder.level != nil:
					level = *holder.level
				}
				requestsTotal.Add(strconv.Itoa(status), 1)
				logger.Log(r.Context(), level, "request",
					"request_id", RequestIDFrom(r.Context()),
					"method", r.Method,
					"route", route,
					"status", status,
					"event", canceledEvent(holder.canceled),
					"ip", ClientIPFrom(r.Context()).String(),
					"duration_ms", time.Since(start).Milliseconds())
			}()
			next.ServeHTTP(sw, r)
			completed = true
		})
	}
}

func canceledEvent(canceled bool) string {
	if canceled {
		return "client_canceled"
	}
	return "request"
}

// CSRFDenyHandler responds with the API's standard 403 format and records a safe security event.
func CSRFDenyHandler(logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		csrfRejectedTotal.Add(1)
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		// route is always unmatchedRoute here, and correctly so: CrossOriginProtection sits before
		// routing in the common chain (NewCommonMiddleware), so a rejected request never reaches
		// chi and SetRoute is never called for it.
		logger.WarnContext(r.Context(), "cross-origin request rejected",
			"security_event", "csrf_rejected", "ip", ClientIPFrom(r.Context()).String(),
			"route", unmatchedRoute, "request_id", RequestIDFrom(r.Context()))
		WriteProblem(w, r, CodeForbidden)
	})
}

// NoStore applies to all API responses. A resource handler may replace the value (e.g. logo).
func NoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
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

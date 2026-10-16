package httpx

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/google/uuid"
)

// T-B912 (DD-12, INV-38): Logging writes tenant_id and user_id only if SetLogPrincipal annotated
// them, in the normal, panic and 499 paths, and never as uuid.Nil.
func TestLoggingPrincipal(t *testing.T) {
	tenantA, userA := uuid.MustParse("11111111-1111-4111-8111-111111111111"), uuid.MustParse("22222222-2222-4222-8222-222222222222")
	tenantB, userB := uuid.MustParse("33333333-3333-4333-8333-333333333333"), uuid.MustParse("44444444-4444-4444-8444-444444444444")
	discard := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	cases := []struct {
		name       string
		h          http.HandlerFunc
		wantStatus int
		wantLevel  string
		wantEvent  string
		wantTenant string // "" means the key must be absent
		wantUser   string
		cancelCtx  bool
		panics     bool
	}{
		{"annotated 200 carries both ids", func(w http.ResponseWriter, r *http.Request) {
			SetLogPrincipal(r, tenantA, userA)
			w.WriteHeader(200)
		}, 200, "INFO", "request", tenantA.String(), userA.String(), false, false},
		{"not annotated has neither key", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(200)
		}, 200, "INFO", "request", "", "", false, false},
		{"nil tenant writes neither key", func(w http.ResponseWriter, r *http.Request) {
			SetLogPrincipal(r, uuid.Nil, userA)
			w.WriteHeader(200)
		}, 200, "INFO", "request", "", "", false, false},
		{"nil user writes neither key", func(w http.ResponseWriter, r *http.Request) {
			SetLogPrincipal(r, tenantA, uuid.Nil)
			w.WriteHeader(200)
		}, 200, "INFO", "request", "", "", false, false},
		{"second call wins", func(w http.ResponseWriter, r *http.Request) {
			SetLogPrincipal(r, tenantA, userA)
			SetLogPrincipal(r, tenantB, userB)
			w.WriteHeader(200)
		}, 200, "INFO", "request", tenantB.String(), userB.String(), false, false},
		{"panic keeps both ids", func(_ http.ResponseWriter, r *http.Request) {
			SetLogPrincipal(r, tenantA, userA)
			panic("boom")
		}, 500, "ERROR", "request", tenantA.String(), userA.String(), false, true},
		{"client canceled keeps both ids", func(w http.ResponseWriter, r *http.Request) {
			SetLogPrincipal(r, tenantA, userA)
			WriteDBError(w, r, db.ErrCanceled, discard)
		}, 499, "INFO", "client_canceled", tenantA.String(), userA.String(), true, false},
		{"503 under WithStatusLogLevel keeps both ids", WithStatusLogLevel(503, slog.LevelWarn, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			SetLogPrincipal(r, tenantA, userA)
			w.WriteHeader(503)
		})).ServeHTTP, 503, "WARN", "request", tenantA.String(), userA.String(), false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))
			h := Recover(logger)(Logging(logger)(tc.h))
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			if tc.cancelCtx {
				ctx, cancel := context.WithCancel(req.Context())
				cancel()
				req = req.WithContext(ctx)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if tc.panics && rec.Code != 500 {
				t.Errorf("response status = %d, want 500", rec.Code)
			}
			line := requestLine(t, &buf)
			if line["status"] != float64(tc.wantStatus) || line["level"] != tc.wantLevel || line["event"] != tc.wantEvent {
				t.Errorf("status=%v level=%v event=%v, want %d %s %s", line["status"], line["level"], line["event"], tc.wantStatus, tc.wantLevel, tc.wantEvent)
			}
			for key, want := range map[string]string{"tenant_id": tc.wantTenant, "user_id": tc.wantUser} {
				got, present := line[key]
				if want == "" && present {
					t.Errorf("%s = %v, want the key absent", key, got)
				}
				if want != "" && got != want {
					t.Errorf("%s = %v, want %s", key, got, want)
				}
			}
		})
	}
}

// T-B912 (INV-38): outside Logging SetLogPrincipal is a no-op: same response, no panic.
func TestSetLogPrincipalWithoutLogging(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SetLogPrincipal(r, uuid.New(), uuid.New())
		w.WriteHeader(202)
		_, _ = w.Write([]byte("ok"))
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != 202 || rec.Body.String() != "ok" {
		t.Errorf("status %d body %q, want 202 ok", rec.Code, rec.Body.String())
	}
}

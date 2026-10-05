package app_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/ratelimit"
	"github.com/fdelillo/crm/internal/testsupport/contract"
	"golang.org/x/time/rate"
)

func TestInvitationRoutesShareIPQuota(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := app.NewAPIRouter()
	limiter := ratelimit.NewLimiter(rate.Every(3*time.Minute), 20, clock.Real{}, time.Hour)
	app.RegisterUserRoutes(r, nil, nil, limiter, logger)
	root := app.NewRootHandler(app.RootDeps{API: r, Liveness: app.LivenessHandler(), Readiness: app.ReadinessPlaceholder(), SPA: app.SPAUnavailableHandler()}, app.NewCommonMiddleware(logger, true, nil))
	for i := range 22 {
		path := "/api/v1/auth/invitations/preview"
		if i%2 == 1 {
			path = "/api/v1/auth/invitations/accept"
		}
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader("{"))
		req.RemoteAddr = "192.0.2.1:1234"
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		root.ServeHTTP(rec, req)
		contract.Default(t).RequireRecorded(t, req, rec)
		want := 400
		if i >= 20 {
			want = 429
		}
		if rec.Code != want {
			t.Fatalf("request %d %s: got %d, want %d", i, path, rec.Code, want)
		}
		if want == 429 && rec.Header().Get("Retry-After") == "" {
			t.Fatal("missing Retry-After")
		}
	}
}

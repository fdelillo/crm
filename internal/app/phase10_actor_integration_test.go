//go:build integration

package app_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/industrytemplate"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/outbox"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/tenant"
	"github.com/fdelillo/crm/internal/testsupport/contract"
	"github.com/fdelillo/crm/internal/testsupport/fixture/e2e"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
)

// gatedRunner holds the next InTenantTx of the services until the test releases it. It models a
// request that already passed Authenticate and RequirePermission and is retained before the
// service transaction (T-B1001 H2: reactivate has no body to retain).
type gatedRunner struct {
	db.TxRunner
	armed            atomic.Bool
	entered, release chan struct{}
}

func (g *gatedRunner) arm() {
	g.entered, g.release = make(chan struct{}), make(chan struct{})
	g.armed.Store(true)
}
func (g *gatedRunner) InTenantTx(ctx context.Context, id uuid.UUID, fn func(context.Context, db.Tx) error) error {
	if g.armed.CompareAndSwap(true, false) {
		close(g.entered)
		<-g.release
	}
	return g.TxRunner.InTenantTx(ctx, id, fn)
}

// firstReadGate is a request body whose first Read announces itself and waits for the test.
type firstReadGate struct {
	data             io.Reader
	entered, release chan struct{}
	once             sync.Once
}

func (b *firstReadGate) Read(p []byte) (int, error) {
	b.once.Do(func() {
		close(b.entered)
		<-b.release
	})
	return b.data.Read(p)
}
func (b *firstReadGate) Close() error { return nil }

type phase10Env struct {
	t      *testing.T
	root   http.Handler
	users  *identity.Service
	runner *gatedRunner
	a, b   authz.Principal
	aRaw   string
	bRaw   string
}

// newPhase10Env builds the real router of BuildAPIRouter over services whose runner can be gated,
// and a company with two active administrators A and B, each with a real session.
func newPhase10Env(t *testing.T) *phase10Env {
	t.Helper()
	ctx := context.Background()
	c := clock.Real{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	runner := &gatedRunner{TxRunner: db.NewTxRunner(pgtest.AppPool(t), db.WithLogger(logger))}
	h := password.NewHasher(2)
	recorder := audit.NewRecorder()
	users := identity.NewService(runner, outbox.NewEnqueuer(c), c, 24*time.Hour, 7*24*time.Hour,
		identity.WithAuthentication(h, recorder, []byte("0123456789abcdef0123456789abcdef"), logger))
	companies := tenant.NewService(runner, users, industrytemplate.NoopSeeder{}, h, recorder, logger)
	suffix := uuid.NewString()
	reg, err := companies.Register(ctx, tenant.Signup{Name: "Admin A", Email: "admin-a-" + suffix + "@example.test", Password: e2e.Password,
		CompanyName: "Company " + suffix, BaseCurrency: "ARS", IndustryTemplateCode: "generic", Timezone: "America/Argentina/Buenos_Aires"}, identity.RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	env := &phase10Env{t: t, users: users, runner: runner, a: reg.Session.Principal, aRaw: reg.Session.RawToken}
	settleOutbox(t, runner, env.a.TenantID)
	invited, _, err := users.Invite(ctx, env.a, "admin-b-"+suffix+"@example.test", authz.RoleAdmin, identity.RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	var token string
	if err := runner.InTenantTx(ctx, env.a.TenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT payload->>'token' FROM app.outbox_messages WHERE tenant_id=$1 AND recipient=$2 AND template='invitation' ORDER BY created_at DESC, id DESC LIMIT 1`, env.a.TenantID, invited.Email).Scan(&token)
	}); err != nil {
		t.Fatal(err)
	}
	accepted, err := users.AcceptInvitation(ctx, token, "Admin B", e2e.Password, identity.RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	env.b, env.bRaw = accepted.Principal, accepted.RawToken
	api := app.BuildAPIRouter(users, companies, c, logger)
	env.root = app.NewRootHandler(app.RootDeps{API: api, Liveness: app.LivenessHandler(), Readiness: app.ReadinessPlaceholder(), SPA: app.SPAUnavailableHandler()}, app.NewCommonMiddleware(logger, true, nil))
	return env
}

// settleOutbox marks the company's pending messages as sent when the test ends. The Phase 2 test
// drains the shared outbox into a Mailpit it expects to hold exactly one message, and this file
// runs before it (phase10 < phase2): a test of Phase 10 must not leave pending messages behind.
func settleOutbox(t *testing.T, runner db.TxRunner, tenantID uuid.UUID) {
	t.Helper()
	t.Cleanup(func() {
		_ = runner.InTenantTx(context.Background(), tenantID, func(ctx context.Context, tx db.Tx) error {
			_, err := tx.Exec(ctx, `UPDATE app.outbox_messages SET status='sent', payload=NULL, sent_at=now() WHERE tenant_id=$1 AND status<>'sent'`, tenantID)
			return err
		})
	})
}

func (e *phase10Env) request(raw, method, path string, body io.Reader) (*httptest.ResponseRecorder, *http.Request) {
	e.t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), method, path, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.AddCookie(&http.Cookie{Name: identity.SessionCookieName, Value: raw})
	return httptest.NewRecorder(), req
}

func (e *phase10Env) do(raw, method, path, body string, want int) *httptest.ResponseRecorder {
	e.t.Helper()
	var reader io.Reader
	if body != "" {
		reader = bytes.NewReader([]byte(body))
	}
	rec, req := e.request(raw, method, path, reader)
	e.root.ServeHTTP(rec, req)
	if rec.Code != want {
		e.t.Fatalf("%s %s: status=%d want %d body=%s", method, path, rec.Code, want, rec.Body.String())
	}
	if body != "" {
		req.Body = io.NopCloser(bytes.NewReader([]byte(body)))
	}
	contract.Default(e.t).RequireRecorded(e.t, req, rec)
	return rec
}

func (e *phase10Env) userState(id uuid.UUID) (role, status string) {
	e.t.Helper()
	if err := e.runner.TxRunner.InTenantTx(context.Background(), e.a.TenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT role, status FROM app.users WHERE tenant_id=$1 AND id=$2`, e.a.TenantID, id).Scan(&role, &status)
	}); err != nil {
		e.t.Fatal(err)
	}
	return role, status
}

// TestActorRevalidationHTTP covers T-B1001 H1 and H2: a retained request of an administrator
// that was demoted or disabled meanwhile must not restore its authority.
func TestActorRevalidationHTTP(t *testing.T) {
	t.Run("H1 demoted administrator cannot promote itself with a retained body", func(t *testing.T) {
		e := newPhase10Env(t)
		gate := &firstReadGate{data: bytes.NewReader([]byte(`{"role":"admin"}`)), entered: make(chan struct{}), release: make(chan struct{})}
		rec, req := e.request(e.aRaw, http.MethodPut, "/api/v1/users/"+e.a.UserID.String()+"/role", gate)
		done := make(chan struct{})
		go func() { defer close(done); e.root.ServeHTTP(rec, req) }()
		select {
		case <-gate.entered:
		case <-time.After(20 * time.Second):
			t.Fatal("request of A never reached its body")
		}
		e.do(e.bRaw, http.MethodPut, "/api/v1/users/"+e.a.UserID.String()+"/role", `{"role":"operator"}`, http.StatusOK)
		close(gate.release)
		<-done
		if rec.Code != http.StatusForbidden {
			t.Fatalf("retained request: status=%d body=%s", rec.Code, rec.Body.String())
		}
		req.Body = io.NopCloser(bytes.NewReader([]byte(`{"role":"admin"}`)))
		contract.Default(t).RequireRecorded(t, req, rec)
		if role, _ := e.userState(e.a.UserID); role != "operator" {
			t.Fatalf("A is %s, want operator", role)
		}
		e.do(e.aRaw, http.MethodGet, "/api/v1/users", "", http.StatusForbidden)
	})
	t.Run("H2 disabled administrator cannot reactivate itself with a retained request", func(t *testing.T) {
		e := newPhase10Env(t)
		e.runner.arm()
		rec, req := e.request(e.aRaw, http.MethodPost, "/api/v1/users/"+e.a.UserID.String()+"/reactivate", nil)
		done := make(chan struct{})
		go func() { defer close(done); e.root.ServeHTTP(rec, req) }()
		select {
		case <-e.runner.entered:
		case <-time.After(20 * time.Second):
			t.Fatal("request of A never reached the service")
		}
		e.do(e.bRaw, http.MethodPost, "/api/v1/users/"+e.a.UserID.String()+"/deactivate", "", http.StatusOK)
		close(e.runner.release)
		<-done
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("retained request: status=%d body=%s", rec.Code, rec.Body.String())
		}
		contract.Default(t).RequireRecorded(t, req, rec)
		cleared := false
		for _, c := range rec.Result().Cookies() {
			cleared = cleared || (c.Name == identity.SessionCookieName && c.MaxAge < 0)
		}
		if !cleared {
			t.Fatal("session cookie was not cleared")
		}
		if _, status := e.userState(e.a.UserID); status != "disabled" {
			t.Fatalf("A is %s, want disabled", status)
		}
	})
}

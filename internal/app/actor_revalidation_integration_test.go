//go:build integration

package app_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
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

// twoAdminCompany is a company with two active administrators, each with a real session.
type twoAdminCompany struct {
	a, b       authz.Principal
	aRaw, bRaw string
}

type apiEnv struct {
	t         *testing.T
	root      http.Handler
	users     *identity.Service
	companies *tenant.Service
	runner    *gatedRunner
	clock     *usersHTTPClock // clock of the rate limiters; the services use the real one
	twoAdminCompany
}

// newAPIEnv builds the real router of BuildAPIRouter over services whose runner can be gated,
// and a first company with two active administrators A and B.
func newAPIEnv(t *testing.T) *apiEnv {
	t.Helper()
	return newAPIEnvWithLogger(t, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func newAPIEnvWithLogger(t *testing.T, logger *slog.Logger) *apiEnv {
	t.Helper()
	c := clock.Real{}
	runner := &gatedRunner{TxRunner: db.NewTxRunner(pgtest.AppPool(t), db.WithLogger(logger))}
	h := password.NewHasher(2)
	recorder := audit.NewRecorder()
	users := identity.NewService(runner, outbox.NewEnqueuer(c), c, 24*time.Hour, 7*24*time.Hour,
		identity.WithAuthentication(h, recorder, []byte("0123456789abcdef0123456789abcdef"), logger))
	companies := tenant.NewService(runner, users, industrytemplate.NoopSeeder{}, h, recorder, logger)
	limiterClock := &usersHTTPClock{now: time.Now().UTC()}
	env := &apiEnv{t: t, users: users, companies: companies, runner: runner, clock: limiterClock}
	env.twoAdminCompany = env.newCompany()
	api := app.BuildAPIRouter(users, companies, limiterClock, logger)
	env.root = app.NewRootHandler(app.RootDeps{API: api, Liveness: app.LivenessHandler(), Readiness: app.ReadinessPlaceholder(), SPA: app.SPAUnavailableHandler()}, app.NewCommonMiddleware(logger, true, nil))
	return env
}

func (e *apiEnv) token(tenantID uuid.UUID, recipient, template string) string {
	e.t.Helper()
	var token string
	if err := e.runner.TxRunner.InTenantTx(context.Background(), tenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT payload->>'token' FROM app.outbox_messages WHERE tenant_id=$1 AND recipient=$2 AND template=$3 ORDER BY created_at DESC, id DESC LIMIT 1`, tenantID, recipient, template).Scan(&token)
	}); err != nil {
		e.t.Fatal(err)
	}
	return token
}

// newCompany registers a company, confirms its founder's email with the token of the outbox (DD-46:
// an unverified administrator cannot invite) and adds administrator B. The founder is administrator A.
func (e *apiEnv) newCompany() twoAdminCompany {
	e.t.Helper()
	c, email := e.newUnverifiedCompany()
	token := e.token(c.a.TenantID, email, "email_verification")
	if err := e.users.ConfirmEmailVerification(context.Background(), token, identity.RequestMeta{}); err != nil {
		e.t.Fatal(err)
	}
	member := e.member(c.a, "b-"+email, authz.RoleAdmin)
	c.b, c.bRaw = member.Principal, member.RawToken
	return c
}

// newUnverifiedCompany is what a signup leaves: one active administrator whose email is not verified.
func (e *apiEnv) newUnverifiedCompany() (twoAdminCompany, string) {
	e.t.Helper()
	suffix := uuid.NewString()
	email := "admin-a-" + suffix + "@example.test"
	reg, err := e.companies.Register(context.Background(), tenant.Signup{Name: "Admin A", Email: email, Password: e2e.Password,
		CompanyName: "Company " + suffix, BaseCurrency: "ARS", IndustryTemplateCode: "generic", Timezone: "America/Argentina/Buenos_Aires"}, identity.RequestMeta{})
	if err != nil {
		e.t.Fatal(err)
	}
	settleOutbox(e.t, e.runner, reg.Session.Principal.TenantID)
	return twoAdminCompany{a: reg.Session.Principal, aRaw: reg.Session.RawToken}, email
}

// unverify clears email_verified_at of users of a company, to reach the state of a fresh signup.
func (e *apiEnv) unverify(tenantID uuid.UUID, ids ...uuid.UUID) {
	e.t.Helper()
	for _, id := range ids {
		if err := e.runner.TxRunner.InTenantTx(context.Background(), tenantID, func(ctx context.Context, tx db.Tx) error {
			_, err := tx.Exec(ctx, `UPDATE app.users SET email_verified_at=NULL WHERE tenant_id=$1 AND id=$2`, tenantID, id)
			return err
		}); err != nil {
			e.t.Fatal(err)
		}
	}
}

// member invites and accepts a user of the company, through the services (no rate limit involved).
func (e *apiEnv) member(admin authz.Principal, email string, role authz.Role) identity.SessionResult {
	e.t.Helper()
	ctx := context.Background()
	if _, _, err := e.users.Invite(ctx, admin, email, role, identity.RequestMeta{}); err != nil {
		e.t.Fatal(err)
	}
	session, err := e.users.AcceptInvitation(ctx, e.token(admin.TenantID, email, "invitation"), "Member", e2e.Password, identity.RequestMeta{})
	if err != nil {
		e.t.Fatal(err)
	}
	return session
}

// settleOutbox marks the company's pending messages as sent when the test ends. No test assumes an
// empty outbox, so this is a courtesy: it keeps the shared outbox small for the tests that run a
// Dispatcher over it.
func settleOutbox(t *testing.T, runner db.TxRunner, tenantID uuid.UUID) {
	t.Helper()
	t.Cleanup(func() {
		err := runner.InTenantTx(context.Background(), tenantID, func(ctx context.Context, tx db.Tx) error {
			_, err := tx.Exec(ctx, `UPDATE app.outbox_messages SET status='sent', payload=NULL, sent_at=now() WHERE tenant_id=$1 AND status<>'sent'`, tenantID)
			return err
		})
		if err != nil {
			t.Errorf("settling the outbox of %s: %v", tenantID, err)
		}
	})
}

func (e *apiEnv) request(raw, method, path string, body io.Reader) (*httptest.ResponseRecorder, *http.Request) {
	e.t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), method, path, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if raw != "" {
		req.AddCookie(&http.Cookie{Name: identity.SessionCookieName, Value: raw})
	}
	return httptest.NewRecorder(), req
}

func (e *apiEnv) do(raw, method, path, body string, want int) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.doFrom(raw, "", method, path, body, want)
}

// doFrom is do with the client address of the request (documentation addresses, DD-32).
func (e *apiEnv) doFrom(raw, ip, method, path, body string, want int) *httptest.ResponseRecorder {
	e.t.Helper()
	var reader io.Reader
	if body != "" {
		reader = bytes.NewReader([]byte(body))
	}
	rec, req := e.request(raw, method, path, reader)
	if ip != "" {
		req.RemoteAddr = net.JoinHostPort(ip, "4242")
	}
	e.root.ServeHTTP(rec, req)
	if rec.Code != want {
		e.t.Fatalf("%s %s from %s: status=%d want %d body=%s", method, path, ip, rec.Code, want, rec.Body.String())
	}
	if body != "" {
		req.Body = io.NopCloser(bytes.NewReader([]byte(body)))
	}
	contract.Default(e.t).RequireRecorded(e.t, req, rec)
	return rec
}

func (e *apiEnv) userState(id uuid.UUID) (role, status string) {
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
		e := newAPIEnv(t)
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
		e := newAPIEnv(t)
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

//go:build integration

package reprovision_test

import (
	"bytes"
	"context"
	"errors"
	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/industrytemplate"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/outbox"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/tenant"
	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/fixture/e2e"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

type reprovisionRunner struct {
	db.TxRunner
	calls atomic.Int64
	hook  func(uuid.UUID) error
}

func (r *reprovisionRunner) WithSessionLock(ctx context.Context, key int64, fn func(context.Context) error) (bool, error) {
	return r.TxRunner.(db.SessionLocker).WithSessionLock(ctx, key, fn)
}
func (r *reprovisionRunner) InSystemTx(ctx context.Context, role db.SystemRole, fn func(context.Context, db.Tx) error) error {
	if role == db.RoleSignup {
		r.calls.Add(1)
	}
	return r.TxRunner.InSystemTx(ctx, role, func(ctx context.Context, tx db.Tx) error { return fn(ctx, reprovisionTx{Tx: tx, runner: r}) })
}

type reprovisionTx struct {
	db.Tx
	runner *reprovisionRunner
}

func (tx reprovisionTx) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	if strings.Contains(query, "provisioning.provision_tenant_role") && tx.runner.hook != nil {
		if err := tx.runner.hook(args[0].(uuid.UUID)); err != nil {
			return errorRow{err}
		}
	}
	return tx.Tx.QueryRow(ctx, query, args...)
}

type errorRow struct{ err error }

func (r errorRow) Scan(...any) error { return r.err }
func service(runner db.TxRunner) *tenant.Service {
	c := clock.Real{}
	users := identity.NewService(runner, outbox.NewEnqueuer(c), c, 24*time.Hour, 7*24*time.Hour)
	return tenant.NewService(runner, users, industrytemplate.NoopSeeder{}, password.NewHasher(2), audit.NewRecorder(), slog.New(slog.NewJSONHandler(io.Discard, nil)))
}
func TestPhase9Reprovision(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool := pgtest.AppPool(t)
	base := db.NewTxRunner(pool)
	// Blockers/probes/session locks use dedicated connections. At most two runtime transactions coexist.
	probe, err := pgx.ConnectConfig(ctx, pgtest.SuperuserPool(t).Config().ConnConfig.Copy())
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Close(context.Background())
	exists := func(id uuid.UUID) bool {
		t.Helper()
		var yes bool
		if err := probe.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1)", db.TenantRoleName(id)).Scan(&yes); err != nil {
			t.Fatal(err)
		}
		return yes
	}
	drop := func(id uuid.UUID) {
		t.Helper()
		if _, err := probe.Exec(ctx, "DROP ROLE "+pgx.Identifier{db.TenantRoleName(id)}.Sanitize()); err != nil {
			t.Fatal(err)
		}
	}
	var ids []uuid.UUID
	for i := 0; i < 3; i++ {
		ids = append(ids, fixture.NewCompany(t, pool).ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	for _, id := range ids {
		drop(id)
	}
	runner := &reprovisionRunner{TxRunner: base}
	runner.hook = func(id uuid.UUID) error {
		if id == ids[1] {
			if !exists(ids[0]) {
				t.Error("previous company not committed before next")
			}
			return errors.New("forced middle failure")
		}
		return nil
	}
	report, err := service(runner).Reprovision(ctx)
	if err == nil || report.Tenants != 3 || len(report.Failed) != 1 || report.Failed[0] != ids[1].String() {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	if runner.calls.Load() != 3 {
		t.Fatalf("signup transactions=%d want=3", runner.calls.Load())
	}
	if !exists(ids[0]) || exists(ids[1]) || !exists(ids[2]) {
		t.Fatal("middle failure did not preserve independent commits")
	}
	runner.hook = nil
	if report, err = service(runner).Reprovision(ctx); err != nil || len(report.Failed) != 0 {
		t.Fatalf("restore=%+v %v", report, err)
	}
	if report, err = service(runner).Reprovision(ctx); err != nil || report.Tenants != 3 {
		t.Fatalf("idempotent=%+v %v", report, err)
	}
	// Two real service-created companies and the actual HTTP router.
	f := e2e.New(t, pool)
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	runtime := db.NewTxRunner(pool, db.WithLogger(logger))
	users := identity.NewService(runtime, outbox.NewEnqueuer(f.Clock), f.Clock, 24*time.Hour, 7*24*time.Hour)
	api := app.BuildAPIRouter(users, f.Companies, f.Clock, logger)
	request := func(company e2e.Company, status int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		req.AddCookie(company.Users["admin"].Cookie)
		rec := httptest.NewRecorder()
		api.ServeHTTP(rec, req)
		if rec.Code != status {
			t.Errorf("company=%s status=%d want=%d body=%s", company.ID, rec.Code, status, rec.Body)
		}
	}
	drop(f.A.ID)
	request(f.A, 500)
	request(f.B, 200)
	if !strings.Contains(logs.String(), db.TenantRoleName(f.A.ID)) {
		t.Error("missing-role failure did not log role")
	}
	if _, err = f.Companies.Reprovision(ctx); err != nil {
		t.Fatal(err)
	}
	request(f.A, 200)
	request(f.B, 200)
	// Hold the first run inside a transaction after it owns the session advisory lock.
	entered := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan error, 1)
	var once atomic.Bool
	locked := &reprovisionRunner{TxRunner: base, hook: func(uuid.UUID) error {
		if once.CompareAndSwap(false, true) {
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}}
	go func() { _, err := service(locked).Reprovision(ctx); firstDone <- err }()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	start := time.Now()
	_, secondErr := service(base).Reprovision(ctx)
	elapsed := time.Since(start)
	close(release)
	if secondErr == nil || !strings.Contains(secondErr.Error(), "otra reprovisión en curso") || elapsed >= 2*time.Second {
		t.Errorf("concurrent run err=%v duration=%s", secondErr, elapsed)
	}
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	// Fifty companies, with a registration launched between the first and second provision calls.
	for i := 0; i < 50; i++ {
		fixture.NewCompany(t, pool)
	}
	entered = make(chan struct{})
	release = make(chan struct{})
	firstDone = make(chan error, 1)
	var calls atomic.Int64
	during := &reprovisionRunner{TxRunner: base, hook: func(uuid.UUID) error {
		if calls.Add(1) == 2 {
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}}
	go func() { _, err := service(during).Reprovision(ctx); firstDone <- err }()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	registration, signupErr := service(base).Register(ctx, tenant.Signup{Name: "Ana", Email: uuid.NewString() + "@phase9.test", Password: "a safe password 123", CompanyName: "Concurrent", BaseCurrency: "ARS", IndustryTemplateCode: "generic"}, identity.RequestMeta{})
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if signupErr != nil || registration.Tenant.ID == uuid.Nil {
		t.Fatalf("registration during 50-company reprovision: %v", signupErr)
	}
	t.Logf("reprovision: per-company commits; middle failure; idempotent; session lock; registration during 50 companies; pool runtime peak <=2")
}

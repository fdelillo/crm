//go:build integration

package app_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fdelillo/crm/db/migrations"
	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/platform/config"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/testsupport/fixture/e2e"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
)

func TestPhase9Readiness(t *testing.T) {
	ctx := context.Background()
	runner := db.NewTxRunner(pgtest.AppPool(t))
	expected, err := migrations.ExpectedVersion(migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	check := func(runner db.TxRunner, expected int64, status int, reason string) {
		t.Helper()
		logs.Reset()
		rec := httptest.NewRecorder()
		requestCtx, stop := context.WithTimeout(ctx, 4*time.Second)
		defer stop()
		app.ReadinessHandler(runner, expected, logger).ServeHTTP(rec, httptest.NewRequest("GET", "/readyz", nil).WithContext(requestCtx))
		want := `{"status":"unavailable"}`
		if status == 200 {
			want = `{"status":"ok"}`
		}
		if rec.Code != status || rec.Body.String() != want || rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("readiness %d %s %v", rec.Code, rec.Body, rec.Header())
		}
		if reason != "" && !strings.Contains(logs.String(), `"reason":"`+reason+`"`) {
			t.Errorf("reason=%s log=%s", reason, logs.String())
		}
		if strings.Contains(rec.Body.String(), "version") {
			t.Error("response leaks version")
		}
	}
	check(runner, expected, 200, "")
	for _, v := range []int64{expected + 1, expected - 1} {
		check(runner, v, 503, "schema_mismatch")
		if !strings.Contains(logs.String(), `"level":"WARN"`) || !strings.Contains(logs.String(), `"db_version":`) || !strings.Contains(logs.String(), `"expected_version":`) {
			t.Error(logs.String())
		}
	}
	migrationDB, err := sql.Open("pgx", pgtest.OwnerURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer migrationDB.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, migrationDB, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	version, err := provider.GetDBVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = runner.InSystemTx(ctx, db.RoleAuth, func(ctx context.Context, tx db.Tx) error {
		v, ok, err := db.SchemaVersion(ctx, tx)
		if err == nil && (!ok || v != version || v != expected) {
			t.Errorf("schema=%d/%v goose=%d expected=%d", v, ok, version, expected)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	unavailable, err := pgxpool.New(ctx, "postgres://crm_app:unused@"+addr+"/crm?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	defer unavailable.Close()
	check(db.NewTxRunner(unavailable), expected, 503, "unavailable")
	// One pool connection for readiness; the blocker is dedicated (never occupies the runtime pool).
	blocker, err := pgx.Connect(ctx, pgtest.OwnerURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Close(ctx)
	tx, err := blocker.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "LOCK TABLE public.goose_db_version IN ACCESS EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	check(runner, expected, 503, "timeout")
	elapsed := time.Since(started)
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if elapsed >= 3*time.Second {
		t.Errorf("readiness deadline elapsed=%s", elapsed)
	}
	// Exhaust the entire pool: the same deadline must include acquiring a connection.
	var held []*pgxpool.Conn
	for i := int32(0); i < pgtest.AppPool(t).Config().MaxConns; i++ {
		conn, err := pgtest.AppPool(t).Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		held = append(held, conn)
	}
	started = time.Now()
	check(runner, expected, 503, "timeout")
	elapsed = time.Since(started)
	for _, conn := range held {
		conn.Release()
	}
	if elapsed >= 3*time.Second {
		t.Errorf("pool deadline=%s", elapsed)
	}
	owner := pgtest.OwnerPool(t)
	if _, err = owner.Exec(ctx, "REVOKE SELECT (version_id) ON public.goose_db_version FROM crm_auth"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, err := owner.Exec(context.Background(), "GRANT SELECT (version_id) ON public.goose_db_version TO crm_auth")
		if err != nil {
			t.Error(err)
		}
	})
	check(runner, expected, 503, "privilege")
	if !strings.Contains(logs.String(), `"level":"ERROR"`) || !strings.Contains(logs.String(), `"security_event":"rls_violation"`) {
		t.Error(logs.String())
	}
	if _, err = owner.Exec(ctx, "GRANT SELECT (version_id) ON public.goose_db_version TO crm_auth"); err != nil {
		t.Fatal(err)
	}
	// A superuser session may enter owner then auth without altering role membership. DELETE runs as owner.
	super, err := pgx.ConnectConfig(ctx, pgtest.SuperuserPool(t).Config().ConnConfig.Copy())
	if err != nil {
		t.Fatal(err)
	}
	defer super.Close(ctx)
	empty, err := super.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer empty.Rollback(ctx)
	for _, query := range []string{"SET LOCAL ROLE crm_owner", "DELETE FROM public.goose_db_version", "SET LOCAL ROLE crm_auth"} {
		if _, err = empty.Exec(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	v, ok, err := db.SchemaVersion(ctx, empty)
	if err != nil || ok || v != 0 {
		t.Fatalf("empty version=%d/%v err=%v", v, ok, err)
	}
	if err = empty.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	check(&emptyVersionRunner{TxRunner: runner}, expected, 503, "schema_unknown")
	f := e2e.New(t, pgtest.AppPool(t))
	for _, role := range []string{"", "crm_worker", "crm_signup", db.TenantRoleName(f.A.ID)} {
		tx, err := pgtest.AppPool(t).Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if role != "" {
			if _, err = tx.Exec(ctx, "SET LOCAL ROLE "+pgx.Identifier{role}.Sanitize()); err != nil {
				t.Fatal(err)
			}
		}
		var v int64
		err = tx.QueryRow(ctx, "SELECT max(version_id) FROM public.goose_db_version").Scan(&v)
		if !errors.Is(db.MapError(err), db.ErrPrivilege) {
			t.Errorf("%s: %v", role, err)
		}
		_ = tx.Rollback(ctx)
	}
	logs.Reset()
	canceled, cancel := context.WithCancel(ctx)
	defer cancel()
	entered := make(chan struct{})
	continueCheck := make(chan struct{})
	cancellationRunner := &readinessCancellationRunner{TxRunner: runner, entered: entered, continueCheck: continueCheck}
	go func() { <-entered; cancel(); close(continueCheck) }()
	rec := httptest.NewRecorder()
	root := app.NewRootHandler(app.RootDeps{API: http.NotFoundHandler(), Liveness: app.LivenessHandler(), Readiness: app.ReadinessHandler(cancellationRunner, expected, logger), SPA: http.NotFoundHandler()}, app.NewCommonMiddleware(logger, true, nil))
	root.ServeHTTP(rec, httptest.NewRequest("GET", "/readyz", nil).WithContext(canceled))
	if rec.Body.Len() != 0 || strings.Contains(logs.String(), `"level":"WARN"`) || strings.Contains(logs.String(), `"level":"ERROR"`) || !strings.Contains(logs.String(), `"status":499`) {
		t.Errorf("client cancellation body=%s log=%s", rec.Body, logs.String())
	}
}

type emptyVersionRunner struct{ db.TxRunner }

func (r *emptyVersionRunner) InSystemTx(ctx context.Context, role db.SystemRole, fn func(context.Context, db.Tx) error) error {
	return r.TxRunner.InSystemTx(ctx, role, func(ctx context.Context, tx db.Tx) error { return fn(ctx, emptyVersionTx{Tx: tx}) })
}

type emptyVersionTx struct{ db.Tx }

func (tx emptyVersionTx) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	if query == "SELECT max(version_id) FROM public.goose_db_version" {
		query = "SELECT NULL::bigint"
	}
	return tx.Tx.QueryRow(ctx, query, args...)
}

func TestPhase9MetricNames(t *testing.T) {
	rec := httptest.NewRecorder()
	app.NewMetricsServer(config.Config{}).Handler.ServeHTTP(rec, httptest.NewRequest("GET", "/debug/vars", nil))
	var values map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &values); err != nil {
		t.Fatal(err)
	}
	for _, name := range strings.Fields("http_requests_total http_client_canceled_total login_failed_total login_locked_total signup_email_exists_total signup_lock_timeout_total set_role_retry_total csrf_rejected_total outbox_pending outbox_oldest_pending_seconds outbox_failed_total outbox_delivery_errors_total outbox_deferred_total db_pool_acquire_wait_ms tenants_total tenant_roles_total tenant_roles_missing") {
		if _, ok := values[name]; !ok {
			t.Errorf("missing metric %s", name)
		}
	}
}

// The client cancels only after readiness has acquired its transaction.
type readinessCancellationRunner struct {
	db.TxRunner
	entered       chan struct{}
	continueCheck chan struct{}
}

func (r *readinessCancellationRunner) InSystemTx(ctx context.Context, role db.SystemRole, fn func(context.Context, db.Tx) error) error {
	return r.TxRunner.InSystemTx(ctx, role, func(ctx context.Context, tx db.Tx) error { close(r.entered); <-r.continueCheck; return fn(ctx, tx) })
}

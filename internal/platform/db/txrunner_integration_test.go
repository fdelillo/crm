//go:build integration

package db_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

func currentUser(t *testing.T, ctx context.Context, tx db.Tx) string {
	t.Helper()
	var u string
	if err := tx.QueryRow(ctx, `SELECT current_user`).Scan(&u); err != nil {
		t.Fatalf("SELECT current_user: %v", err)
	}
	return u
}

// singleConn returns a runner over a pool of ONE connection, and that pool: whatever a transaction does
// to its connection, the next query on the pool runs on the same one.
func singleConn(t *testing.T) (db.TxRunner, *pgxpool.Pool) {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), pgtest.AppURL(t)+"&pool_max_conns=1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return db.NewTxRunner(pool), pool
}

// assertBackToApp checks the connection is crm_app again: SET LOCAL is gone with the transaction, however it ended.
func assertBackToApp(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	var user string
	if err := pool.QueryRow(context.Background(), `SELECT current_user`).Scan(&user); err != nil || user != "crm_app" {
		t.Errorf("current_user on the same connection afterwards = %q (err %v), want crm_app", user, err)
	}
}

func newTenant(t *testing.T) (uuid.UUID, string) {
	t.Helper()
	id := uuid.New()
	return id, fixture.ProvisionRole(t, pgtest.AppPool(t), id)
}

// T-B103: inside InTenantTx every statement runs as the company role; afterwards the connection is
// crm_app again, with nothing left over (SET LOCAL, INV-02).
func TestTxRunner_InTenantTxRunsAsTheCompanyRole(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	id, role := newTenant(t)

	// A pool of one connection: the connection used by the transaction is the one we look at after.
	pool, err := pgxpool.New(ctx, pgtest.AppURL(t)+"&pool_max_conns=1")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	runner := db.NewTxRunner(pool)

	err = runner.InTenantTx(ctx, id, func(ctx context.Context, tx db.Tx) error {
		if got := currentUser(t, ctx, tx); got != role {
			t.Errorf("current_user inside = %q, want %q", got, role)
		}
		if got, ok := tx.TenantID(); !ok || got != id {
			t.Errorf("TenantID() = %v, %v; want %v, true", got, ok, id)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var after string
	if err := pool.QueryRow(ctx, `SELECT current_user`).Scan(&after); err != nil || after != "crm_app" {
		t.Errorf("current_user after COMMIT on the same connection = %q (err %v), want crm_app", after, err)
	}
}

func TestTxRunner_RollsBackWhenTheFunctionFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	id, _ := newTenant(t)
	table := fixture.ProbeTable(t, pgtest.OwnerPool(t))
	runner, pool := singleConn(t)
	boom := errors.New("boom")

	err := runner.InTenantTx(ctx, id, func(ctx context.Context, tx db.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO `+table+` (tenant_id, note) VALUES ($1, 'x')`, id); err != nil {
			t.Fatal(err)
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want it to wrap boom", err)
	}
	assertBackToApp(t, pool)
	assertRows(t, runner, id, table, 0)
}

func TestTxRunner_CommitsWhenTheFunctionSucceeds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	id, _ := newTenant(t)
	table := fixture.ProbeTable(t, pgtest.OwnerPool(t))
	runner := db.NewTxRunner(pgtest.AppPool(t))

	err := runner.InTenantTx(ctx, id, func(ctx context.Context, tx db.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO `+table+` (tenant_id, note) VALUES ($1, 'x')`, id)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRows(t, runner, id, table, 1)
}

func assertRows(t *testing.T, runner db.TxRunner, id uuid.UUID, table string, want int) {
	t.Helper()
	err := runner.InTenantTx(context.Background(), id, func(ctx context.Context, tx db.Tx) error {
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&n); err != nil {
			return err
		}
		if n != want {
			t.Errorf("rows in %s = %d, want %d", table, n, want)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTxRunner_PanicRollsBackAndRepanics(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	id, _ := newTenant(t)
	table := fixture.ProbeTable(t, pgtest.OwnerPool(t))
	runner, pool := singleConn(t)

	func() {
		defer func() {
			if r := recover(); r != "kaboom" {
				t.Errorf("recovered %v, want the original panic value", r)
			}
		}()
		_ = runner.InTenantTx(ctx, id, func(ctx context.Context, tx db.Tx) error {
			if _, err := tx.Exec(ctx, `INSERT INTO `+table+` (tenant_id, note) VALUES ($1, 'x')`, id); err != nil {
				t.Fatal(err)
			}
			panic("kaboom")
		})
		t.Error("InTenantTx returned after a panic")
	}()
	assertBackToApp(t, pool)
	assertRows(t, runner, id, table, 0)
}

// A system transaction may move to one company, and back to a system role.
func TestTxRunner_SystemToTenantAndBack(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	id, role := newTenant(t)
	runner := db.NewTxRunner(pgtest.AppPool(t))

	err := runner.InSystemTx(ctx, db.RoleAuth, func(ctx context.Context, tx db.Tx) error {
		if got := currentUser(t, ctx, tx); got != "crm_auth" {
			t.Errorf("first step current_user = %q, want crm_auth", got)
		}
		if _, ok := tx.TenantID(); ok {
			t.Error("TenantID() is set before AsTenant")
		}
		if err := tx.AsTenant(ctx, id); err != nil {
			return err
		}
		if got := currentUser(t, ctx, tx); got != role {
			t.Errorf("after AsTenant current_user = %q, want %q", got, role)
		}
		if err := tx.AsSystem(ctx, db.RoleAuth); err != nil {
			return err
		}
		if got := currentUser(t, ctx, tx); got != "crm_auth" {
			t.Errorf("after AsSystem current_user = %q, want crm_auth", got)
		}
		// Still bound to the company: asking for it again is fine.
		if got, ok := tx.TenantID(); !ok || got != id {
			t.Errorf("TenantID() = %v, %v after going back to the system role", got, ok)
		}
		return tx.AsTenant(ctx, id)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTxRunner_InSystemTxRoles(t *testing.T) {
	t.Parallel()
	runner := db.NewTxRunner(pgtest.AppPool(t))
	for role, want := range map[db.SystemRole]string{db.RoleAuth: "crm_auth", db.RoleWorker: "crm_worker", db.RoleSignup: "crm_signup"} {
		err := runner.InSystemTx(context.Background(), role, func(ctx context.Context, tx db.Tx) error {
			if got := currentUser(t, ctx, tx); got != want {
				t.Errorf("InSystemTx(%s): current_user = %q, want %q", role, got, want)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	// An unknown role never reaches SET ROLE: it would be an injection vector.
	err := runner.InSystemTx(context.Background(), db.SystemRole("crm_owner"), func(context.Context, db.Tx) error {
		t.Error("fn ran for an unknown system role")
		return nil
	})
	if err == nil {
		t.Error("InSystemTx(crm_owner) returned no error")
	}
}

// INV-03: a transaction belongs to at most one company.
func TestTxRunner_NeverTwoCompaniesInOneTransaction(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a, roleA := newTenant(t)
	b, roleB := newTenant(t)
	runner := db.NewTxRunner(pgtest.AppPool(t))

	t.Run("tenant tx then another company", func(t *testing.T) {
		err := runner.InTenantTx(ctx, a, func(ctx context.Context, tx db.Tx) error {
			if err := tx.AsTenant(ctx, b); !errors.Is(err, db.ErrTenantAlreadyBound) {
				t.Errorf("AsTenant(B) = %v, want ErrTenantAlreadyBound", err)
			}
			if got := currentUser(t, ctx, tx); got != roleA {
				t.Errorf("current_user = %q after the refused switch, want %q (nothing ran as %s)", got, roleA, roleB)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("system tx, company A, then company B", func(t *testing.T) {
		err := runner.InSystemTx(ctx, db.RoleAuth, func(ctx context.Context, tx db.Tx) error {
			if err := tx.AsTenant(ctx, a); err != nil {
				return err
			}
			if err := tx.AsTenant(ctx, b); !errors.Is(err, db.ErrTenantAlreadyBound) {
				t.Errorf("AsTenant(B) = %v, want ErrTenantAlreadyBound", err)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})
}

// A missing company role is a 500-level problem with an actionable message (runbook §12.3).
func TestTxRunner_MissingCompanyRoleNamesTheRole(t *testing.T) {
	t.Parallel()
	id := uuid.New() // never provisioned
	runner := db.NewTxRunner(pgtest.AppPool(t))
	err := runner.InTenantTx(context.Background(), id, func(context.Context, db.Tx) error {
		t.Error("fn ran without a company role")
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "set role "+db.TenantRoleName(id)) {
		t.Errorf("err = %v, want it to name the step and the role: set role %s", err, db.TenantRoleName(id))
	}
	// Only here is the hint right: PostgreSQL 18 answers 22023 for a role that does not exist.
	if err == nil || !strings.Contains(err.Error(), "was the company provisioned?") {
		t.Errorf("err = %v, want the provisioning hint for a nonexistent role", err)
	}
}

func TestTxRunner_CancelledContextRollsBack(t *testing.T) {
	t.Parallel()
	id, _ := newTenant(t)
	table := fixture.ProbeTable(t, pgtest.OwnerPool(t))
	runner, pool := singleConn(t)
	ctx, cancel := context.WithCancel(context.Background())

	start := time.Now()
	err := runner.InTenantTx(ctx, id, func(ctx context.Context, tx db.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO `+table+` (tenant_id, note) VALUES ($1, 'x')`, id); err != nil {
			return err
		}
		time.AfterFunc(100*time.Millisecond, cancel)
		_, err := tx.Exec(ctx, `SELECT pg_sleep(4)`)
		return err
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Errorf("took %v: the query was not cancelled", time.Since(start))
	}
	assertBackToApp(t, pool)
	assertRows(t, db.NewTxRunner(pgtest.AppPool(t)), id, table, 0) // the insert before the sleep was rolled back
}

// A PostgreSQL privilege error while switching role is INV-19's problem, not a generic failure: it must
// classify as ErrPrivilege and still name the role.
func TestTxRunner_RoleSwitchPrivilegeErrorIsErrPrivilege(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	id := uuid.New()
	role := db.TenantRoleName(id)
	// The role exists but crm_app was not granted SET on it (a broken provisioning).
	if _, err := pgtest.SuperuserPool(t).Exec(ctx, `CREATE ROLE `+role+` NOLOGIN`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pgtest.SuperuserPool(t).Exec(ctx, `DROP ROLE `+role) })

	runner := db.NewTxRunner(pgtest.AppPool(t))
	err := runner.InTenantTx(ctx, id, func(context.Context, db.Tx) error {
		t.Error("fn ran without the role switch")
		return nil
	})
	if !errors.Is(err, db.ErrPrivilege) {
		t.Errorf("err = %v, want it to be ErrPrivilege", err)
	}
	if err == nil || !strings.Contains(err.Error(), "set role "+role) {
		t.Errorf("err = %v, want it to name the step and the role: set role %s", err, role)
	}
	if err != nil && strings.Contains(err.Error(), "provisioned") {
		t.Errorf("err = %v: the role exists, so the provisioning hint is misleading", err)
	}

	// The same through the Tx method.
	var inner error
	err = runner.InSystemTx(ctx, db.RoleAuth, func(ctx context.Context, tx db.Tx) error {
		inner = tx.AsTenant(ctx, id)
		return inner // the transaction is aborted by the failed SET ROLE: give it up
	})
	if !errors.Is(inner, db.ErrPrivilege) || !errors.Is(err, db.ErrPrivilege) {
		t.Errorf("AsTenant = %v, InSystemTx = %v; want both to be ErrPrivilege", inner, err)
	}
}

// INV-02: a query outside a transaction helper fails closed.
func TestTxRunner_PoolWithoutARoleSwitchHasNoPrivileges(t *testing.T) {
	t.Parallel()
	table := fixture.ProbeTable(t, pgtest.OwnerPool(t))
	_, err := pgtest.AppPool(t).Exec(context.Background(), `SELECT 1 FROM `+table)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Fatalf("err = %v, want SQLSTATE 42501", err)
	}
	if !errors.Is(db.MapError(err), db.ErrPrivilege) {
		t.Errorf("MapError(42501) is not ErrPrivilege: %v", db.MapError(err))
	}
}

// A company created by one connection must be usable at once from ANY other connection of the pool:
// registration is followed by the next request, which runs on whichever connection is free. PostgreSQL
// keeps, per backend, a cached list of the roles a role may SET ROLE to (roles_is_member_of in acl.c);
// with many roles and concurrent registrations a backend can keep serving a stale list, and the first
// `SET LOCAL ROLE crm_t_...` on it fails with "permission denied to set role" until the next role
// invalidation reaches it. Rebuilding that list takes longer the more roles exist, which is what makes the
// race easy to hit here: 2000 roles, 12 concurrent registrations. See TxRunner.setRole.
//
// This is the detector of R-17, so it must not be blunted by the defence that comes after the one it
// guards: InTenantTx retries once, which turns most failures of the catalog read into a success. Measured
// without the read, final failures drop from 46-93 to 2-3 in 600. Hence each path also asserts that the
// retry counters did NOT move: the read alone has to be enough. InSystemTx -> AsTenant (the GET /me that
// follows a registration) has no retry at all and is covered by the read only.
//
// Not parallel: the counters are process-wide, and parallel tests resume only after this one ends.
func TestTxRunner_NewCompanyIsUsableOnAnyConnectionRightAfterProvisioning(t *testing.T) {
	ctx := context.Background()
	super := pgtest.SuperuserPool(t)
	_, err := super.Exec(ctx, `
		DO $$
		DECLARE r text;
		BEGIN
		  FOR i IN 1..2000 LOOP
		    r := 'crm_t_' || replace(uuidv7()::text, '-', '');
		    EXECUTE format('CREATE ROLE %I NOLOGIN', r);
		    EXECUTE format('GRANT %I TO crm_app WITH INHERIT FALSE, SET TRUE', r);
		  END LOOP;
		END $$`)
	if err != nil {
		t.Fatalf("creating the roles: %v", err)
	}

	pool := pgtest.AppPool(t)
	runner := db.NewTxRunner(pool)
	paths := []struct {
		name string
		use  func(ctx context.Context, id uuid.UUID) error
	}{
		{"InTenantTx", func(ctx context.Context, id uuid.UUID) error {
			return runner.InTenantTx(ctx, id, func(ctx context.Context, tx db.Tx) error {
				_, err := tx.Exec(ctx, `SELECT 1`)
				return err
			})
		}},
		{"InSystemTx then AsTenant", func(ctx context.Context, id uuid.UUID) error {
			return runner.InSystemTx(ctx, db.RoleAuth, func(ctx context.Context, tx db.Tx) error {
				if err := tx.AsTenant(ctx, id); err != nil {
					return err
				}
				_, err := tx.Exec(ctx, `SELECT 1`)
				return err
			})
		}},
	}
	for _, path := range paths {
		t.Run(path.name, func(t *testing.T) {
			recovered, failed := db.SetRoleRetryCount(db.RetryRecovered), db.SetRoleRetryCount(db.RetryFailed)
			var (
				mu       sync.Mutex
				failures []error
				wg       sync.WaitGroup
			)
			for range 12 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for range 50 {
						id := uuid.New()
						fixture.ProvisionRole(t, pool, id) // committed before it returns
						if err := path.use(ctx, id); err != nil {
							mu.Lock()
							failures = append(failures, err)
							mu.Unlock()
						}
					}
				}()
			}
			wg.Wait()
			if len(failures) > 0 {
				t.Errorf("%d of 600 first uses of a just-provisioned company failed on another connection; first: %v", len(failures), failures[0])
			}
			if n := db.SetRoleRetryCount(db.RetryRecovered) - recovered; n != 0 {
				t.Errorf("%d of 600 first uses needed the retry and recovered: the catalog read is no longer enough (R-17)", n)
			}
			if n := db.SetRoleRetryCount(db.RetryFailed) - failed; n != 0 {
				t.Errorf("%d of 600 first uses were retried and failed again (R-17)", n)
			}
		})
	}
}

// permissionDeniedToSetRole is what PostgreSQL answers when a backend's cached list of SET-able roles is stale.
func permissionDeniedToSetRole(role string) error {
	return &pgconn.PgError{Code: "42501", Message: `permission denied to set role "` + role + `"`}
}

type logRecords struct {
	mu   sync.Mutex
	recs []map[string]any
}

func (l *logRecords) Write(p []byte) (int, error) {
	var m map[string]any
	if err := json.Unmarshal(p, &m); err == nil {
		l.mu.Lock()
		l.recs = append(l.recs, m)
		l.mu.Unlock()
	}
	return len(p), nil
}

func (l *logRecords) retries() []map[string]any {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []map[string]any
	for _, r := range l.recs {
		if r["event"] == "set_role_retry" {
			out = append(out, r)
		}
	}
	return out
}

func retryRunner(t *testing.T) (db.TxRunner, *logRecords) {
	t.Helper()
	logs := &logRecords{}
	runner := db.NewTxRunner(pgtest.AppPool(t), db.WithLogger(slog.New(slog.NewJSONHandler(logs, nil))))
	return runner, logs
}

// Second line of defence against the stale role list: entering a company is retried ONCE when the very
// first SET LOCAL ROLE is refused, before fn has done anything; the whole transaction starts again.
// Not parallel: the retry counter is process-wide and these tests assert it exactly (parallel tests only
// resume once every sequential one has finished).
func TestTxRunner_EnteringACompanyIsRetriedOnceWhenSetRoleIsRefused(t *testing.T) {
	ctx := context.Background()
	id, role := newTenant(t)
	runner, logs := retryRunner(t)
	var attempts, ran atomic.Int32
	db.SetFault(runner, func(r string) error {
		if r == role && attempts.Add(1) == 1 {
			return permissionDeniedToSetRole(r)
		}
		return nil
	})
	recovered, failed := db.SetRoleRetryCount(db.RetryRecovered), db.SetRoleRetryCount(db.RetryFailed)

	err := runner.InTenantTx(ctx, id, func(ctx context.Context, tx db.Tx) error {
		ran.Add(1)
		if got := currentUser(t, ctx, tx); got != role {
			t.Errorf("current_user = %q, want %q", got, role)
		}
		// DD-34: the outcome is known only after the retry, so nothing is logged before fn runs.
		if n := len(logs.retries()); n != 0 {
			t.Errorf("%d retry records logged before the retry finished, want 0", n)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("InTenantTx = %v, want success after one retry", err)
	}
	if ran.Load() != 1 {
		t.Errorf("fn ran %d times, want exactly once", ran.Load())
	}
	if attempts.Load() != 2 {
		t.Errorf("SET LOCAL ROLE attempted %d times, want 2 (the refused one and one retry)", attempts.Load())
	}
	if got := db.SetRoleRetryCount(db.RetryRecovered) - recovered; got != 1 {
		t.Errorf("recovered counter grew by %d, want 1", got)
	}
	if got := db.SetRoleRetryCount(db.RetryFailed) - failed; got != 0 {
		t.Errorf("failed counter grew by %d, want 0", got)
	}
	assertRetryLog(t, logs, id, db.RetryRecovered)
}

// assertRetryLog checks the single WARN of DD-34: event set_role_retry with tenant_id and outcome, and
// nothing that could carry personal data.
func assertRetryLog(t *testing.T, logs *logRecords, id uuid.UUID, outcome string) {
	t.Helper()
	recs := logs.retries()
	if len(recs) != 1 {
		t.Fatalf("retry log records = %v, want exactly one", recs)
	}
	r := recs[0]
	if r["level"] != "WARN" || r["tenant_id"] != id.String() || r["outcome"] != outcome {
		t.Errorf("retry log record = %v, want WARN with tenant_id=%s and outcome=%s", r, id, outcome)
	}
}

func TestTxRunner_TheRetryIsSingle(t *testing.T) {
	ctx := context.Background()
	id, role := newTenant(t)
	runner, logs := retryRunner(t)
	var attempts atomic.Int32
	db.SetFault(runner, func(r string) error {
		if r == role {
			attempts.Add(1)
			return permissionDeniedToSetRole(r)
		}
		return nil
	})
	recovered, failed := db.SetRoleRetryCount(db.RetryRecovered), db.SetRoleRetryCount(db.RetryFailed)

	err := runner.InTenantTx(ctx, id, func(context.Context, db.Tx) error {
		t.Error("fn ran although the role was never entered")
		return nil
	})
	if !errors.Is(err, db.ErrPrivilege) {
		t.Errorf("err = %v, want ErrPrivilege after the retry also fails", err)
	}
	// INV-27: the message names the step, so this ERROR is told apart from an RLS violation inside a query.
	if err == nil || !strings.Contains(err.Error(), "set role "+role) {
		t.Errorf("err = %v, want it to name the step %q", err, "set role "+role)
	}
	// The role exists (42501): the "was it provisioned?" hint would point the operator the wrong way.
	if err != nil && strings.Contains(err.Error(), "provisioned") {
		t.Errorf("err = %v, must not suggest the company was not provisioned", err)
	}
	if attempts.Load() != 2 {
		t.Errorf("SET LOCAL ROLE attempted %d times, want exactly 2", attempts.Load())
	}
	if got := db.SetRoleRetryCount(db.RetryFailed) - failed; got != 1 {
		t.Errorf("failed counter grew by %d, want 1", got)
	}
	if got := db.SetRoleRetryCount(db.RetryRecovered) - recovered; got != 0 {
		t.Errorf("recovered counter grew by %d, want 0", got)
	}
	assertRetryLog(t, logs, id, db.RetryFailed)
}

// No retry for system roles, for AsTenant in the middle of a transaction, or once fn has run.
func TestTxRunner_NoRetryOutsideTheEntryOfACompany(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	id, role := newTenant(t)

	t.Run("system role", func(t *testing.T) {
		runner, _ := retryRunner(t)
		var attempts atomic.Int32
		db.SetFault(runner, func(r string) error {
			if r == "crm_auth" {
				attempts.Add(1)
				return permissionDeniedToSetRole(r)
			}
			return nil
		})
		err := runner.InSystemTx(ctx, db.RoleAuth, func(context.Context, db.Tx) error {
			t.Error("fn ran")
			return nil
		})
		if !errors.Is(err, db.ErrPrivilege) || attempts.Load() != 1 {
			t.Errorf("err = %v, attempts = %d; want ErrPrivilege and a single attempt", err, attempts.Load())
		}
	})

	t.Run("AsTenant inside a transaction", func(t *testing.T) {
		runner, _ := retryRunner(t)
		var attempts, ran atomic.Int32
		db.SetFault(runner, func(r string) error {
			if r == role {
				attempts.Add(1)
				return permissionDeniedToSetRole(r)
			}
			return nil
		})
		err := runner.InSystemTx(ctx, db.RoleAuth, func(ctx context.Context, tx db.Tx) error {
			ran.Add(1)
			return tx.AsTenant(ctx, id)
		})
		if !errors.Is(err, db.ErrPrivilege) || attempts.Load() != 1 || ran.Load() != 1 {
			t.Errorf("err = %v, attempts = %d, fn runs = %d; want ErrPrivilege, 1 and 1", err, attempts.Load(), ran.Load())
		}
	})

	t.Run("fn already ran", func(t *testing.T) {
		runner, _ := retryRunner(t)
		var ran atomic.Int32
		denied := &pgconn.PgError{Code: "42501", Message: `permission denied to set role "x"`}
		err := runner.InTenantTx(ctx, id, func(context.Context, db.Tx) error {
			ran.Add(1)
			return denied // fn itself reports the same error: it may have done work, so no second run
		})
		if ran.Load() != 1 || !errors.Is(err, denied) {
			t.Errorf("fn runs = %d, err = %v; want 1 and the error unchanged", ran.Load(), err)
		}
	})
}

// queryLog is a pgx.QueryTracer that records the SQL of every call, in order.
type queryLog struct {
	mu   sync.Mutex
	sqls []string
}

func (q *queryLog) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	q.mu.Lock()
	q.sqls = append(q.sqls, d.SQL)
	q.mu.Unlock()
	return ctx
}

func (q *queryLog) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

// roleCalls returns the recorded calls that touch the role or the membership catalog, in order.
func (q *queryLog) roleCalls() []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	var out []string
	for _, s := range q.sqls {
		if u := strings.ToUpper(s); strings.Contains(u, "ROLE") || strings.Contains(u, "PG_AUTH_MEMBERS") {
			out = append(out, s)
		}
	}
	return out
}

func tracedPool(t *testing.T) (*pgxpool.Pool, *queryLog) {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(pgtest.AppURL(t))
	if err != nil {
		t.Fatal(err)
	}
	q := &queryLog{}
	cfg.ConnConfig.Tracer = q
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool, q
}

// assertOneRoundTripPerSwitch checks every role change left in ONE call, catalog read first, and that
// the calls, in order, are for the wanted roles. It returns the calls so they can be compared.
func assertOneRoundTripPerSwitch(t *testing.T, q *queryLog, roles ...string) []string {
	t.Helper()
	calls := q.roleCalls()
	if len(calls) != len(roles) {
		t.Fatalf("role-related calls = %q, want exactly %d (one per role change)", calls, len(roles))
	}
	for i, role := range roles {
		read := strings.Index(calls[i], "pg_auth_members")
		set := strings.Index(calls[i], "SET LOCAL ROLE")
		if read < 0 || set < 0 || read > set {
			t.Errorf("call %d = %q: want the pg_auth_members read and then SET LOCAL ROLE in the same call", i, calls[i])
		}
		if !strings.HasSuffix(calls[i], pgx.Identifier{role}.Sanitize()) {
			t.Errorf("call %d = %q, want it to switch to %s", i, calls[i], role)
		}
	}
	return calls
}

// DD-34 defence (1): the catalog read and the SET LOCAL ROLE leave together, in every path that changes
// the role, and the fixtures that provision companies do the same through the same SQL.
func TestTxRunner_EveryRoleChangeIsOneRoundTripWithTheCatalogRead(t *testing.T) {
	ctx := context.Background()
	id, role := newTenant(t)
	noop := func(context.Context, db.Tx) error { return nil }

	t.Run("InTenantTx", func(t *testing.T) {
		pool, q := tracedPool(t)
		if err := db.NewTxRunner(pool).InTenantTx(ctx, id, noop); err != nil {
			t.Fatal(err)
		}
		assertOneRoundTripPerSwitch(t, q, role)
	})
	t.Run("InSystemTx", func(t *testing.T) {
		pool, q := tracedPool(t)
		if err := db.NewTxRunner(pool).InSystemTx(ctx, db.RoleAuth, noop); err != nil {
			t.Fatal(err)
		}
		assertOneRoundTripPerSwitch(t, q, "crm_auth")
	})
	t.Run("AsTenant", func(t *testing.T) {
		pool, q := tracedPool(t)
		err := db.NewTxRunner(pool).InSystemTx(ctx, db.RoleAuth, func(ctx context.Context, tx db.Tx) error {
			return tx.AsTenant(ctx, id)
		})
		if err != nil {
			t.Fatal(err)
		}
		assertOneRoundTripPerSwitch(t, q, "crm_auth", role)
	})
	t.Run("AsSystem", func(t *testing.T) {
		pool, q := tracedPool(t)
		err := db.NewTxRunner(pool).InTenantTx(ctx, id, func(ctx context.Context, tx db.Tx) error {
			return tx.AsSystem(ctx, db.RoleWorker)
		})
		if err != nil {
			t.Fatal(err)
		}
		assertOneRoundTripPerSwitch(t, q, role, "crm_worker")
	})
	t.Run("fixture.SetRole sends what the runner sends", func(t *testing.T) {
		poolR, qR := tracedPool(t)
		if err := db.NewTxRunner(poolR).InTenantTx(ctx, id, noop); err != nil {
			t.Fatal(err)
		}
		fromRunner := assertOneRoundTripPerSwitch(t, qR, role)

		poolF, qF := tracedPool(t)
		tx, err := poolF.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if err := fixture.SetRole(ctx, tx, role); err != nil {
			t.Fatal(err)
		}
		fromFixture := assertOneRoundTripPerSwitch(t, qF, role)
		if fromFixture[0] != fromRunner[0] {
			t.Errorf("fixture.SetRole sent %q, the runner sends %q: the fixtures would not exercise the same defence", fromFixture[0], fromRunner[0])
		}
	})
}

// Cancelling between the first attempt and the retry: no second attempt, ErrCanceled, and nothing is
// reported, because the outcome of the retry is unknown. Not parallel: it asserts the process-wide counters.
func TestTxRunner_CancelBetweenTheAttemptAndTheRetryReportsNothing(t *testing.T) {
	id, role := newTenant(t)
	runner, logs := retryRunner(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var attempts atomic.Int32
	db.SetFault(runner, func(r string) error {
		if r == role {
			attempts.Add(1)
			cancel() // the client leaves right after the first refusal
			return permissionDeniedToSetRole(r)
		}
		return nil
	})
	recovered, failed := db.SetRoleRetryCount(db.RetryRecovered), db.SetRoleRetryCount(db.RetryFailed)

	err := runner.InTenantTx(ctx, id, func(context.Context, db.Tx) error {
		t.Error("fn ran although the context was cancelled")
		return nil
	})
	if !errors.Is(err, db.ErrCanceled) {
		t.Errorf("err = %v, want ErrCanceled", err)
	}
	if attempts.Load() != 1 {
		t.Errorf("SET LOCAL ROLE attempted %d times, want 1 (no second attempt after the cancellation)", attempts.Load())
	}
	if n := len(logs.retries()); n != 0 {
		t.Errorf("%d retry records logged, want none: the retry did not finish", n)
	}
	if db.SetRoleRetryCount(db.RetryRecovered) != recovered || db.SetRoleRetryCount(db.RetryFailed) != failed {
		t.Error("the retry counters moved although the retry did not finish")
	}
}

// Only a refused first SET LOCAL ROLE is retried. Not parallel: it asserts the process-wide counters.
func TestTxRunner_OnlyTheRefusedEntryIsRetried(t *testing.T) {
	ctx := context.Background()
	id, _ := newTenant(t)
	other, _ := newTenant(t)
	table := fixture.ProbeTable(t, pgtest.OwnerPool(t))

	assertNoRetry := func(t *testing.T, logs *logRecords, recovered, failed int64) {
		t.Helper()
		if db.SetRoleRetryCount(db.RetryRecovered) != recovered || db.SetRoleRetryCount(db.RetryFailed) != failed {
			t.Error("set_role_retry_total moved, want no change")
		}
		if n := len(logs.retries()); n != 0 {
			t.Errorf("%d retry records logged, want none", n)
		}
	}

	t.Run("a real RLS violation inside fn", func(t *testing.T) {
		runner, logs := retryRunner(t)
		var attempts, ran atomic.Int32
		db.SetFault(runner, func(string) error { attempts.Add(1); return nil }) // counts entries, never fails
		recovered, failed := db.SetRoleRetryCount(db.RetryRecovered), db.SetRoleRetryCount(db.RetryFailed)

		err := runner.InTenantTx(ctx, id, func(ctx context.Context, tx db.Tx) error {
			ran.Add(1)
			// WITH CHECK of the policy: a row of another company. PostgreSQL answers 42501, like a refused SET ROLE.
			_, err := tx.Exec(ctx, `INSERT INTO `+table+` (tenant_id, note) VALUES ($1, 'x')`, other)
			return db.MapError(err) // what a store does with what pgx returns
		})
		var pgErr *pgconn.PgError
		if !errors.Is(err, db.ErrPrivilege) || !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Fatalf("err = %v, want ErrPrivilege with SQLSTATE 42501", err)
		}
		if ran.Load() != 1 || attempts.Load() != 1 {
			t.Errorf("fn ran %d times and the company was entered %d times, want 1 and 1", ran.Load(), attempts.Load())
		}
		if strings.Contains(err.Error(), "set role") {
			t.Errorf("err = %v: an RLS violation must not look like a refused role switch (INV-27)", err)
		}
		assertNoRetry(t, logs, recovered, failed)
	})

	t.Run("another 42501 while entering", func(t *testing.T) {
		runner, logs := retryRunner(t)
		var attempts atomic.Int32
		db.SetFault(runner, func(string) error {
			attempts.Add(1)
			return &pgconn.PgError{Code: "42501", Message: `permission denied for schema app`}
		})
		recovered, failed := db.SetRoleRetryCount(db.RetryRecovered), db.SetRoleRetryCount(db.RetryFailed)
		err := runner.InTenantTx(ctx, id, func(context.Context, db.Tx) error { t.Error("fn ran"); return nil })
		if !errors.Is(err, db.ErrPrivilege) || attempts.Load() != 1 {
			t.Errorf("err = %v, attempts = %d; want ErrPrivilege and a single attempt", err, attempts.Load())
		}
		assertNoRetry(t, logs, recovered, failed)
	})

	t.Run("a company without a role", func(t *testing.T) {
		runner, logs := retryRunner(t)
		var attempts atomic.Int32
		db.SetFault(runner, func(string) error { attempts.Add(1); return nil })
		recovered, failed := db.SetRoleRetryCount(db.RetryRecovered), db.SetRoleRetryCount(db.RetryFailed)
		err := runner.InTenantTx(ctx, uuid.New(), func(context.Context, db.Tx) error { t.Error("fn ran"); return nil })
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "22023" {
			t.Fatalf("err = %v, want SQLSTATE 22023 (nonexistent role in PostgreSQL 18)", err)
		}
		if attempts.Load() != 1 {
			t.Errorf("the company was entered %d times, want a single attempt", attempts.Load())
		}
		assertNoRetry(t, logs, recovered, failed)
	})
}

//go:build integration

package db_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
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
	runner := db.NewTxRunner(pgtest.AppPool(t))
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
	runner := db.NewTxRunner(pgtest.AppPool(t))

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
	if err == nil || !strings.Contains(err.Error(), db.TenantRoleName(id)) {
		t.Errorf("err = %v, want it to name %s", err, db.TenantRoleName(id))
	}
}

func TestTxRunner_CancelledContextRollsBack(t *testing.T) {
	t.Parallel()
	id, _ := newTenant(t)
	runner := db.NewTxRunner(pgtest.AppPool(t))
	ctx, cancel := context.WithCancel(context.Background())

	start := time.Now()
	err := runner.InTenantTx(ctx, id, func(ctx context.Context, tx db.Tx) error {
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
	// The pool is still healthy.
	if err := pgtest.AppPool(t).Ping(context.Background()); err != nil {
		t.Errorf("pool after cancel: %v", err)
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

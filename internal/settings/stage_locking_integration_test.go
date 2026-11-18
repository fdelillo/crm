//go:build integration

package settings_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/settings"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// holdSettingsLock is mechanism E: T_admin takes the row lock of tenant_settings, the calls start, and the
// test waits until PostgreSQL reports all of them blocked behind it before rolling T_admin back. So the calls
// really overlap, with no sleeps. Without the lock in the service no call would block and the wait fails.
func holdSettingsLock(t *testing.T, tenantID uuid.UUID, calls ...func(context.Context) error) {
	t.Helper()
	holdSettingsLockThen(t, tenantID, nil, calls...)
}

// holdSettingsLockThen is holdSettingsLock with whileBlocked, which runs once every call is blocked behind
// T_admin and before T_admin releases the lock.
func holdSettingsLockThen(t *testing.T, tenantID uuid.UUID, whileBlocked func(), calls ...func(context.Context) error) {
	t.Helper()
	holdRowLockThen(t, tenantID, `SELECT 1 FROM app.tenant_settings WHERE tenant_id = $1 FOR NO KEY UPDATE`, []any{tenantID}, whileBlocked, calls...)
}

// holdRowLockThen is the same mechanism over any row: lockSQL takes the lock the calls must wait for, with lockArgs.
func holdRowLockThen(t *testing.T, tenantID uuid.UUID, lockSQL string, lockArgs []any, whileBlocked func(), calls ...func(context.Context) error) {
	t.Helper()
	if maxConns := pgtest.AppPool(t).Config().MaxConns; len(calls)+1 > int(maxConns) {
		t.Fatalf("app pool has %d connections: not enough for T_admin plus %d calls", maxConns, len(calls))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	// A separate crm_app connection watches the backends: it must not use the pool the calls compete for.
	// The waiters queue behind each other, so the count follows the chain of blockers from T_admin.
	monitor, err := pgx.Connect(ctx, pgtest.AppURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		closeCtx, closeCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer closeCancel()
		_ = monitor.Close(closeCtx)
	}()

	done := make(chan error, len(calls))
	started, returned := 0, 0
	release := errors.New("test: release the settings lock")
	err = testRunner(t).InTenantTx(ctx, tenantID, func(ctx context.Context, tx db.Tx) error {
		if _, err := tx.Exec(ctx, lockSQL, lockArgs...); err != nil {
			return err
		}
		var pid int32
		if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
			return err
		}
		for _, call := range calls {
			started++
			go func() { done <- call(ctx) }()
		}
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				returned++
				return errors.New("a call returned before the lock was released: the service does not take the row lock")
			case <-ticker.C:
				var blocked int
				if err := monitor.QueryRow(ctx, `WITH RECURSIVE waiting(pid) AS (
						SELECT pid FROM pg_stat_activity WHERE $1 = ANY(pg_blocking_pids(pid))
						UNION SELECT a.pid FROM pg_stat_activity a JOIN waiting w ON w.pid = ANY(pg_blocking_pids(a.pid))
					) SELECT count(*) FROM waiting`, pid).Scan(&blocked); err != nil {
					return err
				}
				if blocked == len(calls) {
					if whileBlocked != nil {
						whileBlocked()
					}
					return release
				}
			case <-ctx.Done():
				return fmt.Errorf("waiting for %d blocked backends: %w", len(calls), ctx.Err())
			}
		}
	})
	// Join the calls also when the wait failed, so none of them outlives the test.
	for ; returned < started; returned++ {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("a call never returned")
		}
	}
	if !errors.Is(err, release) {
		t.Fatal(err)
	}
}

// T-002-B401 (002/INV-2, DD-5): two concurrent deletions of the two won stages never leave the pipeline
// without a won one. Mutation: without LockTenantSettings both deletions read two won, both pass
// validatePipeline and none is left.
func TestDeleteStageConcurrentlyKeepsAWonStage(t *testing.T) {
	e := newStagesEnv(t)
	delivered := e.create(settings.NewStage{Name: "Entregado", Type: "won", Color: "green"})
	finished := e.id("Finalizado")

	var errFinished, errDelivered error
	holdSettingsLock(t, e.co.ID,
		func(ctx context.Context) error {
			errFinished = e.svc.DeleteStage(ctx, e.co.Admin, finished, e.meta)
			return nil
		},
		func(ctx context.Context) error {
			errDelivered = e.svc.DeleteStage(ctx, e.co.Admin, delivered.ID, e.meta)
			return nil
		},
	)

	var rejected int
	for _, err := range []error{errFinished, errDelivered} {
		if errors.Is(err, settings.ErrPipelineRule) {
			rejected++
		} else if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if rejected != 1 {
		t.Fatalf("errors = %v and %v, want exactly one ErrPipelineRule", errFinished, errDelivered)
	}
	won := 0
	for _, s := range e.list() {
		if s.Type == "won" {
			won++
		}
	}
	if won != 1 {
		t.Fatalf("won stages = %d, want 1: %v", won, e.summary())
	}
	e.assertDense()
}

// T-002-B401 (002/INV-2, DD-5): two concurrent creations at the end get positions 6 and 7, with no repeats or
// gaps. Mutation: without the lock both read five stages and end up in position 6.
func TestCreateStageConcurrentlyKeepsPositionsDense(t *testing.T) {
	e := newStagesEnv(t)
	var first, second settings.Stage
	var errFirst, errSecond error
	holdSettingsLock(t, e.co.ID,
		func(ctx context.Context) error {
			first, errFirst = e.svc.CreateStage(ctx, e.co.Admin, settings.NewStage{Name: "Instalación", Type: "open", Color: "blue"}, e.meta)
			return nil
		},
		func(ctx context.Context) error {
			second, errSecond = e.svc.CreateStage(ctx, e.co.Admin, settings.NewStage{Name: "Entrega", Type: "open", Color: "teal"}, e.meta)
			return nil
		},
	)
	if errFirst != nil || errSecond != nil {
		t.Fatalf("errors: %v, %v", errFirst, errSecond)
	}
	positions := []int{first.Position, second.Position}
	slices.Sort(positions)
	if !slices.Equal(positions, []int{6, 7}) {
		t.Fatalf("returned positions = %v, want [6 7]", positions)
	}
	if got := len(e.list()); got != 7 {
		t.Fatalf("stages = %d, want 7", got)
	}
	e.assertDense()
}

// Codex review of PR #63 (002/DD-5): a write stamps updated_at with the instant it holds the settings lock,
// not the instant it started waiting for it; otherwise a write that waited could stamp an older time over a
// newer change. The call blocks behind T_admin, the clock moves an hour and only then the lock is released.
// Mutation: reading the clock before LockTenantSettings stamps the old instant.
func TestStageWriteStampsTheInstantItHoldsTheLock(t *testing.T) {
	e := newStagesEnv(t)
	stage := e.id("Finalizado")
	waited := e.clk.Now()
	var errUpdate error
	holdSettingsLockThen(t, e.co.ID, func() { e.clk.Advance(time.Hour) },
		func(ctx context.Context) error {
			_, errUpdate = e.svc.UpdateStage(ctx, e.co.Admin, stage, settings.StageChanges{Color: ptr(settings.Color("teal"))}, e.meta)
			return nil
		},
	)
	if errUpdate != nil {
		t.Fatal(errUpdate)
	}
	var updated time.Time
	err := testRunner(t).InTenantTx(context.Background(), e.co.ID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT updated_at FROM app.stages WHERE tenant_id = $1 AND id = $2`, e.co.ID, stage).Scan(&updated)
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := waited.Add(time.Hour); !updated.Equal(want) {
		t.Fatalf("updated_at = %s, want %s (the instant under the lock, not %s)", updated.UTC(), want, waited)
	}
}

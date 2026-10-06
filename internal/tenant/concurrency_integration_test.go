//go:build integration

package tenant_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/tenant/store"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
)

// Mechanism E holds the tenant until every call waits directly or transitively.
// ordered launches each next call only after PostgreSQL reports the previous waiter.
func runTenantQueue(t *testing.T, runner db.TxRunner, p authz.Principal, ordered bool, calls ...func(context.Context) error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	done := make(chan error, len(calls))
	var outcomes []error
	release := errors.New("test: release tenant lock")
	started := 0
	err := runner.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		if _, err := store.New(tx).LockTenant(ctx, p.TenantID); err != nil {
			return err
		}
		var pid int32
		if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
			return err
		}
		wait := func(want int) error {
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case err := <-done:
					outcomes = append(outcomes, err)
					if err != nil {
						return fmt.Errorf("INV-34/DD-40: call returned before tenant lock release: %w", err)
					}
					return errors.New("INV-34/DD-40: call returned before tenant lock release")
				case <-ticker.C:
					var blocked int
					if err := pgtest.AppPool(t).QueryRow(ctx, `WITH RECURSIVE waiting(pid) AS (
     SELECT pid FROM pg_stat_activity WHERE $1 = ANY(pg_blocking_pids(pid))
     UNION SELECT a.pid FROM pg_stat_activity a JOIN waiting w ON w.pid = ANY(pg_blocking_pids(a.pid))
    ) SELECT count(*) FROM waiting`, pid).Scan(&blocked); err != nil {
						return err
					}
					if blocked == want {
						return nil
					}
				case <-ctx.Done():
					return fmt.Errorf("INV-34/DD-40: wanted %d blocked backends: %w", want, ctx.Err())
				}
			}
		}
		for i, call := range calls {
			started++
			go func() { done <- call(ctx) }()
			if ordered {
				if err := wait(i + 1); err != nil {
					return err
				}
			}
		}
		if !ordered {
			if err := wait(len(calls)); err != nil {
				return err
			}
		}
		return release
	})
	// The runner releases T_admin before joining, also when a mutation fails.
	for len(outcomes) < started {
		outcomes = append(outcomes, <-done)
	}
	if !errors.Is(err, release) {
		t.Fatal(err)
	}
	for _, err := range outcomes {
		if err != nil {
			t.Fatalf("queued call: %v", err)
		}
	}
}

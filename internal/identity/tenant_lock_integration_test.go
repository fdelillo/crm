//go:build integration

package identity

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/identity/store"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/jackc/pgx/v5/pgconn"
)

type gatedResetHasher struct {
	password.Hasher
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (h *gatedResetHasher) Hash(ctx context.Context, plain string) (string, error) {
	h.once.Do(func() {
		close(h.entered)
		select {
		case <-h.release:
		case <-ctx.Done():
		}
	})
	return h.Hasher.Hash(ctx, plain)
}

// Diagnostic of a design gap, NOT a passing phase-6 checkpoint. It reproduces
// the prescribed tenant -> user lock sequence against the unchanged phase-5
// reset, whose audit INSERT takes a tenant KEY SHARE lock through its FK.
// Replace this diagnostic with a regression test after the lock design is approved.
func TestProposedTenantLockConflictsWithPasswordReset(t *testing.T) {
	h := password.NewHasher(2)
	svc, runner, company, email, _, _ := loginFixture(t, h)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	hash, err := h.Hash(ctx, "old-password")
	if err != nil {
		t.Fatal(err)
	}
	setLoginPassword(t, runner, company, hash, "active")
	if err := svc.RequestPasswordReset(ctx, email, RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	raw := deliveredToken(t, runner, company.ID, email, "password_reset")
	gate := &gatedResetHasher{Hasher: h, entered: make(chan struct{}), release: make(chan struct{})}
	svc.hasher = gate
	var release sync.Once
	defer release.Do(func() { close(gate.release) })
	resetDone := make(chan error, 1)
	go func() { resetDone <- svc.ConfirmPasswordReset(ctx, raw, "new-password", RequestMeta{}) }()
	select {
	case <-gate.entered:
	case <-ctx.Done():
		t.Fatal("reset did not reach hashing while holding the user")
	}
	tenantLocked := make(chan int32, 1)
	managementDone := make(chan error, 1)
	go func() {
		managementDone <- runner.InTenantTx(ctx, company.ID, func(ctx context.Context, tx db.Tx) error {
			q := store.New(tx)
			if _, err := q.LockUsersTenant(ctx, company.ID); err != nil {
				return err
			}
			var pid int32
			if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
				return err
			}
			tenantLocked <- pid
			_, err := q.GetManagedUserForUpdate(ctx, store.GetManagedUserForUpdateParams{TenantID: company.ID, UserID: company.UserID})
			return db.MapError(err)
		})
	}()
	var pid int32
	select {
	case pid = <-tenantLocked:
	case <-ctx.Done():
		t.Fatal("management did not acquire the tenant")
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var wait string
		if err := pgtest.AppPool(t).QueryRow(ctx, `SELECT coalesce(wait_event_type, '') FROM pg_stat_activity WHERE pid=$1`, pid).Scan(&wait); err != nil {
			t.Fatal(err)
		}
		if wait == "Lock" {
			break
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal("management did not wait for the user lock")
		}
	}
	release.Do(func() { close(gate.release) })
	results := []error{<-resetDone, <-managementDone}
	deadlocks := 0
	successes := 0
	for _, err := range results {
		if err == nil {
			successes++
			continue
		}
		var pgerr *pgconn.PgError
		if errors.As(err, &pgerr) && pgerr.Code == "40P01" {
			deadlocks++
			continue
		}
		t.Fatalf("unexpected diagnostic failure: %v", err)
	}
	if deadlocks != 1 || successes != 1 {
		t.Fatalf("expected one PostgreSQL deadlock victim; got %v", results)
	}
	t.Log("confirmed design conflict: phase 6 holds tenant and waits user; phase 5 holds user and its audit FK waits tenant")
}

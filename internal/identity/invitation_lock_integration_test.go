//go:build integration

package identity

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/identity/store"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
)

// Mechanism E deliberately holds the tenant before either Invite starts. Both
// must wait before deciding whether the email is new (T-B601, INV-10, DD-40).
// PostgreSQL queues tuple locks: the second waiter can wait behind the first.
// Count the direct/transitive blocking chain, approved by the user on 2026-10-06.
func TestConcurrentNewInvitationsLockTenantBeforeEmailLookup(t *testing.T) {
	s, r, p, _ := usersFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	email := uuid.NewString() + "@example.com"
	type outcome struct {
		call     int
		user     User
		reissued bool
		err      error
	}
	done := make(chan outcome, 2)
	var outcomes []outcome
	rollback := errors.New("test: release tenant through rollback")
	err := r.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		if _, err := store.New(tx).LockUsersTenant(ctx, p.TenantID); err != nil {
			return err
		}
		var pid int32
		if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
			return err
		}
		for i := range 2 {
			go func() {
				u, reissued, err := s.Invite(ctx, p, email, authz.RoleOperator, RequestMeta{})
				done <- outcome{call: i, user: u, reissued: reissued, err: err}
			}()
		}
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case result := <-done:
				outcomes = append(outcomes, result)
				if result.err != nil {
					return fmt.Errorf("INV-10/DD-40: Invite %d returned before acquiring the tenant lock: %w", result.call, result.err)
				}
				return fmt.Errorf("INV-10/DD-40: Invite %d returned before acquiring the tenant lock", result.call)
			case <-ticker.C:
				var blocked int
				if err := pgtest.AppPool(t).QueryRow(ctx, `WITH RECURSIVE waiting(pid) AS (
     SELECT pid FROM pg_stat_activity WHERE $1 = ANY(pg_blocking_pids(pid))
     UNION
     SELECT a.pid FROM pg_stat_activity a JOIN waiting w ON w.pid = ANY(pg_blocking_pids(a.pid))
    ) SELECT count(*) FROM waiting`, pid).Scan(&blocked); err != nil {
					return err
				}

				if blocked == 2 {
					return rollback
				}
			case <-ctx.Done():
				return fmt.Errorf("INV-10/DD-40: two Invite backends did not block on tenant: %w", ctx.Err())
			}
		}
	})
	// Always roll back the held tenant and join both calls before failing, including
	// the mutation that removes its lock. The buffered channel cannot strand a sender.
	for len(outcomes) < 2 {
		outcomes = append(outcomes, <-done)
	}
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	var created, reissued int
	for _, result := range outcomes {
		if result.err != nil {
			t.Fatalf("INV-10/DD-40: Invite %d must succeed after rollback (lookup must follow tenant lock): %v", result.call, result.err)
		}
		if result.reissued {
			reissued++
		} else {
			created++
		}
		if result.user.Email != email || result.user.Role != authz.RoleOperator || result.user.Status != "invited" {
			t.Fatal(result.user)
		}
	}
	if created != 1 || reissued != 1 || outcomes[0].user.ID != outcomes[1].user.ID {
		t.Fatalf("creation/reissue outcomes: %+v", outcomes)
	}
	id := outcomes[0].user.ID
	requireUserAudit(t, r, p, id, "user.invited", p.UserID, map[string]any{"role": "operator"})
	requireUserAudit(t, r, p, id, "user.invitation_reissued", p.UserID, map[string]any{"role": "operator", "trigger": "admin"})
	if err := r.InTenantTx(ctx, p.TenantID, func(ctx context.Context, tx db.Tx) error {
		var users, mails, open, revoked int
		if err := tx.QueryRow(ctx, `SELECT
   (SELECT count(*) FROM app.users WHERE tenant_id=$1 AND email=$2),
   (SELECT count(*) FROM app.outbox_messages WHERE tenant_id=$1 AND recipient=$2 AND template='invitation' AND status='pending'),
   (SELECT count(*) FROM app.user_tokens WHERE tenant_id=$1 AND user_id=$3 AND purpose='invitation' AND used_at IS NULL AND revoked_at IS NULL),
   (SELECT count(*) FROM app.user_tokens WHERE tenant_id=$1 AND user_id=$3 AND purpose='invitation' AND revoked_at IS NOT NULL)`, p.TenantID, email, id).Scan(&users, &mails, &open, &revoked); err != nil {
			return err
		}
		if users != 1 || mails != 2 || open != 1 || revoked != 1 {
			t.Errorf("users=%d mails=%d open=%d revoked=%d", users, mails, open, revoked)
		}
		var actions []string
		rows, err := tx.Query(ctx, `SELECT action FROM app.audit_log WHERE tenant_id=$1 AND target_id=$2 AND action LIKE 'user.%' ORDER BY action`, p.TenantID, id)
		if err != nil {
			return err
		}
		for rows.Next() {
			var action string
			if err := rows.Scan(&action); err != nil {
				rows.Close()
				return err
			}
			actions = append(actions, action)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if !reflect.DeepEqual(actions, []string{"user.invitation_reissued", "user.invited"}) {
			t.Errorf("actions=%v", actions)
		}
		var openXID, reissueXID string
		if err := tx.QueryRow(ctx, `SELECT xmin::text FROM app.user_tokens WHERE tenant_id=$1 AND user_id=$2 AND purpose='invitation' AND used_at IS NULL AND revoked_at IS NULL`, p.TenantID, id).Scan(&openXID); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT xmin::text FROM app.audit_log WHERE tenant_id=$1 AND target_id=$2 AND action='user.invitation_reissued'`, p.TenantID, id).Scan(&reissueXID); err != nil {
			return err
		}
		if openXID != reissueXID {
			t.Error("open invitation must belong to reissue transaction")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

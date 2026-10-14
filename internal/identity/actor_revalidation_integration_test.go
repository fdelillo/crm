//go:build integration

package identity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/identity/store"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
)

// actorPair is a company with two active administrators: a (the founder, with the fixture's real
// session) and b (accepted from an invitation).
type actorPair struct {
	s      *Service
	r      db.TxRunner
	a, b   authz.Principal
	ctx    context.Context
	nowNow func() time.Time
}

func newActorPair(t *testing.T) *actorPair {
	t.Helper()
	s, r, a, c := usersFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	invited := inviteForTest(t, s, a, authz.RoleAdmin)
	accepted, err := s.AcceptInvitation(ctx, deliveredToken(t, r, a.TenantID, invited.Email, "invitation"), "B", "valid-password", RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	return &actorPair{s: s, r: r, a: a, b: accepted.Principal, ctx: ctx, nowNow: c.Now}
}

// demoteA / disableA apply, inside T_B's transaction, what ChangeRole and Deactivate would, with the
// queries of store (no audit): they only need to produce the committed state.
func (p *actorPair) demoteA(ctx context.Context, tx db.Tx) error {
	q := store.New(tx)
	if _, err := q.LockManagedUser(ctx, store.LockManagedUserParams{TenantID: p.a.TenantID, UserID: p.a.UserID}); err != nil {
		return err
	}
	return q.SetManagedUserRole(ctx, store.SetManagedUserRoleParams{TenantID: p.a.TenantID, UserID: p.a.UserID, Role: "operator", Now: p.nowNow()})
}
func (p *actorPair) disableA(ctx context.Context, tx db.Tx) error {
	return adminMutation(ctx, tx, fixture.Company{ID: p.a.TenantID, UserID: p.a.UserID}, false, p.nowNow())
}

// underLock is mechanism E of T-B604: T_B takes the tenant lock and applies mutate; the test waits,
// polling pg_blocking_pids every 10 ms, until the operation of A is blocked by T_B, and only then
// T_B commits. A's Principal was resolved before the mutation.
func (p *actorPair) underLock(t *testing.T, mutate func(context.Context, db.Tx) error, op func(context.Context) error) error {
	t.Helper()
	done := make(chan error, 1)
	err := p.r.InTenantTx(p.ctx, p.a.TenantID, func(ctx context.Context, tx db.Tx) error {
		if _, err := store.New(tx).LockUsersTenant(ctx, p.a.TenantID); err != nil {
			return err
		}
		var pid int32
		if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
			return err
		}
		if err := mutate(ctx, tx); err != nil {
			return err
		}
		go func() { done <- op(p.ctx) }()
		awaitLock(t, ctx, pid, true)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		return err
	case <-p.ctx.Done():
		t.Fatal(p.ctx.Err())
		return nil
	}
}

func (p *actorPair) counts(t *testing.T) [4]int {
	t.Helper()
	var n [4]int
	if err := p.r.InTenantTx(p.ctx, p.a.TenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM app.users WHERE tenant_id=$1),
		  (SELECT count(*) FROM app.user_tokens WHERE tenant_id=$1),
		  (SELECT count(*) FROM app.outbox_messages WHERE tenant_id=$1),
		  (SELECT count(*) FROM app.audit_log WHERE tenant_id=$1)`, p.a.TenantID).Scan(&n[0], &n[1], &n[2], &n[3])
	}); err != nil {
		t.Fatal(err)
	}
	return n
}

func (p *actorPair) state(t *testing.T, id uuid.UUID) (role, status string) {
	t.Helper()
	if err := p.r.InTenantTx(p.ctx, p.a.TenantID, func(ctx context.Context, tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT role, status FROM app.users WHERE tenant_id=$1 AND id=$2`, p.a.TenantID, id).Scan(&role, &status)
	}); err != nil {
		t.Fatal(err)
	}
	return role, status
}

func TestActorRevalidationUnderTenantLock(t *testing.T) {
	meta := RequestMeta{}
	t.Run("S1 demoted actor cannot promote itself", func(t *testing.T) {
		p := newActorPair(t)
		before := p.counts(t)
		err := p.underLock(t, p.demoteA, func(ctx context.Context) error {
			_, err := p.s.ChangeRole(ctx, p.a, p.a.UserID, authz.RoleAdmin, meta)
			return err
		})
		if !errors.Is(err, authz.ErrForbidden) {
			t.Fatalf("got %v, want ErrForbidden", err)
		}
		if role, _ := p.state(t, p.a.UserID); role != "operator" {
			t.Fatalf("A is %s, want operator", role)
		}
		if p.counts(t) != before {
			t.Fatal("rejected operation left rows behind")
		}
	})
	t.Run("S2 disabled actor cannot reactivate itself", func(t *testing.T) {
		p := newActorPair(t)
		before := p.counts(t)
		err := p.underLock(t, p.disableA, func(ctx context.Context) error {
			_, err := p.s.Reactivate(ctx, p.a, p.a.UserID, meta)
			return err
		})
		if !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("got %v, want ErrUnauthenticated", err)
		}
		if _, status := p.state(t, p.a.UserID); status != "disabled" {
			t.Fatalf("A is %s, want disabled", status)
		}
		if p.counts(t) != before {
			t.Fatal("rejected operation left rows behind")
		}
	})
	t.Run("S3 demoted actor cannot invite", func(t *testing.T) {
		p := newActorPair(t)
		before := p.counts(t)
		err := p.underLock(t, p.demoteA, func(ctx context.Context) error {
			_, _, err := p.s.Invite(ctx, p.a, "new-"+uuid.NewString()+"@example.com", authz.RoleAdmin, meta)
			return err
		})
		if !errors.Is(err, authz.ErrForbidden) {
			t.Fatalf("got %v, want ErrForbidden", err)
		}
		if p.counts(t) != before {
			t.Fatal("rejected invitation left a user, token, message or audit entry")
		}
	})
	t.Run("S4 disabled actor cannot deactivate another admin", func(t *testing.T) {
		p := newActorPair(t)
		err := p.underLock(t, p.disableA, func(ctx context.Context) error {
			_, err := p.s.Deactivate(ctx, p.a, p.b.UserID, meta)
			return err
		})
		if !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("got %v, want ErrUnauthenticated", err)
		}
		if _, status := p.state(t, p.b.UserID); status != "active" {
			t.Fatalf("B is %s, want active", status)
		}
	})
	t.Run("S5 logged out session cannot administer", func(t *testing.T) {
		p := newActorPair(t)
		var session SessionResult
		if err := p.r.InTenantTx(p.ctx, p.a.TenantID, func(ctx context.Context, tx db.Tx) error {
			var err error
			session, err = p.s.CreateSession(ctx, tx, p.a, meta)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		operator := inviteForTest(t, p.s, p.a, authz.RoleOperator)
		if err := p.s.Logout(p.ctx, session.RawToken, meta); err != nil {
			t.Fatal(err)
		}
		before := p.counts(t)
		if _, err := p.s.ChangeRole(p.ctx, session.Principal, operator.ID, authz.RoleAdmin, meta); !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("got %v, want ErrUnauthenticated", err)
		}
		if role, _ := p.state(t, operator.ID); role != "operator" || p.counts(t) != before {
			t.Fatal("rejected operation changed state")
		}
	})
	t.Run("S6 nil session fails closed in all four operations", func(t *testing.T) {
		p := newActorPair(t)
		anon := p.a
		anon.SessionID = uuid.Nil
		target := inviteForTest(t, p.s, p.a, authz.RoleOperator)
		before := p.counts(t)
		ops := map[string]func() error{
			"invite": func() error {
				_, _, err := p.s.Invite(p.ctx, anon, "x-"+uuid.NewString()+"@example.com", authz.RoleOperator, meta)
				return err
			},
			"change":     func() error { _, err := p.s.ChangeRole(p.ctx, anon, target.ID, authz.RoleAdmin, meta); return err },
			"deactivate": func() error { _, err := p.s.Deactivate(p.ctx, anon, target.ID, meta); return err },
			"reactivate": func() error { _, err := p.s.Reactivate(p.ctx, anon, target.ID, meta); return err },
		}
		for name, op := range ops {
			if err := op(); !errors.Is(err, ErrUnauthenticated) {
				t.Errorf("%s: got %v, want ErrUnauthenticated", name, err)
			}
		}
		if p.counts(t) != before {
			t.Fatal("rejected operations changed state")
		}
	})
	t.Run("S7 control: a valid actor operates as before", func(t *testing.T) {
		p := newActorPair(t)
		invited, _, err := p.s.Invite(p.ctx, p.a, "ok-"+uuid.NewString()+"@example.com", authz.RoleOperator, meta)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := p.s.ChangeRole(p.ctx, p.a, invited.ID, authz.RoleAdmin, meta); err != nil {
			t.Fatal(err)
		}
		if _, err := p.s.Deactivate(p.ctx, p.a, invited.ID, meta); err != nil {
			t.Fatal(err)
		}
		if _, err := p.s.Reactivate(p.ctx, p.a, invited.ID, meta); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("S8 revalidation comes before looking up the email", func(t *testing.T) {
		p := newActorPair(t)
		if _, err := p.s.ChangeRole(p.ctx, p.b, p.a.UserID, authz.RoleOperator, meta); err != nil {
			t.Fatal(err)
		}
		other := fixture.NewCompany(t, pgtest.AppPool(t))
		var email string
		if err := p.r.InTenantTx(p.ctx, other.ID, func(ctx context.Context, tx db.Tx) error {
			return tx.QueryRow(ctx, `SELECT email FROM app.users WHERE tenant_id=$1 AND id=$2`, other.ID, other.UserID).Scan(&email)
		}); err != nil {
			t.Fatal(err)
		}
		if _, _, err := p.s.Invite(p.ctx, p.a, email, authz.RoleOperator, meta); !errors.Is(err, authz.ErrForbidden) {
			t.Fatalf("got %v, want ErrForbidden and not ErrEmailTaken", err)
		}
	})
}

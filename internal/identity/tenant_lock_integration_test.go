//go:build integration

package identity

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/fdelillo/crm/internal/authz"
	"github.com/fdelillo/crm/internal/identity/store"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/testsupport/fixture"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"github.com/google/uuid"
)

type gatedResetHasher struct {
	password.Hasher
	entered, release chan struct{}
	once             sync.Once
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

type logoutGate struct {
	audit.Recorder
	entered chan int32
	release chan struct{}
}

func (g *logoutGate) Record(ctx context.Context, tx db.Tx, e audit.Entry) error {
	if e.Action == "auth.logout" {
		var pid int32
		if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
			return err
		}
		g.entered <- pid
		select {
		case <-g.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return g.Recorder.Record(ctx, tx, e)
}
func lockFixture(t *testing.T) (*Service, db.TxRunner, fixture.Company, string, context.Context) {
	t.Helper()
	h := password.NewHasher(2)
	s, r, c, email, _, _ := loginFixture(t, h)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	hash, err := h.Hash(ctx, "old-password")
	if err != nil {
		t.Fatal(err)
	}
	setLoginPassword(t, r, c, hash, "active")
	if err := r.InTenantTx(ctx, c.ID, func(ctx context.Context, tx db.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM app.sessions WHERE tenant_id=$1 AND user_id=$2`, c.ID, c.UserID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE app.users SET role='operator' WHERE tenant_id=$1 AND id=$2`, c.ID, c.UserID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return s, r, c, email, ctx
}
func awaitLock(t *testing.T, ctx context.Context, pid int32, blockedBy bool) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		sql := `SELECT coalesce(wait_event_type,'')='Lock' FROM pg_stat_activity WHERE pid=$1`
		if blockedBy {
			sql = `SELECT EXISTS(SELECT pid FROM pg_stat_activity WHERE $1 = ANY(pg_blocking_pids(pid)))`
		}
		if err := pgtest.AppPool(t).QueryRow(ctx, sql, pid).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			return
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal("DD-40: expected lock wait was not observed")
		}
	}
}
func adminMutation(ctx context.Context, tx db.Tx, c fixture.Company, role bool, now time.Time) error {
	q := store.New(tx)
	if _, err := q.GetManagedUserForUpdate(ctx, store.GetManagedUserForUpdateParams{TenantID: c.ID, UserID: c.UserID}); err != nil {
		return err
	}
	if role {
		return q.SetManagedUserRole(ctx, store.SetManagedUserRoleParams{TenantID: c.ID, UserID: c.UserID, Role: "admin", Now: now})
	}
	if err := q.SetManagedUserStatus(ctx, store.SetManagedUserStatusParams{TenantID: c.ID, UserID: c.UserID, Status: "disabled", Now: now}); err != nil {
		return err
	}
	if _, err := q.RevokeDisabledUserSessions(ctx, store.RevokeDisabledUserSessionsParams{TenantID: c.ID, UserID: c.UserID, Now: &now}); err != nil {
		return err
	}
	return q.RevokeDisabledUserTokens(ctx, store.RevokeDisabledUserTokensParams{TenantID: c.ID, UserID: c.UserID, Now: &now})
}
func requireFlow(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("DD-40: flow must commit without deadlock: %v", err)
	}
}
func requireState(t *testing.T, r db.TxRunner, c fixture.Company, status, reason string, session uuid.UUID) {
	t.Helper()
	requireFlow(t, r.InTenantTx(context.Background(), c.ID, func(ctx context.Context, tx db.Tx) error {
		var got string
		if err := tx.QueryRow(ctx, `SELECT status FROM app.users WHERE tenant_id=$1 AND id=$2`, c.ID, c.UserID).Scan(&got); err != nil {
			return err
		}
		if got != status {
			t.Errorf("status=%s want %s", got, status)
		}
		if session != uuid.Nil {
			var actual string
			if err := tx.QueryRow(ctx, `SELECT coalesce(revoked_reason,'') FROM app.sessions WHERE tenant_id=$1 AND id=$2`, c.ID, session).Scan(&actual); err != nil {
				return err
			}
			if actual != reason {
				t.Errorf("revoked_reason=%s want %s", actual, reason)
			}
		}
		return nil
	}))
}
func requireAuthAudit(t *testing.T, r db.TxRunner, c fixture.Company, action string, count int) {
	t.Helper()
	requireFlow(t, r.InTenantTx(context.Background(), c.ID, func(ctx context.Context, tx db.Tx) error {
		var n int
		err := tx.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE tenant_id=$1 AND (target_id=$2 OR actor_user_id=$2) AND action=$3`, c.ID, c.UserID, action).Scan(&n)
		if n != count {
			t.Errorf("%s count=%d want %d", action, n, count)
		}
		return err
	}))
}

func TestTenantLockRegressionE(t *testing.T) {
	for _, name := range []string{"R1", "R2", "R4"} {
		t.Run(name, func(t *testing.T) {
			s, r, c, email, ctx := lockFixture(t)
			var session SessionResult
			if name == "R2" {
				requireFlow(t, r.InTenantTx(ctx, c.ID, func(ctx context.Context, tx db.Tx) error {
					_, err := tx.Exec(ctx, `UPDATE app.users SET status='invited',password_hash=NULL,name=NULL,email_verified_at=NULL WHERE tenant_id=$1 AND id=$2`, c.ID, c.UserID)
					if err != nil {
						return err
					}
					return s.issueToken(ctx, tx, c.ID, c.UserID, email, "invitation", 7*24*time.Hour, nil)
				}))
			}
			done := make(chan error, 1)
			var blocked bool
			err := r.InTenantTx(ctx, c.ID, func(ctx context.Context, tx db.Tx) error {
				q := store.New(tx)
				if _, err := q.LockUsersTenant(ctx, c.ID); err != nil {
					return err
				}
				var pid int32
				if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
					return err
				}
				go func() {
					if name == "R4" {
						var err error
						session, err = s.Login(ctx, email, "old-password", RequestMeta{})
						done <- err
					} else {
						done <- s.RequestPasswordReset(ctx, email, RequestMeta{})
					}
				}()
				ticker := time.NewTicker(10 * time.Millisecond)
				defer ticker.Stop()
				for {
					select {
					case err := <-done:
						if err != nil {
							return err
						}
						return adminMutation(ctx, tx, c, name == "R2", s.clock.Now())
					case <-ticker.C:
						var waiting bool
						if err := pgtest.AppPool(t).QueryRow(ctx, `SELECT EXISTS(SELECT pid FROM pg_stat_activity WHERE $1 = ANY(pg_blocking_pids(pid)))`, pid).Scan(&waiting); err != nil {
							return err
						}
						if waiting {
							blocked = true
							return errors.New("DD-40: flow blocked by tenant lock")
						}
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			})
			if blocked {
				requireFlow(t, <-done)
			}
			requireFlow(t, err)
			if name == "R2" {
				requireState(t, r, c, "invited", "", uuid.Nil)
				preview, err := s.PreviewInvitation(ctx, deliveredToken(t, r, c.ID, email, "invitation"))
				requireFlow(t, err)
				if preview.Role != authz.RoleAdmin {
					t.Fatal(preview)
				}
				requireFlow(t, r.InTenantTx(ctx, c.ID, func(ctx context.Context, tx db.Tx) error {
					var n int
					err := tx.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE tenant_id=$1 AND target_id=$2 AND actor_user_id IS NULL AND action='user.invitation_reissued' AND data='{"role":"operator","trigger":"password_reset_request"}'::jsonb`, c.ID, c.UserID).Scan(&n)
					if n != 1 {
						t.Errorf("reissued audits=%d", n)
					}
					return err
				}))
				requireFlow(t, r.InTenantTx(ctx, c.ID, func(ctx context.Context, tx db.Tx) error {
					var open, revoked int
					var role string
					err := tx.QueryRow(ctx, `SELECT count(*) FILTER(WHERE revoked_at IS NULL),count(*) FILTER(WHERE revoked_at IS NOT NULL) FROM app.user_tokens WHERE tenant_id=$1 AND user_id=$2 AND purpose='invitation'`, c.ID, c.UserID).Scan(&open, &revoked)
					if err != nil {
						return err
					}
					if open != 1 || revoked != 1 {
						t.Errorf("tokens open=%d revoked=%d", open, revoked)
					}
					err = tx.QueryRow(ctx, `SELECT payload->>'role' FROM app.outbox_messages WHERE tenant_id=$1 AND recipient=$2 AND template='invitation' ORDER BY created_at DESC,id DESC LIMIT 1`, c.ID, email).Scan(&role)
					if role != "operator" {
						t.Errorf("mail role=%s", role)
					}
					return err
				}))
			} else {
				requireState(t, r, c, "disabled", "user_disabled", session.Principal.SessionID)
				if name == "R4" {
					_, err := s.ResolveSession(ctx, session.RawToken)
					if !errors.Is(err, ErrUnauthenticated) {
						t.Fatal(err)
					}
					requireAuthAudit(t, r, c, "auth.login_succeeded", 1)
				} else {
					requireAuthAudit(t, r, c, "auth.password_reset_requested", 1)
					requireFlow(t, r.InTenantTx(ctx, c.ID, func(ctx context.Context, tx db.Tx) error {
						var revoked, mails int
						err := tx.QueryRow(ctx, `SELECT count(*) FROM app.user_tokens WHERE tenant_id=$1 AND user_id=$2 AND purpose='password_reset' AND revoked_at IS NOT NULL`, c.ID, c.UserID).Scan(&revoked)
						if err != nil {
							return err
						}
						err = tx.QueryRow(ctx, `SELECT count(*) FROM app.outbox_messages WHERE tenant_id=$1 AND recipient=$2 AND template='password_reset'`, c.ID, email).Scan(&mails)
						if revoked != 1 || mails != 1 {
							t.Errorf("revoked=%d mails=%d", revoked, mails)
						}
						return err
					}))
				}
			}
		})
	}
}
func TestTenantLockRegressionR3(t *testing.T) {
	s, r, c, email, ctx := lockFixture(t)
	session, err := s.Login(ctx, email, "old-password", RequestMeta{})
	requireFlow(t, err)
	requireFlow(t, s.RequestPasswordReset(ctx, email, RequestMeta{}))
	raw := deliveredToken(t, r, c.ID, email, "password_reset")
	gate := &gatedResetHasher{Hasher: s.hasher, entered: make(chan struct{}), release: make(chan struct{})}
	s.hasher = gate
	var once sync.Once
	release := func() { once.Do(func() { close(gate.release) }) }
	defer release()
	resetDone := make(chan error, 1)
	go func() { resetDone <- s.ConfirmPasswordReset(ctx, raw, "new-password", RequestMeta{}) }()
	select {
	case <-gate.entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	adminDone := make(chan error, 1)
	tenantPID := make(chan int32, 1)
	go func() {
		adminDone <- r.InTenantTx(ctx, c.ID, func(ctx context.Context, tx db.Tx) error {
			if _, err := store.New(tx).LockUsersTenant(ctx, c.ID); err != nil {
				return err
			}
			var pid int32
			if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
				return err
			}
			tenantPID <- pid
			return adminMutation(ctx, tx, c, false, s.clock.Now())
		})
	}()
	select {
	case pid := <-tenantPID:
		awaitLock(t, ctx, pid, false)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	release()
	a, b := <-resetDone, <-adminDone
	requireFlow(t, a)
	requireFlow(t, b)
	requireState(t, r, c, "disabled", "password_reset", session.Principal.SessionID)
	requireResetState(t, s, r, c, email)
	requireAuthAudit(t, r, c, "auth.password_reset_completed", 1)
}
func requireResetState(t *testing.T, s *Service, r db.TxRunner, c fixture.Company, email string) {
	t.Helper()
	requireFlow(t, r.InTenantTx(context.Background(), c.ID, func(ctx context.Context, tx db.Tx) error {
		var hash string
		var used int
		if err := tx.QueryRow(ctx, `SELECT password_hash FROM app.users WHERE tenant_id=$1 AND id=$2`, c.ID, c.UserID).Scan(&hash); err != nil {
			return err
		}
		ok, _, err := s.hasher.Verify(ctx, "new-password", hash)
		if !ok {
			t.Error("new password not stored")
		}
		if err != nil {
			return err
		}
		err = tx.QueryRow(ctx, `SELECT count(*) FROM app.user_tokens WHERE tenant_id=$1 AND user_id=$2 AND purpose='password_reset' AND used_at IS NOT NULL`, c.ID, c.UserID).Scan(&used)
		if used != 1 {
			t.Errorf("used=%d", used)
		}
		return err
	}))
	requireFlow(t, r.InSystemTx(context.Background(), db.RoleAuth, func(ctx context.Context, tx db.Tx) error {
		var n int
		err := tx.QueryRow(ctx, `SELECT count(*) FROM app.login_throttles WHERE email_hmac=$1`, s.emailHMAC(email)).Scan(&n)
		if n != 0 {
			t.Errorf("throttles=%d", n)
		}
		return err
	}))
}
func TestTenantLockRegressionR5(t *testing.T) {
	s, r, c, email, ctx := lockFixture(t)
	gate := &gatedLoginHasher{Hasher: s.hasher, entered: make(chan struct{}), release: make(chan struct{})}
	s.hasher = gate
	var once sync.Once
	release := func() { once.Do(func() { close(gate.release) }) }
	defer release()
	done := make(chan error, 1)
	var session SessionResult
	go func() { var err error; session, err = s.Login(ctx, email, "old-password", RequestMeta{}); done <- err }()
	select {
	case <-gate.entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	requireFlow(t, r.InTenantTx(ctx, c.ID, func(ctx context.Context, tx db.Tx) error {
		if _, err := store.New(tx).LockUsersTenant(ctx, c.ID); err != nil {
			return err
		}
		return adminMutation(ctx, tx, c, false, s.clock.Now())
	}))
	release()
	requireFlow(t, <-done)
	requireState(t, r, c, "disabled", "", session.Principal.SessionID)
	if _, err := s.ResolveSession(ctx, session.RawToken); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal(err)
	}
	requireAuthAudit(t, r, c, "auth.login_succeeded", 1)
	requireFlow(t, r.InTenantTx(ctx, c.ID, func(ctx context.Context, tx db.Tx) error {
		return store.New(tx).SetManagedUserStatus(ctx, store.SetManagedUserStatusParams{TenantID: c.ID, UserID: c.UserID, Status: "active", Now: s.clock.Now()})
	}))
	if _, err := s.ResolveSession(ctx, session.RawToken); err != nil {
		t.Fatal("DD-41: accepted reactivation consequence", err)
	}
}
func TestUserLockRegressionLogout(t *testing.T) {
	for _, name := range []string{"L1", "L2"} {
		t.Run(name, func(t *testing.T) {
			s, r, c, email, ctx := lockFixture(t)
			session, err := s.Login(ctx, email, "old-password", RequestMeta{})
			requireFlow(t, err)
			raw := ""
			if name == "L2" {
				requireFlow(t, s.RequestPasswordReset(ctx, email, RequestMeta{}))
				raw = deliveredToken(t, r, c.ID, email, "password_reset")
			}
			gate := &logoutGate{Recorder: s.audit, entered: make(chan int32, 1), release: make(chan struct{})}
			s.audit = gate
			var once sync.Once
			release := func() { once.Do(func() { close(gate.release) }) }
			defer release()
			logoutDone := make(chan error, 1)
			go func() { logoutDone <- s.Logout(ctx, session.RawToken, RequestMeta{}) }()
			var pid int32
			select {
			case pid = <-gate.entered:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			otherDone := make(chan error, 1)
			go func() {
				if name == "L2" {
					otherDone <- s.ConfirmPasswordReset(ctx, raw, "new-password", RequestMeta{})
				} else {
					otherDone <- r.InTenantTx(ctx, c.ID, func(ctx context.Context, tx db.Tx) error {
						if _, err := store.New(tx).LockUsersTenant(ctx, c.ID); err != nil {
							return err
						}
						return adminMutation(ctx, tx, c, false, s.clock.Now())
					})
				}
			}()
			awaitLock(t, ctx, pid, true)
			release()
			a, b := <-logoutDone, <-otherDone
			requireFlow(t, a)
			requireFlow(t, b)
			status := "active"
			if name == "L1" {
				status = "disabled"
			}
			requireState(t, r, c, status, "logout", session.Principal.SessionID)
			requireAuthAudit(t, r, c, "auth.logout", 1)
			if name == "L2" {
				requireResetState(t, s, r, c, email)
				requireUserAudit(t, r, authz.Principal{TenantID: c.ID}, c.UserID, "auth.password_reset_completed", c.UserID, map[string]any{"sessions_revoked": float64(0)})
			}
		})
	}
}
